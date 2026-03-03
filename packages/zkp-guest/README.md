# zkp-guest

RISC Zero zkVM guest: cryptographic blinding and high-stakes policy. No PII is committed to the journal.

## Build

**Requires:** [RISC Zero toolchain](https://dev.risczero.com/api/zkvm/install) (e.g. `cargo risczero install` or `rzup install`).

From repo root:

- `pnpm run build-zkp` — runs `cargo build --release` in this package (Turbo caches on `Cargo.lock` and `**/*.rs`).
- Or: `cd packages/zkp-guest && cargo build --release` (builds for `riscv32im-risc0-zkvm-elf` per `.cargo/config.toml`).

## Input layout (host → guest)

1. `u64` trade_amount (LE)
2. `u32` payload_len (LE)
3. `[u8; payload_len]` (user_id + sensitive_context blob)

## Journal (public receipt only)

- 1 byte: decision (1 = approve, 0 = reject)
- 32 bytes: SHA-256 hash of full private input (blinding; no plaintext)

Policy: `approve = (trade_amount <= 10_000)`.
