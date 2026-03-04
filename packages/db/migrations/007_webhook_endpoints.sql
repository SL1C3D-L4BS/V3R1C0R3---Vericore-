-- 007_webhook_endpoints.sql
-- Tenant webhook endpoints for high-stakes event notifications (e.g. FinOps settlement).

CREATE TABLE IF NOT EXISTS tenant_webhooks (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    endpoint_url TEXT NOT NULL,
    secret_key  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tenant_webhooks_tenant_id ON tenant_webhooks (tenant_id);
