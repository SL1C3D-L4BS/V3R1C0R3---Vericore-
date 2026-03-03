# Vericore OS: The High-Assurance AI Control Plane

**The API-first compliance primitive for the EU AI Act & ISO 42001—enabling legally insurable autonomous agents.**

---

## Executive Summary

**Vericore OS** is a high-assurance AI control plane that turns every state-changing action into a cryptographically verifiable, tamper-evident audit trail. It is built for **API-first** deployment: B2B tenants authenticate with API keys; every request is scoped to a tenant and recorded in a **Merkle Mountain Range (MMR)** before execution. Guardrail interventions are logged (no shadow blocks); human approvals are enforced via **FIDO2/WebAuthn**; and **read-your-own-writes (RYOW)** causal consistency ensures execution workers never act on stale state. The design targets **EU AI Act** (Articles 12, 14, 72), **ISO 42001**, and **GDPR**-aligned cryptographic erasure—so high-risk AI systems can be operated, monitored, and defended in hostile audits and insured with confidence.

---

## Core Architecture: The 4 Pillars

| Pillar | Name | What It Delivers |
|--------|------|------------------|
| **1** | **Cryptographic Integrity** | Stateless Merkle Mountain Range Flight Recorder with write-ahead batching capable of thousands of RPS. |
| **2** | **Distributed Efficiency** | LibSQL RYOW causal consistency with primary fallback. |
| **3** | **Data Privacy & Topography** | ClickHouse tiered storage tombstones; RISC Zero ZKP blinding. |
| **4** | **Multi-Tenant Security** | API-key auth, Tetragon eBPF guardrails, FIDO2 WebAuthn. |

---

### Pillar 1: Cryptographic Integrity

- **Stateless MMR Flight Recorder** (`packages/mcp-flight-recorder`): append-only audit log with peak-merging and root sealing; no in-memory MMR state across requests.
- **Write-ahead batching**: HTTP handlers enqueue events; a background worker batches up to 500 requests or 50 ms, then runs a single transaction—**capable of thousands of RPS** without fsync-per-request penalty.
- **Single-writer pool** (`packages/db`): LibSQL primary with `MaxOpenConns(1)` so writes are serialized and SQLITE_BUSY is avoided.
- **Guardrails + kill-switch**: invalid or non-approved payloads are blocked; **every** block is appended to the MMR (Article 72; no silent drops).

### Pillar 2: Distributed Efficiency

- **Dual DB pools**: primary (write), replica (read). Go API (`apps/api`) uses both for RYOW.
- **WaitForCommit + ExecuteWithFallback** (`packages/db/ryow.go`): workers wait for the replica to sync verification-queue state (or 500 ms timeout), then run the job; on timeout they **fall back to the primary** so availability is preserved.
- **LSN-wait simulation**: header `X-RYOW-Simulate-Lag` forces replica lag locally for testing the fallback path in the Crucible.

### Pillar 3: Data Privacy & Topography

- **ClickHouse tiered storage** (`packages/db/tiered_storage.go`): Hot (LibSQL) → Warm (ClickHouse) → Cold (S3). A **tombstone hash** for each partition is computed and persisted in LibSQL *before* `ALTER TABLE ... MOVE PARTITION TO VOLUME 'cold'`, so MMR proofs remain valid across the lifecycle.
- **RISC Zero ZKP blinding** (`packages/zkp`): only decision and input hash enter the proof; plaintext PII never enters the zkVM.
- **Tetragon eBPF** (`deploy/tetragon`): policy-driven observability and audit at the kernel layer.

### Pillar 4: Multi-Tenant Security

