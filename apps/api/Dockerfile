# Multi-stage Dockerfile for Vericore OS Go API.
# Build from repo root: docker build -f apps/api/Dockerfile .
# CGO_ENABLED=1 for compatibility with go-sqlite3/libsql; runtime uses glibc (bookworm-slim).

# ---- Builder ----
FROM golang:1.22-bookworm AS builder

WORKDIR /build

# Copy workspace and module manifests (context = repo root).
COPY go.work ./
COPY go.mod ./
COPY apps/api/go.mod apps/api/
COPY packages/auth/go.mod packages/auth/
COPY packages/db/go.mod packages/db/
COPY packages/guardrails/go.mod packages/guardrails/
COPY packages/mcp-flight-recorder/go.mod packages/mcp-flight-recorder/

# Copy source.
COPY apps/api/ apps/api/
COPY packages/auth/ packages/auth/
COPY packages/db/ packages/db/
COPY packages/guardrails/ packages/guardrails/
COPY packages/mcp-flight-recorder/ packages/mcp-flight-recorder/

# Build with CGO for sqlite/libsql; strip and static link where possible.
ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64
RUN cd apps/api && go build -ldflags="-s -w" -o /api .

# ---- Production ----
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /api /app/api

EXPOSE 8080

ENTRYPOINT ["/app/api"]
