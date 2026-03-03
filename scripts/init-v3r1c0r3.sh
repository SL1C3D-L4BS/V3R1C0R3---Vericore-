#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

die() {
  echo "ERROR: $*" >&2
  exit 1
}

log() {
  echo "+ $*"
}

usage() {
  cat <<'EOF'
init-v3r1c0r3.sh

Scaffold a clean V3R1C0R3 monorepo (pnpm + Turborepo) with:
  - .cursor/rules/*.mdc (MDC pattern)
  - docs/architecture/decisions/001-causal-consistency-lsn.md (MADR)
  - package.json, pnpm-workspace.yaml, turbo.json
  - apps/* and packages/* boundaries

Safety model (Quarantine and Abort):
  - If the target directory contains any entries other than .git, the script aborts.
  - Use --quarantine to move existing entries (excluding .git) into a timestamped archive directory.

Usage:
  ./init-v3r1c0r3.sh [--target DIR] [--quarantine]

Examples:
  ./init-v3r1c0r3.sh --target ./sl1c3d-l4bs-v3r1c0r3
  ./init-v3r1c0r3.sh --target ./repo --quarantine
EOF
}

TARGET="."
QUARANTINE=false

while (($#)); do
  case "${1:-}" in
    --target)
      shift
      TARGET="${1:-}"
      [[ -n "${TARGET}" ]] || die "--target requires a directory argument"
      ;;
    --quarantine)
      QUARANTINE=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "Unknown argument: ${1}. Use --help."
      ;;
  esac
  shift
done

timestamp() {
  date +"%Y%m%d-%H%M%S"
}

ensure_dir() {
  local dir="$1"
  if [[ -e "$dir" && ! -d "$dir" ]]; then
    die "Target exists but is not a directory: $dir"
  fi
  if [[ ! -d "$dir" ]]; then
    log "mkdir -p \"$dir\""
    mkdir -p "$dir"
  fi
}

abspath() {
  local p="$1"
  (cd "$p" >/dev/null 2>&1 && pwd) || return 1
}

ensure_dir "$TARGET"
TARGET="$(abspath "$TARGET")" || die "Unable to resolve absolute path for target: $TARGET"

STAGING=""
cleanup() {
  local exit_code=$?
  trap - EXIT
  if [[ -n "${STAGING}" && -d "${STAGING}" ]]; then
    log "rm -rf \"${STAGING}\""
    rm -rf "${STAGING}"
  fi
  exit "${exit_code}"
}
trap cleanup EXIT

check_or_quarantine_target() {
  local target="$1"
  local quarantine="$2"
  local -a items=()
  local -a non_git=()

  shopt -s dotglob nullglob
  items=("${target}"/*)
  shopt -u dotglob nullglob

  for p in "${items[@]}"; do
    local base
    base="$(basename "$p")"
    if [[ "$base" == ".git" ]]; then
      continue
    fi
    non_git+=("$p")
  done

  if ((${#non_git[@]} == 0)); then
    return 0
  fi

  if [[ "$quarantine" != "true" ]]; then
    echo "Refusing to scaffold into a non-empty directory: ${target}" >&2
    echo "Found existing entries (excluding .git):" >&2
    for p in "${non_git[@]}"; do
      echo "  - $(basename "$p")" >&2
    done
    echo "Run again with --quarantine to archive and continue." >&2
    exit 1
  fi

  local archive="${target}/.v3r1c0r3-archive-$(timestamp)"
  log "mkdir -p \"${archive}\""
  mkdir -p "${archive}"

  for p in "${non_git[@]}"; do
    log "mv \"${p}\" \"${archive}/\""
    mv "${p}" "${archive}/"
  done
}

check_or_quarantine_target "${TARGET}" "${QUARANTINE}"

STAGING="${TARGET}/.v3r1c0r3-staging-$(timestamp)"
log "mkdir -p \"${STAGING}\""
mkdir -p "${STAGING}"

write_file() {
  local path="$1"
  local dir
  dir="$(dirname "$path")"
  log "mkdir -p \"${dir}\""
  mkdir -p "${dir}"
  log "write \"${path}\""
  cat >"${path}"
}

## Directories
log "mkdir -p \"${STAGING}/.cursor/rules\""
mkdir -p "${STAGING}/.cursor/rules"
log "mkdir -p \"${STAGING}/docs/architecture/decisions\""
mkdir -p "${STAGING}/docs/architecture/decisions"

log "mkdir -p \"${STAGING}/apps/web\" \"${STAGING}/apps/api\""
mkdir -p "${STAGING}/apps/web" "${STAGING}/apps/api"

log "mkdir -p \"${STAGING}/packages/db\" \"${STAGING}/packages/mcp-flight-recorder\" \"${STAGING}/packages/guardrails\" \"${STAGING}/packages/zkp-guest\""
mkdir -p "${STAGING}/packages/db" "${STAGING}/packages/mcp-flight-recorder" "${STAGING}/packages/guardrails" "${STAGING}/packages/zkp-guest"

## Cursor MDC rules
write_file "${STAGING}/.cursor/rules/000-v3r1c0r3-core.mdc" <<'EOF'
---
description: "Global architectural constraints for V3R1C0R3. Applies to all files."
globs:
  - "**/*"
alwaysApply: true
---
# V3R1C0R3 Core Directives

- **Integrity First:** Every state-changing operation MUST route through the `packages/mcp-flight-recorder`. No direct database writes from the web or API layers.
- **Secrets Management:** NO static secrets. Never use `os.Getenv` or `process.env` for production keys. All secrets are injected dynamically via Infisical/SPIFFE at runtime.
- **Cryptographic Blinding:** If a task touches `packages/zkp-guest`, plaintext PII is strictly forbidden from entering the zkVM as a public input.
- **Strict Boundaries:** `mcp-flight-recorder` and `guardrails` are immutable internal packages.
EOF

write_file "${STAGING}/.cursor/rules/101-go-monolith.mdc" <<'EOF'
---
description: "Rules for Go API, Database interactions, and LibSQL."
globs:
  - "apps/api/**/*.go"
  - "packages/db/**/*.go"
---
# Go Monolith & LibSQL Constraints

- **Language Standard:** Use Go 1.24+ idioms.
- **Database Concurrency:** Enforce `PRAGMA journal_mode=WAL;`. You must implement a dual-pool strategy: 1 write connection (IMMEDIATE locking), N read connections.
- **The LSN-Wait Rule (Article 14):** For high-stakes execution, you CANNOT write fire-and-forget code. The worker MUST block using `db.WaitForLSN(commit_lsn)` before executing, or fallback to the primary sqld.
- **Stateless Routing:** Assume BGP Anycast is active. Do not store in-memory session state in the Go application layer.
EOF

write_file "${STAGING}/.cursor/rules/102-frontend-telemetry.mdc" <<'EOF'
---
description: "Rules for the Next.js frontend, dashboards, and visualizations."
globs:
  - "apps/web/**/*.tsx"
  - "apps/web/**/*.ts"
---
# Frontend & Dashboard Standards

- **Framework:** Next.js 19 App Router with strict React Server Components (RSC) for sensitive auth flows.
- **Data Visualization:** For the Article 72 telemetry dashboards and Merkle Mountain Range (MMR) peak monitoring, utilize **Chart.js**. Build lightweight, enterprise-grade graphical representations of the audit logs without bleeding client state into the RSC payload.
- **Hardware Auth:** All approval state transitions must implement FIDO2/WebAuthn API calls to capture the hardware-backed assertion.
- **Trace Continuity:** Ensure `@opentelemetry/instrumentation-fetch` propagates the `traceparent` header to the backend API.
EOF

## ADR (MADR-style markdown)
write_file "${STAGING}/docs/architecture/decisions/001-causal-consistency-lsn.md" <<'EOF'
# ADR 001: Causal Consistency via LSN for Article 14 Double-Verification

## Context and Problem Statement
Multi-actor double-verification workflows (EU AI Act Article 14) create a replication race condition. When Approver 2 commits a state change to the self-hosted sqld primary, the execution worker (running on a distributed bare-metal node) may read from an embedded LibSQL replica that has not yet synced, leading to false anomalies or dropped executions.

## Decision Drivers
- Must support distributed nodes under BGP Anycast.
- Must guarantee execution only happens on cryptographically verified state.
- Time-based polling fallbacks are unreliable and unacceptable.

## Considered Options
1. Direct primary reads for all high-stakes actions.
2. Token-based causal consistency (LSN wait).

## Decision Outcome
Chosen option: **Token-based causal consistency (LSN wait)**. Upon the final approval commit, the primary issues a Log Sequence Number (LSN). The execution worker receives this token and strictly blocks execution until its local embedded replica confirms synchronization up to or beyond this LSN. If a 500ms timeout occurs, it falls back to a primary read.

## Consequences
- Good: Eliminates replication race conditions; maintains edge-node efficiency.
- Bad: Adds complexity to the worker queue payload and requires robust timeout handling in the Go API.
EOF

## pnpm + Turborepo foundation
write_file "${STAGING}/pnpm-workspace.yaml" <<'EOF'
packages:
  - "apps/*"
  - "packages/*"
EOF

write_file "${STAGING}/package.json" <<'EOF'
{
  "name": "sl1c3d-l4bs-v3r1c0r3",
  "private": true,
  "version": "0.1.0",
  "packageManager": "pnpm@9.14.2",
  "engines": {
    "node": ">=20"
  },
  "scripts": {
    "dev": "turbo run dev",
    "build": "turbo run build",
    "test": "turbo run test",
    "lint": "turbo run lint"
  },
  "devDependencies": {
    "turbo": "2.5.2"
  }
}
EOF

write_file "${STAGING}/turbo.json" <<'EOF'
{
  "$schema": "https://turbo.build/schema.json",
  "tasks": {
    "build": {
      "dependsOn": ["^build"]
    },
    "test": {
      "dependsOn": ["^test"]
    },
    "lint": {
      "dependsOn": ["^lint"]
    },
    "dev": {
      "cache": false,
      "persistent": true
    },
    "web#build": {
      "dependsOn": ["^build"],
      "outputs": [".next/**", "!.next/cache/**"]
    },
    "web#dev": {
      "cache": false,
      "persistent": true
    },
    "api#build": {
      "dependsOn": ["^build"],
      "inputs": ["$TURBO_DEFAULT$", "go.mod", "go.sum", "**/*.go"],
      "outputs": ["bin/**"]
    },
    "api#dev": {
      "cache": false,
      "persistent": true,
      "inputs": ["$TURBO_DEFAULT$", "go.mod", "go.sum", "**/*.go"]
    },
    "zkp-guest#build": {
      "dependsOn": ["^build"],
      "inputs": ["$TURBO_DEFAULT$", "Cargo.toml", "Cargo.lock", "**/*.rs"],
      "outputs": ["target/**"]
    }
  }
}
EOF

## Move staged contents into target
log "finalize: move staged contents into target"
shopt -s dotglob nullglob
for entry in "${STAGING}"/*; do
  log "mv \"${entry}\" \"${TARGET}/\""
  mv "${entry}" "${TARGET}/"
done
shopt -u dotglob nullglob

log "rmdir \"${STAGING}\""
rmdir "${STAGING}"
STAGING=""

log "done: V3R1C0R3 scaffold created in ${TARGET}"
