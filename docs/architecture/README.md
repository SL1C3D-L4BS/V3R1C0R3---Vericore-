# Architecture Overview

This directory contains architecture decision records (ADRs) and design notes for **SL1C3D-L4BS - V3R1C0R3 (Vericore)**.

Authoritative implementation guidance lives in:

- `.cursor/plans/d3v_efficiency_integrity_stack_6fd27a32.plan.md`

Key ADRs to maintain here:

- **001-causal-consistency-lsn.md** – LibSQL causal consistency tokens (LSN) for Article 14 multi-actor flows.
- `00x-secrets-identity.md` – Infisical + SPIFFE/SPIRE + Private PKI/CLM strategy.
- `00x-mmr-audit-log.md` – MMR audit design, root sealing, tombstone proofs, and GDPR cryptographic shredding.
- `00x-zkp-pipeline.md` – RISC Zero guest, cryptographic blinding, prover offload, and recursion.
- `00x-deployment-ha.md` – FRR BGP Anycast, RPKI/ROA, L7 RHI, RFC 8326, Tetragon, OOB management, PTP.

Each ADR should follow a consistent template (Context, Decision, Consequences) and explicitly reference the relevant regulatory clauses (EU AI Act, ISO 42001, prEN 18286, Colorado AI Act, etc.).

