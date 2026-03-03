package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"v3r1c0r3.local/auth"
)

const (
	apiKeyPrefix   = "vcore_test_"
	apiKeyPrefixLen = 15
	apiKeyRawLen   = 32
)

// EnsureAPIKeysTable creates the api_keys table if it does not exist and seeds
// the bootstrap key (sk_test_123 -> tenant_alpha) so existing clients keep working.
func EnsureAPIKeysTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS api_keys (
		id         TEXT PRIMARY KEY,
		tenant_id  TEXT NOT NULL,
		key_prefix TEXT NOT NULL,
		key_hash   TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		revoked_at DATETIME
	)`)
	if err != nil {
		return err
	}
	_, _ = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys (key_hash)`)
	_, _ = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id ON api_keys (tenant_id)`)

	// Seed bootstrap key so sk_test_123 continues to work after migration.
	seedHash := auth.HashAPIKey("sk_test_123")
	_, _ = db.ExecContext(ctx,
		`INSERT OR IGNORE INTO api_keys (id, tenant_id, key_prefix, key_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
		"seed-bootstrap", "tenant_alpha", "sk_test_123", seedHash, time.Now().UTC().Format(time.RFC3339))
	return nil
}

// apiKeyValidator implements auth.KeyValidator by querying the api_keys table.
type apiKeyValidator struct {
	db *sql.DB
}

func (v *apiKeyValidator) LookupTenantByKeyHash(ctx context.Context, keyHash string) (tenantID string, ok bool) {
	err := v.db.QueryRowContext(ctx,
		`SELECT tenant_id FROM api_keys WHERE key_hash = ? AND revoked_at IS NULL`,
		keyHash).Scan(&tenantID)
	return tenantID, err == nil
}

// POST /api/v1/portal/keys — generate a new API key for the authenticated tenant.
// Returns the raw key once in the JSON response; only the hash is stored.
func handlePortalKeysCreate(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tenantID := auth.TenantIDFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "tenant context required", http.StatusInternalServerError)
			return
		}

		rawBytes := make([]byte, apiKeyRawLen)
		if _, err := rand.Read(rawBytes); err != nil {
			http.Error(w, "failed to generate key", http.StatusInternalServerError)
			return
		}
		rawSuffix := hex.EncodeToString(rawBytes)
		rawKey := apiKeyPrefix + rawSuffix
		keyHash := auth.HashAPIKey(rawKey)
		keyPrefix := rawKey
		if len(keyPrefix) > apiKeyPrefixLen {
			keyPrefix = keyPrefix[:apiKeyPrefixLen]
		}
		id := "key_" + hex.EncodeToString(rawBytes[:8])

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, err := db.ExecContext(ctx,
			`INSERT INTO api_keys (id, tenant_id, key_prefix, key_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
			id, tenantID, keyPrefix, keyHash, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			http.Error(w, "failed to store key", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"key":        rawKey,
			"key_prefix": keyPrefix,
			"tenant_id":  tenantID,
			"created_at": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// GET /api/v1/portal/keys — list active API keys for the authenticated tenant (key_prefix, created_at only).
func handlePortalKeysList(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tenantID := auth.TenantIDFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "tenant context required", http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		rows, err := db.QueryContext(ctx,
			`SELECT key_prefix, created_at FROM api_keys WHERE tenant_id = ? AND revoked_at IS NULL ORDER BY created_at DESC`,
			tenantID)
		if err != nil {
			http.Error(w, "failed to list keys", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var keys []struct {
			KeyPrefix string `json:"key_prefix"`
			CreatedAt string `json:"created_at"`
		}
		for rows.Next() {
			var k struct {
				KeyPrefix string
				CreatedAt string
			}
			if err := rows.Scan(&k.KeyPrefix, &k.CreatedAt); err != nil {
				http.Error(w, "failed to scan key", http.StatusInternalServerError)
				return
			}
			keys = append(keys, struct {
				KeyPrefix string `json:"key_prefix"`
				CreatedAt string `json:"created_at"`
			}{KeyPrefix: k.KeyPrefix, CreatedAt: k.CreatedAt})
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "failed to list keys", http.StatusInternalServerError)
			return
		}
		if keys == nil {
			keys = []struct {
				KeyPrefix string `json:"key_prefix"`
				CreatedAt string `json:"created_at"`
			}{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": keys})
	}
}
