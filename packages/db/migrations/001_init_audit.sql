-- 001_init_audit.sql
-- Initial schema for audit logging and Article 14 verification queue.

PRAGMA foreign_keys = ON;

BEGIN TRANSACTION;

CREATE TABLE IF NOT EXISTS audit_events (
  id                   INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp            DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  start_time           DATETIME NOT NULL,
  end_time             DATETIME NOT NULL,
  agent_id             TEXT NOT NULL,
  intent               TEXT NOT NULL,
  tool_name            TEXT NOT NULL,
  params_json          TEXT NOT NULL,
  state_before         TEXT,
  state_after          TEXT,
  reference_databases  TEXT,
  input_trigger_data   TEXT,
  verifier_crypto_id   TEXT,
  entry_hash           BLOB NOT NULL, -- leaf hash
  mmr_index            INTEGER NOT NULL,
  mmr_root             BLOB NOT NULL, -- checkpoint root at time of insertion
  receipt_ref          TEXT,
  guardrail_results    TEXT,
  identity_snapshot    TEXT,          -- immutable identity context (User ID, role, key id, etc.)
  webauthn_signature   BLOB,          -- hardware-backed approval signature
  tenant_id            TEXT NOT NULL DEFAULT 'default'
);

CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events (timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_events_agent_intent ON audit_events (agent_id, intent);

CREATE TABLE IF NOT EXISTS verification_queue (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at         DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at         DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  state              TEXT NOT NULL, -- pending, first_approval, second_approval, committed
  payload_json       TEXT NOT NULL,
  first_approver_id  TEXT,
  first_approved_at  DATETIME,
  second_approver_id TEXT,
  second_approved_at DATETIME,
  commit_lsn         TEXT,          -- commit LSN / version from sqld primary
  mmr_index          INTEGER,       -- optional link into audit_events MMR
  incident_id        INTEGER        -- link to incident table when anomalies occur
);

CREATE INDEX IF NOT EXISTS idx_verification_queue_state ON verification_queue (state);

CREATE TABLE IF NOT EXISTS incidents (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at         DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at         DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  type               TEXT NOT NULL, -- e.g. algorithmic_discrimination, security_violation
  description        TEXT NOT NULL,
  severity           TEXT NOT NULL,
  detection_source   TEXT NOT NULL, -- guardrail, anomaly_detector, manual_report, etc.
  reporting_deadline DATETIME,      -- e.g. Colorado 90-day AG deadline
  report_status      TEXT NOT NULL DEFAULT 'pending'
);

COMMIT;

