-- 003_tenant_id.sql
-- Add tenant_id to LibSQL tables for multi-tenancy. Run once on existing DBs
-- that were created before tenant_id was added to the schema in code.

-- mmr_meta and mmr_leaves are created in packages/db/libsql_store.go; this migration
-- adds the column to existing tables. New installs get the column from ensureMMRSchema.
-- SQLite does not support IF NOT EXISTS for ADD COLUMN; ignore duplicate-column errors if re-run.

ALTER TABLE mmr_meta ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE mmr_leaves ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE audit_event_intents ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
