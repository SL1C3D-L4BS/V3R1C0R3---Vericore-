SL1C3D-L4BS - V3R1C0R3 (Vericore)
==================================

This repository contains the implementation for the **SL1C3D-L4BS - V3R1C0R3 (Vericore)** Efficiency & Integrity stack.

The authoritative implementation plan lives in:

- `.cursor/plans/d3v_efficiency_integrity_stack_6fd27a32.plan.md`

At a high level the stack is:

- **Pillar 1 – Integrity**
  - MCP Flight Recorder (in-process Go library)
  - Merkle Mountain Range (MMR) audit log with root sealing and tombstone proofs
  - Envelope encryption (KEK/DEK) for GDPR cryptographic shredding
  - Guardrails + kill-switch + Article 14 double verification (FIDO2/WebAuthn, cognitive friction)
  - ZKP pipeline (RISC Zero, prover offload, recursive aggregation)
  - OTel ↔ Tetragon cross-layer observability
- **Pillar 2 – Efficiency**
  - Go modular monolith (`apps/api`)
  - LibSQL (`sqld`) primary on Confidential VMs + embedded replicas with WAL, dual pools, and LSN-based causal consistency
  - Tiered storage: LibSQL (Hot) → ClickHouse (Warm) → S3 (Cold) with native TTL + cross-tier MMR proofs
  - Next.js 19 RSC frontend (`apps/web`) with verification dashboards
  - Bare-metal HA: FRR BGP Anycast + RPKI/ROA + L7 Route Health Injection + RFC 8326 graceful shutdown, backed by Kamal

### Monorepo layout

- `apps/api` – Go monolith (backend surface, MCP/guardrails/ZKP integration)
- `apps/web` – Next.js 19 RSC frontend
- `packages/db` – LibSQL schema, migrations, and client
- `packages/mcp-flight-recorder` – MCP Flight Recorder Go library
- `packages/guardrails` – Guardrail interfaces and policies
- `packages/zkp-guest` – RISC Zero guest(s) and recursion circuits
- `docs/architecture` – ADRs and technical design notes
- `docs/compliance*.md` – EU AI Act, ISO 42001, prEN 18286, Colorado AI Act mappings

See the plan file for detailed phase ordering and deliverables.

