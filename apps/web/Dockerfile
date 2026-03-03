# Multi-stage Dockerfile for Vericore OS Next.js frontend (standalone output).
# Build from repo root: docker build -f apps/web/Dockerfile .
# Uses pnpm and copies .next/standalone + .next/static for minimal production image.

# ---- Builder ----
FROM node:22-bookworm-slim AS builder

WORKDIR /build

# Install pnpm and copy workspace root for monorepo install.
RUN corepack enable pnpm
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
COPY packages/sdk-node/package.json packages/sdk-node/
COPY packages/sdk-node/ packages/sdk-node/
COPY apps/web/ apps/web/

# Install dependencies (frozen lockfile for reproducible builds).
RUN pnpm install --frozen-lockfile

# Build Next.js with standalone output.
ENV NEXT_TELEMETRY_DISABLED=1
RUN pnpm run build --filter=v3r1c0r3-web

# ---- Runner ----
FROM node:22-bookworm-slim AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

RUN addgroup --system --gid 1001 nodejs && adduser --system --uid 1001 nextjs

# Copy standalone server and static assets.
COPY --from=builder --chown=nextjs:nodejs /build/apps/web/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /build/apps/web/.next/static ./apps/web/.next/static
COPY --from=builder /build/apps/web/public ./apps/web/public

USER nextjs

EXPOSE 3000

CMD ["node", "apps/web/server.js"]
