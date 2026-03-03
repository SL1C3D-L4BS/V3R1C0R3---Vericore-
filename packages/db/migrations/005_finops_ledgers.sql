-- 005_finops_ledgers.sql
-- FinOps domain: accounts and transfers for Autonomous Corporate Treasury.
-- Strict foreign keys and tenant_id indexes for multi-tenant isolation.

PRAGMA foreign_keys = ON;

BEGIN TRANSACTION;

CREATE TABLE IF NOT EXISTS finops_accounts (
  id            TEXT PRIMARY KEY,
  tenant_id     TEXT NOT NULL,
  name          TEXT NOT NULL,
  balance_cents INTEGER NOT NULL,
  currency      TEXT NOT NULL DEFAULT 'USD'
);

CREATE INDEX IF NOT EXISTS idx_finops_accounts_tenant_id ON finops_accounts (tenant_id);

CREATE TABLE IF NOT EXISTS finops_transfers (
  id            TEXT PRIMARY KEY,
  tenant_id     TEXT NOT NULL,
  from_account  TEXT NOT NULL,
  to_account    TEXT NOT NULL,
  amount_cents  INTEGER NOT NULL,
  status        TEXT NOT NULL,
  created_at    DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  FOREIGN KEY (from_account) REFERENCES finops_accounts(id),
  FOREIGN KEY (to_account)   REFERENCES finops_accounts(id)
);

CREATE INDEX IF NOT EXISTS idx_finops_transfers_tenant_id ON finops_transfers (tenant_id);
CREATE INDEX IF NOT EXISTS idx_finops_transfers_status ON finops_transfers (status);

COMMIT;
