# Canonical Compliance Mapping

This document maps the V3R1C0R3 architecture to international AI and data-protection regulations. It is the single source of truth for defending against hostile audits.

| Regulation / Article | Requirement | Satisfied By | Evidence / Location |
|----------------------|-------------|--------------|---------------------|
| **EU AI Act – Art. 12** | Record-keeping: logs of high-risk AI system operation for post-market monitoring and ex-post compliance | Merkle Mountain Range (MMR) append-only audit log | `packages/mcp-flight-recorder`: `Store`, `Append`, `AuditEvent`; MMR leaves and peaks persisted via `packages/db` `LibsqlStore`. All state-changing actions are recorded before execution (fail-closed). |
| **EU AI Act – Art. 14** | Human oversight: human in the loop, ability to intervene, interpret and understand outputs | (1) Double-verification UI with FIDO2; (2) Causal consistency so overseers see committed state | (1) **Next.js FIDO2 WebAuthn**: `apps/web/app/verification/page.tsx` – user must type confirmation phrase and complete hardware authenticator before approval. (2) **Causal consistency**: `packages/db/ryow.go` – `WaitForCommit` / `ExecuteWithFallback` ensure execution workers only act after replica has synced to the committed state (LSN/state polling). |
| **EU AI Act – Art. 72** | Post-market monitoring: monitor performance and report serious incidents | Guardrail kill-switch and **intervention logging** (no shadow blocks) | `packages/guardrails` `StrictSchemaValidator` blocks invalid/non-approved actions. `apps/api/main.go`: when `IsKillSwitch(err)`, the handler constructs an `AuditEvent` with `Intent: "guardrail_intervention_blocked"`, calls `recorder.Append()`, then returns 403. Every blocked action is logged (Article 72); no silent drops. |
| **GDPR – Right to Erasure** | Erasure of personal data when no longer necessary or on request | Envelope encryption + DEK shredding; ZKP blinding so PII never enters public proof | (1) **Envelope shredding**: `packages/db/crypto.go` – `EnvelopeManager` with `Encrypt` (DEK-wrapped ciphertext) and `ShredKey`. Destroying the DEK renders MMR ciphertext unrecoverable while hashes remain for proofs. (2) **ZKP blinding**: `packages/zkp-guest` – only decision and `input_hash` (SHA-256 of private input) are committed to the journal; plaintext PII never enters the receipt. |

## Summary

- **Art. 12**: MMR flight recorder (`mcp-flight-recorder` + `db` store).
- **Art. 14**: Verification UI (WebAuthn) + RYOW causal consistency (`db/ryow.go`).
- **Art. 72**: Guardrail kill-switch + mandatory audit of every block (`guardrails` + API intervention logging).
- **GDPR Erasure**: Envelope manager (`db/crypto.go`) + zkVM guest blinding (`zkp-guest`).
