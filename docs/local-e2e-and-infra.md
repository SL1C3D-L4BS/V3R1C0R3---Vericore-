# Local E2E and Infrastructure Bootstrapping

Before bare-metal Confidential VMs or BGP routing, prove the End-to-End flow locally.

---

## 1. Cryptographic Baseline (SLSA L4 ground truth)

Already done:

- `git init` and baseline commit: `chore: establish V3R1C0R3 architectural baseline`
- Remote: `git@github.com:SL1C3D-L4BS/V3R1C0R3---Vericore-.git`, branch `main`
- Push with SL1C3D key; this repo is the source of truth.

---

## 2. Local E2E Simulation (The Crucible)

Simulates edge nodes, primary DB, and Go API locally using file-based SQLite as primary and replica.

### Prerequisites

- **sqlite3** (CLI): `brew install sqlite3`
- **Go 1.22+**
- **pnpm** (for Next.js, optional for API-only E2E)

### One-command local stack

From repo root:

```bash
./scripts/e2e-local.sh
```

This will:

1. Create `primary.db` and `replica.db` in the repo root (if missing) using `packages/db/migrations/001_init_audit.sql`.
2. Seed `verification_queue` with row `id=1` so the API’s RYOW/fallback path can run.
3. Start the Go API on **http://localhost:8080** (health: `/health`, readiness: `/ready`).
4. Optionally start Next.js: `E2E_WEB=1 ./scripts/e2e-local.sh` (dev server on port 3000).

### Manual steps (alternative)

```bash
# Create DBs
sqlite3 primary.db < packages/db/migrations/001_init_audit.sql
sqlite3 replica.db  < packages/db/migrations/001_init_audit.sql

# Seed verification_queue (so POST /api/v1/agent/action has a row to update)
sqlite3 primary.db "INSERT OR IGNORE INTO verification_queue (id, state, payload_json) VALUES (1, 'committed', '{}');"
sqlite3 replica.db  "INSERT OR IGNORE INTO verification_queue (id, state, payload_json) VALUES (1, 'committed', '{}');"

# Start API (from repo root; primary.db / replica.db must be in CWD)
go run ./apps/api

# In another terminal: frontend (optional)
pnpm --filter v3r1c0r3-web dev
```

### Quick API checks

- `curl http://localhost:8080/health` → 200 and DB reachable.
- `curl -X POST http://localhost:8080/api/v1/agent/action -H 'Content-Type: application/json' -d '{"action_id":"test","decision":"approved","reasoning":"e2e"}'` → 200 and audit path exercised (guardrail + flight recorder).

This local setup does **not** run real LibSQL replication or WebAuthn FIDO2; it proves the API, DB schema, RYOW fallback, and audit boundary. For Article 14 causal consistency and FIDO2, use a real sqld primary + replicas and a browser with WebAuthn.

---

## 3. Infrastructure Bootstrapping

External dependencies for moving beyond local file-based DBs and static env.

### Infisical (dynamic secrets injection)

- **Purpose:** No static secrets in repo or env; inject at runtime (KEK/DEK, Turso tokens, etc.).
- **Local setup:**
  1. Create a project at [infisical.com](https://infisical.com) (or self-host).
  2. Install CLI: `brew install infisical/get-cli/infisical`
  3. Log in: `infisical login`
  4. Run the API (or any process) with injected env:  
     `infisical run -- go run ./apps/api`  
     Or export for the shell: `eval "$(infisical export --env=dev)"` then `go run ./apps/api`.
- **Next step:** Replace any `os.Getenv("LIBSQL_AUTH_TOKEN")` (or similar) in the app with Infisical-backed env so production never reads secrets from disk.

### LibSQL / Turso (primary database)

Two ways to move off local `primary.db` / `replica.db`:

**A. Local sqld (LibSQL server)**

- Acts as primary; replicas can sync from it.
- Install: see [libsql/docs](https://github.com/libsql/libsql/blob/main/docs/BUILD-RUN.md) (or Turso CLI for local dev).
- Example (conceptual):  
  `sqld -a 127.0.0.1:8081`  
  Then point the app’s primary DSN at `http://127.0.0.1:8081` (or the URL your sqld build uses). Use embedded replicas or separate sqld replicas for LSN/causal consistency when you implement that client path.

**B. Turso (hosted LibSQL)**

- Create a DB in the Turso dashboard (or `turso db create <name>`).
- Get URL and token: `turso db show <name> --url`, `turso db tokens create <name>`.
- Put them in `.env` (for local dev) or in Infisical (for production):  
  `LIBSQL_URL=...`, `LIBSQL_AUTH_TOKEN=...`
- When the Go app uses the LibSQL client (with WAL / LSN support), point it at `LIBSQL_URL` for the primary and use `LIBSQL_REPLICA_PATH` (or equivalent) for the local embedded replica.

### Summary

| Dependency   | Role                          | Local bootstrap                                              |
|-------------|--------------------------------|--------------------------------------------------------------|
| **Infisical** | Dynamic secrets (no static keys) | Account + CLI; `infisical run -- go run ./apps/api`         |
| **LibSQL/Turso** | Primary (and later replicas)   | Local: sqld or file DB. Cloud: Turso DB + URL + token in Infisical |

After bootstrap, the next steps are: wire the Go API to `LIBSQL_*` when using Turso/sqld, and keep all secrets in Infisical (or your chosen secret manager) for production.
