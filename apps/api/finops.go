package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"v3r1c0r3.local/auth"
	"v3r1c0r3.local/kms"
	"v3r1c0r3.local/mcp-flight-recorder"
	"v3r1c0r3.local/webhooks"
)

const (
	HighStakesThresholdCents = 100_000_000 // 1M USD in cents
	finopsAgentID            = "finops"
	statusExecuted           = "executed"
	statusPendingApproval    = "pending_approval"
	expectedStateCFOApproval = "CFO_APPROVAL"
	intentAutoExecuted       = "finops_auto_transfer_executed"
	intentHeldForFIDO        = "finops_transfer_held_for_fido"
)

// transferProposePayload is the body for POST /api/v1/finops/transfer/propose.
type transferProposePayload struct {
	FromAccount  string `json:"from_account"`
	ToAccount    string `json:"to_account"`
	AmountCents  int64  `json:"amount_cents"`
	Reasoning    string `json:"reasoning"`
}

// EnsureFinopsTables creates finops_accounts and finops_transfers if they do not exist.
func EnsureFinopsTables(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS finops_accounts (
		id            TEXT PRIMARY KEY,
		tenant_id     TEXT NOT NULL,
		name          TEXT NOT NULL,
		balance_cents INTEGER NOT NULL,
		currency      TEXT NOT NULL DEFAULT 'USD'
	)`)
	if err != nil {
		return err
	}
	_, _ = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_finops_accounts_tenant_id ON finops_accounts (tenant_id)`)

	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS finops_transfers (
		id                     TEXT PRIMARY KEY,
		tenant_id              TEXT NOT NULL,
		from_account           TEXT NOT NULL,
		to_account             TEXT NOT NULL,
		amount_cents           INTEGER NOT NULL,
		status                 TEXT NOT NULL,
		created_at             DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		verification_queue_id   INTEGER,
		FOREIGN KEY (from_account) REFERENCES finops_accounts(id),
		FOREIGN KEY (to_account)   REFERENCES finops_accounts(id)
	)`)
	if err != nil {
		return err
	}
	_, _ = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_finops_transfers_tenant_id ON finops_transfers (tenant_id)`)
	_, _ = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_finops_transfers_status ON finops_transfers (status)`)
	// Ensure verification_queue_id exists for DBs created before 006.
	if _, err := db.ExecContext(ctx, `ALTER TABLE finops_transfers ADD COLUMN verification_queue_id INTEGER`); err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}
	return nil
}

// EnsureWebhookEndpointsTable creates tenant_webhooks if it does not exist (migration 007).
func EnsureWebhookEndpointsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS tenant_webhooks (
		id          TEXT PRIMARY KEY,
		tenant_id   TEXT NOT NULL,
		endpoint_url TEXT NOT NULL,
		secret_key  TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}
	_, _ = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tenant_webhooks_tenant_id ON tenant_webhooks (tenant_id)`)
	return nil
}

