-- 009_mcp_context.sql
-- MCP context hash on each MMR leaf for deep provenance binding.

ALTER TABLE mmr_leaves ADD COLUMN context_hash TEXT;
