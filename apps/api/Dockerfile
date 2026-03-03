# Multi-stage build: builder then distroless runtime.
# Build from repo root: docker build -f apps/api/Dockerfile .
# Final image contains only the Go binary; no shell, no package manager.

# ---- Builder ----
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy workspace and module manifests (context = repo root).
COPY go.work ./
COPY apps/api/go.mod apps/api/go.sum apps/api/
COPY packages/db/go.mod packages/db/
COPY packages/guardrails/go.mod packages/guardrails/
COPY packages/mcp-flight-recorder/go.mod packages/mcp-flight-recorder/

# Copy source.
COPY apps/api/ apps/api/
COPY packages/db/ packages/db/
COPY packages/guardrails/ packages/guardrails/
COPY packages/mcp-flight-recorder/ packages/mcp-flight-recorder/

# Build static binary from apps/api (CGO_ENABLED=0 for distroless).
ENV CGO_ENABLED=0 GOOS=linux
RUN cd apps/api && go build -ldflags="-s -w" -o /api .

# ---- Runtime ----
FROM gcr.io/distroless/static-debian12:nonroot

USER nonroot:nonroot
WORKDIR /app

COPY --from=builder /api /app/api

EXPOSE 8080

ENTRYPOINT ["/app/api"]
