-- 006_finops_verification_queue_id.sql
-- Link pending transfers to verification_queue for CFO approval flow.

ALTER TABLE finops_transfers ADD COLUMN verification_queue_id INTEGER;