// transferProposeHandler handles POST /api/v1/finops/transfer/propose with high-stakes interceptor.
func transferProposeHandler(primary *sql.DB, recorder flightrecorder.FlightRecorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		var payload transferProposePayload
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if payload.FromAccount == "" || payload.ToAccount == "" {
			http.Error(w, "from_account and to_account required", http.StatusBadRequest)
			return
		}
		if payload.AmountCents <= 0 {
			http.Error(w, "amount_cents must be positive", http.StatusBadRequest)
			return
		}
		if payload.FromAccount == payload.ToAccount {
			http.Error(w, "from_account and to_account must differ", http.StatusBadRequest)
			return
		}

		tenantID := auth.TenantIDFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		transferID, err := generateFinopsID()
		if err != nil {
			http.Error(w, "failed to generate transfer id", http.StatusInternalServerError)
			return
		}

		if payload.AmountCents < HighStakesThresholdCents {
			// Execute immediately in a LibSQL transaction.
			tx, err := primary.BeginTx(r.Context(), nil)
			if err != nil {
				http.Error(w, "failed to start transaction", http.StatusInternalServerError)
				return
			}
			defer tx.Rollback()

			var fromBalance int64
			err = tx.QueryRowContext(r.Context(),
				`SELECT balance_cents FROM finops_accounts WHERE id = ? AND tenant_id = ?`,
				payload.FromAccount, tenantID).Scan(&fromBalance)
			if err == sql.ErrNoRows {
				http.Error(w, "from_account not found or access denied", http.StatusNotFound)
				return
			}
			if err != nil {
				http.Error(w, "failed to read from_account", http.StatusInternalServerError)
				return
			}
			if fromBalance < payload.AmountCents {
				http.Error(w, "insufficient balance", http.StatusUnprocessableEntity)
				return
			}

			_, err = tx.ExecContext(r.Context(),
				`UPDATE finops_accounts SET balance_cents = balance_cents - ? WHERE id = ? AND tenant_id = ?`,
				payload.AmountCents, payload.FromAccount, tenantID)
			if err != nil {
				http.Error(w, "failed to deduct from sender", http.StatusInternalServerError)
				return
			}
			res, err := tx.ExecContext(r.Context(),
				`UPDATE finops_accounts SET balance_cents = balance_cents + ? WHERE id = ? AND tenant_id = ?`,
				payload.AmountCents, payload.ToAccount, tenantID)
			if err != nil {
				http.Error(w, "failed to credit receiver", http.StatusInternalServerError)
				return
			}
			aff, _ := res.RowsAffected()
			if aff == 0 {
				http.Error(w, "to_account not found or access denied", http.StatusNotFound)
				return
			}
			_, err = tx.ExecContext(r.Context(),
				`INSERT INTO finops_transfers (id, tenant_id, from_account, to_account, amount_cents, status) VALUES (?, ?, ?, ?, ?, ?)`,
				transferID, tenantID, payload.FromAccount, payload.ToAccount, payload.AmountCents, statusExecuted)
			if err != nil {
				http.Error(w, "failed to record transfer", http.StatusInternalServerError)
				return
			}
			if err := tx.Commit(); err != nil {
				http.Error(w, "failed to commit transfer", http.StatusInternalServerError)
				return
			}

			eventID, _ := generateEventID()
			paramsJSON, _ := json.Marshal(map[string]interface{}{
				"transfer_id": transferID, "from_account": payload.FromAccount, "to_account": payload.ToAccount,
				"amount_cents": payload.AmountCents, "reasoning": payload.Reasoning,
			})
			h := hexEncodeSHA256(paramsJSON)
			ev := flightrecorder.AuditEvent{
				ID:           eventID,
				Timestamp:    time.Now().UTC(),
				TenantID:     tenantID,
				AgentID:      finopsAgentID,
				Intent:       intentAutoExecuted,
				ToolName:     "finops_transfer_propose",
				ParamsJSON:   paramsJSON,
				EnvelopeHash: h,
			}
			if _, appendErr := recorder.Append(r.Context(), ev); appendErr != nil {
				// Log but do not fail the response; transfer already committed.
			}
			RecordAuditIntent(r.Context(), primary, eventID, intentAutoExecuted, tenantID)

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"transfer_id": transferID,
				"status":      statusExecuted,
			})
			return
		}

		// High-stakes: enqueue first, then insert transfer with verification_queue_id.
		queuePayload, _ := json.Marshal(map[string]interface{}{
			"transfer_id":    transferID,
			"expected_state": expectedStateCFOApproval,
			"from_account":   payload.FromAccount,
			"to_account":     payload.ToAccount,
			"amount_cents":   payload.AmountCents,
			"reasoning":       payload.Reasoning,
		})
		res, err := primary.ExecContext(r.Context(),
			`INSERT INTO verification_queue (state, payload_json) VALUES (?, ?)`,
			"pending", string(queuePayload))
		if err != nil {
			http.Error(w, "failed to enqueue for FIDO2 approval", http.StatusInternalServerError)
			return
		}
		queueID, _ := res.LastInsertId()

		_, err = primary.ExecContext(r.Context(),
			`INSERT INTO finops_transfers (id, tenant_id, from_account, to_account, amount_cents, status, verification_queue_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			transferID, tenantID, payload.FromAccount, payload.ToAccount, payload.AmountCents, statusPendingApproval, queueID)
		if err != nil {
			http.Error(w, "failed to record pending transfer", http.StatusInternalServerError)
			return
		}

		eventID, _ := generateEventID()
		paramsJSON, _ := json.Marshal(map[string]interface{}{
			"transfer_id": transferID, "verification_queue_id": queueID,
			"from_account": payload.FromAccount, "to_account": payload.ToAccount,
			"amount_cents": payload.AmountCents, "reasoning": payload.Reasoning,
		})
		h := hexEncodeSHA256(paramsJSON)
		ev := flightrecorder.AuditEvent{
			ID:           eventID,
			Timestamp:    time.Now().UTC(),
			TenantID:     tenantID,
			AgentID:      finopsAgentID,
			Intent:       intentHeldForFIDO,
			ToolName:     "finops_transfer_propose",
			ParamsJSON:   paramsJSON,
			EnvelopeHash: h,
		}
		if _, appendErr := recorder.Append(r.Context(), ev); appendErr != nil {
			// Log but do not fail; transfer and queue row already written.
		}
		RecordAuditIntent(r.Context(), primary, eventID, intentHeldForFIDO, tenantID)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"transfer_id":         transferID,
			"status":              statusPendingApproval,
			"verification_queue_id": queueID,
			"expected_state":      expectedStateCFOApproval,
		})
	}
}

func generateFinopsID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ft_" + hex.EncodeToString(b), nil
}

func hexEncodeSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// finopsAccountRow is the shape returned for each account.
type finopsAccountRow struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	Name         string `json:"name"`
	BalanceCents int64  `json:"balance_cents"`
	Currency     string `json:"currency"`
}

// WebhookDispatcher is used to enqueue events after settlement. May be nil to skip webhooks.
type WebhookDispatcher interface {
	Enqueue(ev webhooks.WebhookEvent)
}

// WebhookSecretDecrypter is used to decrypt stored webhook secrets before use. May be nil (treat as plaintext).
type WebhookSecretDecrypter interface {
	Decrypt(ciphertext []byte) ([]byte, error)
}

// SettleTransferByVerificationQueueID settles a pending_approval transfer after CFO FIDO2 approval.
// It updates verification_queue state, moves balances, and sets transfer status to executed.
// If the tenant has a webhook endpoint, a WebhookEvent with the transfer data is enqueued to dispatcher.
// The secret_key from tenant_webhooks is decrypted via kmsProvider before being passed to the webhook (envelope encryption).
// Call from agent/action when expected_state is CFO_APPROVAL.
func SettleTransferByVerificationQueueID(ctx context.Context, primary *sql.DB, verificationQueueID string, dispatcher WebhookDispatcher, kmsProvider WebhookSecretDecrypter) error {
	tracer := otel.Tracer("finops")
	ctx, span := tracer.Start(ctx, "finops.settle_transfer")
	defer span.End()

	var transferID, tenantID, fromAccount, toAccount string
	var amountCents int64
	err := primary.QueryRowContext(ctx,
		`SELECT id, tenant_id, from_account, to_account, amount_cents FROM finops_transfers WHERE verification_queue_id = ? AND status = ?`,
		verificationQueueID, statusPendingApproval).Scan(&transferID, &tenantID, &fromAccount, &toAccount, &amountCents)
	if err == sql.ErrNoRows {
		return nil // no pending transfer for this queue id
	}
	if err != nil {
		return err
	}
	span.SetAttributes(
		attribute.String("transfer_id", transferID),
		attribute.Int64("amount_cents", amountCents),
	)
	tx, err := primary.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`UPDATE verification_queue SET state = ? WHERE id = ?`,
		expectedStateCFOApproval, verificationQueueID); err != nil {
		return err
	}
	var fromBalance int64
	if err := tx.QueryRowContext(ctx,
		`SELECT balance_cents FROM finops_accounts WHERE id = ? AND tenant_id = ?`,
		fromAccount, tenantID).Scan(&fromBalance); err != nil {
		return err
	}
	if fromBalance < amountCents {
		return nil // leave transfer pending; insufficient balance
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE finops_accounts SET balance_cents = balance_cents - ? WHERE id = ? AND tenant_id = ?`,
		amountCents, fromAccount, tenantID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE finops_accounts SET balance_cents = balance_cents + ? WHERE id = ? AND tenant_id = ?`,
		amountCents, toAccount, tenantID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE finops_transfers SET status = ? WHERE id = ?`, statusExecuted, transferID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// Notify tenant webhook if configured (after DB is committed). Decrypt secret_key before use.
	if dispatcher != nil {
		var endpointURL, secretKeyStored string
		err := primary.QueryRowContext(ctx,
			`SELECT endpoint_url, secret_key FROM tenant_webhooks WHERE tenant_id = ? LIMIT 1`,
			tenantID).Scan(&endpointURL, &secretKeyStored)
		if err == nil && endpointURL != "" {
			plainSecret := secretKeyStored
			if kmsProvider != nil {
				ct, decErr := hex.DecodeString(secretKeyStored)
				if decErr == nil {
					plain, openErr := kmsProvider.Decrypt(ct)
					if openErr == nil {
						plainSecret = string(plain)
					}
				}
			}
			payload, _ := json.Marshal(map[string]interface{}{
				"event":            "finops_transfer_executed",
				"transfer_id":      transferID,
				"tenant_id":       tenantID,
				"from_account":   fromAccount,
				"to_account":      toAccount,
				"amount_cents":   amountCents,
				"status":         statusExecuted,
			})
			dispatcher.Enqueue(webhooks.WebhookEvent{
				EndpointURL:  endpointURL,
				Payload:      payload,
				TenantSecret: plainSecret,
			})
		}
	}
	return nil
}

