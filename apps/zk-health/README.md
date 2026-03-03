# zk-health: RISC Zero ZKP Enclave for HealthTech Triage

Zero-knowledge prover that evaluates Protected Health Information (PHI) and commits only the triage decision to the public journal.

## Layout

- **methods/** — RISC Zero methods crate: `embed_methods()` builds the guest and generates `TRIAGE_ELF` / `TRIAGE_ID`.
- **methods/guest/** — Guest program (runs inside the zkVM): reads PHI JSON, applies BP threshold, commits `TriageDecision` via `env::commit()`.
- **host/** — Host CLI: reads JSON from argv or stdin, runs the prover, outputs `{"receipt_base64","journal_base64","journal"}`.

## Build

Full build requires the [RISC Zero toolchain](https://dev.risczero.com/api/zkvm/install) (and on macOS, Xcode Command Line Tools for Metal if using the `prove` feature).

```bash
cd apps/zk-health
cargo build
```

For development without the full prover toolchain, use `RISC0_DEV_MODE=1` when running the host; the Go API sets this when invoking the binary.

Binary name: **zk-health** (from `host` crate). Set `ZK_HEALTH_BIN` to the path of the built binary when running the API (default: `zk-health` on PATH).

## Guest logic

- Input: length-prefixed JSON with at least `"blood_pressure_systolic": N`.
- If `N > 180`: commit `{"decision":"URGENT_ER_ROUTING","reason":"BP_THRESHOLD_EXCEEDED"}`.
- Else: commit `{"decision":"ROUTINE","reason":"BP_WITHIN_LIMITS"}`.
- Raw PHI is never committed; only the triage decision is public.
