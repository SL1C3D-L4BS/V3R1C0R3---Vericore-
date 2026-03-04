-- 008_pqc_signatures.sql
-- Post-quantum signature and public key on each MMR leaf (Dilithium).

ALTER TABLE mmr_leaves ADD COLUMN pqc_signature TEXT;
ALTER TABLE mmr_leaves ADD COLUMN pqc_public_key TEXT;
