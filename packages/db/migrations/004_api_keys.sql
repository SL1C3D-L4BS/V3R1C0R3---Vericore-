-- 004_api_keys.sql
-- API keys for multi-tenant auth. Only key_hash is stored; raw keys are never persisted.

CREATE TABLE IF NOT EXISTS api_keys (
  id         TEXT PRIMARY KEY,
  tenant_id  TEXT NOT NULL,
  key_prefix TEXT NOT NULL,
  key_hash   TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  revoked_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys (key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id ON api_keys (tenant_id);
