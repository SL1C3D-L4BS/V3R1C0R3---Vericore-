-- 010_causal_swarm.sql
-- Causal Swarm Orchestration: DAG lineage via parent_hash for multi-agent causality.

ALTER TABLE mmr_leaves ADD COLUMN parent_hash TEXT;
CREATE INDEX IF NOT EXISTS idx_mmr_leaves_parent_hash ON mmr_leaves(parent_hash);
