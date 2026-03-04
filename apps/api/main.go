package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"v3r1c0r3.local/auth"
	"v3r1c0r3.local/db"
	"v3r1c0r3.local/guardrails"
	"v3r1c0r3.local/kms"
	"v3r1c0r3.local/mcp-flight-recorder"
	"v3r1c0r3.local/pqc"
	"v3r1c0r3.local/telemetry"
	"v3r1c0r3.local/webhooks"
)

const (
	dbPathDefault     = "file:primary.db"
	ryowTimeout       = 500 * time.Millisecond
	defaultAgentID    = "api"
	defaultSyncInterval = 5 * time.Second
)

func main() {
	ctx := context.Background()
	shutdown, err := telemetry.InitProvider(ctx)
	if err != nil {
		log.Fatalf("telemetry init: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			log.Printf("telemetry shutdown: %v", err)
		}
	}()

	dbPath := os.Getenv("LIBSQL_DB_PATH")
	if dbPath == "" {
		dbPath = dbPathDefault
	}
	primaryURL := os.Getenv("LIBSQL_PRIMARY_URL")
	authToken := os.Getenv("LIBSQL_AUTH_TOKEN")
	syncInterval := defaultSyncInterval
	if d := os.Getenv("LIBSQL_SYNC_INTERVAL"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil && parsed > 0 {
			syncInterval = parsed
		}
	}

	storeConfig := db.StoreConfig{
		DBPath:        dbPath,
		PrimaryURL:   primaryURL,
		AuthToken:    authToken,
		SyncInterval: syncInterval,
	}
	store, primary, err := db.NewLibsqlStore(storeConfig)
	if err != nil {
		log.Fatalf("new libsql store: %v", err)
	}
	defer primary.Close()

	// Single DB for both write path and RYOW; replica mode uses embedded replica syncing to primary URL.
	replica := primary

	if err := EnsureAuditEventIntentsTable(context.Background(), primary); err != nil {
		log.Fatalf("ensure audit_event_intents: %v", err)
	}
	if err := EnsureAPIKeysTable(context.Background(), primary); err != nil {
		log.Fatalf("ensure api_keys: %v", err)
	}
	if err := EnsureFinopsTables(context.Background(), primary); err != nil {
		log.Fatalf("ensure finops tables: %v", err)
	}
	if err := EnsureWebhookEndpointsTable(context.Background(), primary); err != nil {
		log.Fatalf("ensure webhook endpoints table: %v", err)
	}

	masterKeyHex := os.Getenv("VERICORE_MASTER_KEY")
	if masterKeyHex == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatalf("failed to generate ephemeral KMS key: %v", err)
		}
		masterKeyHex = hex.EncodeToString(b)
		log.Printf("WARNING: Using ephemeral KMS key. Webhook secrets will be lost on restart.")
	}
	kmsProvider, err := kms.NewAESGCMProvider(masterKeyHex)
	if err != nil {
		log.Fatalf("invalid VERICORE_MASTER_KEY: %v", err)
	}

	webhookDispatcher := webhooks.NewDispatcher(1000)

	pqcPub, pqcPriv, err := pqc.GenerateKeypair()
	if err != nil {
		log.Fatalf("pqc keypair: %v", err)
	}
	log.Printf("WARNING: Using ephemeral PQC keypair. Leaf signatures are not durable across restarts.")

	recorder := flightrecorder.NewFlightRecorder(nil, store)
	validator := guardrails.NewStrictSchemaValidator()

	afterAppend := func(ctx context.Context, eventID, intent string) {
		RecordAuditIntent(ctx, primary, eventID, intent, auth.TenantIDFromContext(ctx))
	}

	r := chi.NewRouter()
	r.Use(OTelMiddleware)

	// L7 Route Health Injection: strict, stateless health that pings the DB write pool.
	r.Get("/health", healthHandler(primary))

	r.Get("/ready", readinessHandler())

	r.Get("/api/v1/telemetry/stats", telemetryStatsHandler(primary))

	keyValidator := &apiKeyValidator{db: primary}
	// Tenant auth then audit: API key required, then integrity boundary and guardrail.
	actionHandler := http.HandlerFunc(agentActionHandler(primary, replica, recorder, validator, webhookDispatcher, kmsProvider))
	audited := flightrecorder.AuditMiddleware(recorder, defaultAgentID, actionHandler, afterAppend, pqcPriv, pqcPub)
	actionWithAuth := auth.TenantAuthMiddleware(keyValidator, audited)
	r.Post("/api/v1/agent/action", func(w http.ResponseWriter, r *http.Request) { actionWithAuth.ServeHTTP(w, r) })

	// Developer portal: list and create API keys (requires tenant auth).
	portalKeysHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handlePortalKeysCreate(primary).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet {
			handlePortalKeysList(primary).ServeHTTP(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	portalWithAuth := auth.TenantAuthMiddleware(keyValidator, portalKeysHandler)
	r.Get("/api/v1/portal/keys", func(w http.ResponseWriter, r *http.Request) { portalWithAuth.ServeHTTP(w, r) })
	r.Post("/api/v1/portal/keys", func(w http.ResponseWriter, r *http.Request) { portalWithAuth.ServeHTTP(w, r) })

	// FinOps: propose transfer (high-stakes interceptor: auto-execute below $1M, hold for FIDO2 above).
	finopsProposeHandler := transferProposeHandler(primary, recorder)
	finopsProposeWithAuth := auth.TenantAuthMiddleware(keyValidator, finopsProposeHandler)
	r.Post("/api/v1/finops/transfer/propose", func(w http.ResponseWriter, r *http.Request) { finopsProposeWithAuth.ServeHTTP(w, r) })
	// FinOps: list accounts and transfers for CFO dashboard.
	finopsAccountsWithAuth := auth.TenantAuthMiddleware(keyValidator, finopsAccountsHandler(primary))
	finopsTransfersWithAuth := auth.TenantAuthMiddleware(keyValidator, finopsTransfersHandler(primary))
	r.Get("/api/v1/finops/accounts", func(w http.ResponseWriter, r *http.Request) { finopsAccountsWithAuth.ServeHTTP(w, r) })
	r.Get("/api/v1/finops/transfers", func(w http.ResponseWriter, r *http.Request) { finopsTransfersWithAuth.ServeHTTP(w, r) })

	// HealthTech: ZKP triage (PHI stays in enclave; only receipt + journal audited).
	zkHealthBin := os.Getenv("ZK_HEALTH_BIN")
	if zkHealthBin == "" {
		zkHealthBin = "zk-health"
	}
	triageWithAuth := auth.TenantAuthMiddleware(keyValidator, triageHandler(primary, recorder, zkHealthBin))
	r.Post("/api/v1/healthtech/triage", func(w http.ResponseWriter, r *http.Request) { triageWithAuth.ServeHTTP(w, r) })

	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// healthHandler returns 200 only if the DB write pool (primary) is reachable.
func healthHandler(writePool *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := writePool.PingContext(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("db unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

func readinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}

// actionPayload is the body for POST /api/v1/agent/action: ApprovalDecision (guardrail)
// plus record_id/expected_state for RYOW.
type actionPayload struct {
	ActionID       string `json:"action_id"`
	Decision       string `json:"decision"`
	Reasoning      string `json:"reasoning"`
	FIDOSignature  []byte `json:"fido_signature"`
	RecordID       string `json:"record_id"`
	ExpectedState  string `json:"expected_state"`
}

func agentActionHandler(primary, replica *sql.DB, recorder flightrecorder.FlightRecorder, validator guardrails.Validator, webhookDispatcher *webhooks.Dispatcher, kmsProvider kms.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		// Kill-switch: no state change without valid, approved decision (Article 14.5).
		decision, err := validator.Evaluate(r.Context(), bodyBytes)
		if err != nil {
			if guardrails.IsKillSwitch(err) {
				// Article 72: log the intervention before returning 403 (no shadow blocks).
				eventID, _ := generateEventID()
				paramsJSON, _ := json.Marshal(map[string]string{"reason": err.Error()})
				h := sha256.Sum256(paramsJSON)
				interventionEvent := flightrecorder.AuditEvent{
					ID:           eventID,
					Timestamp:    time.Now().UTC(),
					TenantID:     auth.TenantIDFromContext(r.Context()),
					AgentID:      defaultAgentID,
					Intent:       "guardrail_intervention_blocked",
					ToolName:     "StrictSchemaValidator",
					ParamsJSON:   paramsJSON,
					EnvelopeHash: hex.EncodeToString(h[:]),
				}
				if _, appendErr := recorder.Append(r.Context(), interventionEvent); appendErr != nil {
					log.Printf("guardrail_intervention_blocked: MMR append failed: %v", appendErr)
				} else {
					log.Printf("guardrail_intervention_blocked: event %s appended to MMR (Article 72); returning 403", eventID)
					RecordAuditIntent(r.Context(), primary, eventID, "guardrail_intervention_blocked", auth.TenantIDFromContext(r.Context()))
				}
				http.Error(w, err.Error(), http.StatusForbidden)
			} else {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			return
		}
		if decision.Decision != guardrails.DecisionApproved {
			http.Error(w, "action not approved", http.StatusForbidden)
			return
		}

		var payload actionPayload
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if payload.RecordID == "" {
			payload.RecordID = "1"
		}
		if payload.ExpectedState == "" {
			payload.ExpectedState = "committed"
		}

		// LSN-wait simulation: force replica to lag so WaitForCommit times out and we fall back to primary.
		if r.Header.Get("X-RYOW-Simulate-Lag") == "true" || r.Header.Get("X-RYOW-Simulate-Lag") == "1" {
			_, _ = replica.ExecContext(r.Context(), `UPDATE verification_queue SET state = 'pending' WHERE id = ?`, payload.RecordID)
			log.Printf("ryow: simulation: replica forced to state=pending for record %s (expected=%s), will timeout and fall back to primary", payload.RecordID, payload.ExpectedState)
		}

		ctx := r.Context()
		err = db.ExecuteWithFallback(ctx, replica, primary, payload.RecordID, payload.ExpectedState, ryowTimeout, func(d *sql.DB) error {
			_, err := d.ExecContext(ctx, `UPDATE verification_queue SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`, payload.RecordID)
			return err
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// CFO FinOps: settle high-stakes transfer when expected_state is CFO_APPROVAL.
		if payload.ExpectedState == "CFO_APPROVAL" {
			if settleErr := SettleTransferByVerificationQueueID(ctx, primary, payload.RecordID, webhookDispatcher, kmsProvider); settleErr != nil {
				log.Printf("finops settle after CFO_APPROVAL: %v", settleErr)
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

func generateEventID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