- **API-key authentication** (`packages/auth`): `Authorization: Bearer <API_KEY>`; hashed keys map to tenant IDs; tenant ID is injected into the request context and attached to every audit event and MMR leaf.
- **Tetragon eBPF**: kernel-level guardrails and process/network audit.
- **FIDO2/WebAuthn** (`apps/web`): double-verification UI with confirmation phrase and hardware authenticator; approval payload includes `fido_signature` so the guardrail accepts the action.

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| **Backend** | **Go 1.24** (Chi, std lib) |
| **Frontend** | **Next.js 19** (App Router, RSC) |
| **Hot DB** | **LibSQL** / SQLite (file or sqld) |
| **Warm/Cold** | **ClickHouse** (MergeTree, tiered policy) |
| **ZKP** | **Rust** + **RISC Zero** (guest + host FFI) |
| **Observability** | OpenTelemetry, Tetragon eBPF |

---

## Getting Started (The Crucible)

### Prerequisites

- **Go** 1.22+, **sqlite3**, **pnpm** (optional, for Next.js).

### 1. Run the local E2E simulation

From the repo root:

```bash
./scripts/e2e-local.sh
```

This creates `primary.db` and `replica.db`, seeds the verification queue, and starts the Go API at **http://localhost:8080**. Optional: bring up the Next.js app as well:

```bash
E2E_WEB=1 ./scripts/e2e-local.sh
```

### 2. Call the API (with tenant auth)

All state-changing requests to `/api/v1/agent/action` require a valid API key. Example:

```bash
curl -X POST http://localhost:8080/api/v1/agent/action \
  -H "Authorization: Bearer sk_test_123" \
  -H "Content-Type: application/json" \
  -d '{"decision":"approved","action_id":"test-1","reasoning":"E2E check"}'
```

Use **`Authorization: Bearer sk_test_123`** for the default test tenant (`tenant_alpha`). Missing or invalid keys receive **401 Unauthorized**.

### 3. Chaos load test

With the API running (e.g. in another terminal: `go run ./apps/api`):

```bash
go run ./scripts/load_test
```

The script sends **5,000 concurrent POSTs** to `/api/v1/agent/action` (using the test key), then reports HTTP 200/403/500, locked/busy counts, RPS, and a post-attack integrity check via `GET /api/v1/telemetry/stats`.

---

## Deployment & Supply Chain

### Deployment: Bare-metal and HA

Production targets **bare-metal** or VM fleets with high availability:

- **Kamal**: deploy the Go API and Next.js app as containers; rolling updates and lifecycle hooks.
- **HAProxy** (`deploy/haproxy`): L7 load balancing and health checks (e.g. `/health` against the primary DB); graceful drain on deploy.

The Go API remains **stateless**; any node can serve after the recorder’s batch channel. eBPF/Tetragon provides kernel-level policies for process and network audit.

### Supply Chain: SLSA L4 + Sigstore

- **SLSA L4** and **Sigstore keyless attestation**: the CI/CD pipeline (e.g. `.github/workflows/build-and-attest.yml`) builds, attests, and **keyless-signs** artifacts using Sigstore (cosign) OIDC. SBOM and attestations are produced so every artifact has a verifiable provenance chain—ground truth for the cryptographic baseline.

---

## Compliance Mapping

Regulatory and standard mappings (EU AI Act, ISO 42001, prEN 18286, Colorado AI Act, GDPR) are maintained in:

- **[docs/compliance-mapping.md](docs/compliance-mapping.md)** — canonical mapping of articles to components and evidence.

---

## Monorepo Layout

| Path | Description |
|------|-------------|
| `apps/api` | Go monolith (handlers, guardrails, flight recorder, tenant auth) |
| `apps/web` | Next.js 19 RSC frontend (verification queue, Article 72 dashboard) |
| `packages/auth` | API-key auth and tenant context (Bearer token → TenantID) |
| `packages/db` | LibSQL schema, migrations, RYOW, tiered storage, tombstones |
| `packages/mcp-flight-recorder` | MMR flight recorder (batch worker, peak-merge, Store interface) |
| `packages/guardrails` | Validator interface and strict schema kill-switch |
| `packages/zkp` | RISC Zero guest (Rust) and recursion |
| `docs/architecture` | ADRs and design notes |
| `scripts/e2e-local.sh` | Local Crucible (DBs + API ± web) |
| `scripts/load_test` | Chaos load tester (5k concurrent POSTs) |
