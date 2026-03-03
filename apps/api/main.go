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
	_ "modernc.org/sqlite"
	"v3r1c0r3.local/db"
	"v3r1c0r3.local/guardrails"
	"v3r1c0r3.local/mcp-flight-recorder"
)

const (
	primaryDSN = "file:primary.db"
	replicaDSN = "file:replica.db"
	ryowTimeout = 500 * time.Millisecond
	defaultAgentID = "api"
)

func main() {
	primary, err := sql.Open("sqlite", primaryDSN)
	if err != nil {
		log.Fatalf("open primary db: %v", err)
	}
	defer primary.Close()

	replica, err := sql.Open("sqlite", replicaDSN)
	if err != nil {
		log.Fatalf("open replica db: %v", err)
	}
	defer replica.Close()

	store, err := db.NewLibsqlStore(primary)
	if err != nil {
		log.Fatalf("new libsql store: %v", err)
	}

	recorder := flightrecorder.NewFlightRecorder(nil, store)
	validator := guardrails.NewStrictSchemaValidator()

	r := chi.NewRouter()

	// L7 Route Health Injection: strict, stateless health that pings the DB write pool.
	r.Get("/health", healthHandler(primary))

	r.Get("/ready", readinessHandler())

	// Integrity boundary: audit before proceed. Guardrail: kill-switch before RYOW.
	actionHandler := http.HandlerFunc(agentActionHandler(primary, replica, recorder, validator))
	r.Post("/api/v1/agent/action", flightrecorder.AuditMiddleware(recorder, defaultAgentID, actionHandler).ServeHTTP)

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

func agentActionHandler(primary, replica *sql.DB, recorder flightrecorder.FlightRecorder, validator guardrails.Validator) http.HandlerFunc {
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
					AgentID:      defaultAgentID,
					Intent:       "guardrail_intervention_blocked",
					ToolName:     "StrictSchemaValidator",
					ParamsJSON:   paramsJSON,
					EnvelopeHash: hex.EncodeToString(h[:]),
				}
				_, _ = recorder.Append(r.Context(), interventionEvent)
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

		ctx := r.Context()
		err = db.ExecuteWithFallback(ctx, replica, primary, payload.RecordID, payload.ExpectedState, ryowTimeout, func(d *sql.DB) error {
			_, err := d.ExecContext(ctx, `UPDATE verification_queue SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`, payload.RecordID)
			return err
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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
