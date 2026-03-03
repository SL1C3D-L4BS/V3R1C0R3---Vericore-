#!/usr/bin/env bash
# Local E2E simulation: primary + replica SQLite DBs, Go API, optional Next.js.
# Run from repo root. Requires: sqlite3, go, pnpm (for web).

set -euo pipefail
cd "$(dirname "$0")/.."
ROOT="$PWD"

die() { echo "ERROR: $*" >&2; exit 1; }
log() { echo "+ $*"; }

# --- 1. Create primary.db and replica.db with audit schema ---
MIGRATION="$ROOT/packages/db/migrations/001_init_audit.sql"
[[ -f "$MIGRATION" ]] || die "Migration not found: $MIGRATION"

if ! command -v sqlite3 &>/dev/null; then
  die "sqlite3 not found. Install with: brew install sqlite3"
fi

for db in primary.db replica.db; do
  if [[ ! -f "$ROOT/$db" ]]; then
    log "Creating $db from migration"
    sqlite3 "$ROOT/$db" < "$MIGRATION"
  else
    log "Using existing $db"
  fi
done

# Seed verification_queue so POST /api/v1/agent/action can UPDATE a row (id=1).
log "Seeding verification_queue (id=1) if missing"
sqlite3 "$ROOT/primary.db" "
  INSERT OR IGNORE INTO verification_queue (id, state, payload_json)
  VALUES (1, 'committed', '{}');
"
sqlite3 "$ROOT/replica.db" "
  INSERT OR IGNORE INTO verification_queue (id, state, payload_json)
  VALUES (1, 'committed', '{}');
"

# --- 2. Start Go API (listens on :8080) ---
log "Starting API on :8080 (CWD=$ROOT)"
export API_ADDR="${API_ADDR:-:8080}"
(
  cd "$ROOT"
  go run ./apps/api
) &
API_PID=$!
trap 'kill $API_PID 2>/dev/null || true' EXIT

# Wait for API health
for i in {1..30}; do
  if curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health 2>/dev/null | grep -q 200; then
    log "API healthy at http://localhost:8080"
    break
  fi
  [[ $i -eq 30 ]] && die "API did not become healthy in time"
  sleep 0.2
done

# --- 3. Optional: start Next.js (default: skip unless E2E_WEB=1) ---
if [[ "${E2E_WEB:-0}" == "1" ]]; then
  log "Starting Next.js (E2E_WEB=1)"
  (cd "$ROOT/apps/web" && pnpm dev) &
  WEB_PID=$!
  trap 'kill $API_PID $WEB_PID 2>/dev/null || true' EXIT
  log "Web dev server starting at http://localhost:3000"
fi

log "Local E2E ready. API: http://localhost:8080"
log "  health: curl http://localhost:8080/health"
log "  ready:  curl http://localhost:8080/ready"
log "  action: curl -X POST http://localhost:8080/api/v1/agent/action -H 'Content-Type: application/json' -d '{\"action_id\":\"test\",\"decision\":\"approved\",\"reasoning\":\"e2e\"}'"
wait $API_PID