// RegisterTenantWebhook inserts a webhook endpoint for a tenant. plainSecret is encrypted with kmsProvider before storage.
func RegisterTenantWebhook(ctx context.Context, db *sql.DB, kmsProvider kms.Provider, id, tenantID, endpointURL, plainSecret string) error {
	var secretKeyStored string
	if kmsProvider != nil {
		ct, err := kmsProvider.Encrypt([]byte(plainSecret))
		if err != nil {
			return err
		}
		secretKeyStored = hex.EncodeToString(ct)
	} else {
		secretKeyStored = plainSecret
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO tenant_webhooks (id, tenant_id, endpoint_url, secret_key) VALUES (?, ?, ?, ?)`,
		id, tenantID, endpointURL, secretKeyStored)
	return err
}

// finopsAccountsHandler returns all finops_accounts for the authenticated tenant.
func finopsAccountsHandler(primary *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tenantID := auth.TenantIDFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		rows, err := primary.QueryContext(r.Context(),
			`SELECT id, tenant_id, name, balance_cents, currency FROM finops_accounts WHERE tenant_id = ? ORDER BY id`,
			tenantID)
		if err != nil {
			http.Error(w, "failed to list accounts", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var list []finopsAccountRow
		for rows.Next() {
			var row finopsAccountRow
			if err := rows.Scan(&row.ID, &row.TenantID, &row.Name, &row.BalanceCents, &row.Currency); err != nil {
				http.Error(w, "failed to scan account", http.StatusInternalServerError)
				return
			}
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "failed to iterate accounts", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"accounts": list})
	}
}

// finopsTransferRow is the shape returned for each transfer.
type finopsTransferRow struct {
	ID                   string  `json:"id"`
	TenantID             string  `json:"tenant_id"`
	FromAccount          string  `json:"from_account"`
	ToAccount            string  `json:"to_account"`
	AmountCents          int64   `json:"amount_cents"`
	Status               string  `json:"status"`
	CreatedAt            string  `json:"created_at"`
	VerificationQueueID   *int64  `json:"verification_queue_id,omitempty"`
}

// finopsTransfersHandler returns all finops_transfers for the tenant, newest first.
func finopsTransfersHandler(primary *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tenantID := auth.TenantIDFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		rows, err := primary.QueryContext(r.Context(),
			`SELECT id, tenant_id, from_account, to_account, amount_cents, status, created_at, verification_queue_id
			 FROM finops_transfers WHERE tenant_id = ? ORDER BY created_at DESC`,
			tenantID)
		if err != nil {
			http.Error(w, "failed to list transfers", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var list []finopsTransferRow
		for rows.Next() {
			var row finopsTransferRow
			var queueID sql.NullInt64
			if err := rows.Scan(&row.ID, &row.TenantID, &row.FromAccount, &row.ToAccount, &row.AmountCents, &row.Status, &row.CreatedAt, &queueID); err != nil {
				http.Error(w, "failed to scan transfer", http.StatusInternalServerError)
				return
			}
			if queueID.Valid {
				row.VerificationQueueID = &queueID.Int64
			}
			list = append(list, row)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "failed to iterate transfers", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"transfers": list})
	}
}
