# syntax=docker/dockerfile:1

# ============================================================
# meuRPG — backend (NestJS) — multi-stage Docker image
# Build context: ./server  (see docker-compose.yml)
# Secrets (server/.env) are NOT copied into the image; they are
# injected at runtime via docker-compose `env_file`.
# ============================================================

# ---------- Build stage ----------
FROM node:22-alpine AS builder
WORKDIR /app

# Install all deps (incl. dev) so `tsc` and types are available.
# `npm ci` requires a committed package-lock.json. Some deploy branches
# (e.g. `main`) may not ship one yet, so fall back to `npm install` when
# the lockfile is absent instead of failing the whole build.
COPY package*.json ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi

COPY . .

# Compile with tsc (not `nest build`/webpack) so the full dist is emitted,
# including db/migrate.js which the startup entrypoint needs.
RUN npx tsc -p tsconfig.build.json \
  # migrate.ts resolves schema.sql relative to its own dir — copy it next to it
  && cp src/db/schema.sql dist/db/schema.sql

# ---------- Runtime stage ----------
FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production

# Production-only dependencies (same lockfile fallback as the builder stage)
COPY package*.json ./
RUN if [ -f package-lock.json ]; then npm ci --omit=dev; else npm install --omit=dev; fi \
  && npm cache clean --force

# Compiled app + schema.sql (migrate runs against DB on first boot)
COPY --from=builder /app/dist ./dist
COPY docker-entrypoint.sh ./

# Run as non-root for a smaller attack surface
RUN chmod +x docker-entrypoint.sh \
  && addgroup -S app && adduser -S app -G app \
  && chown -R app:app /app

USER app

EXPOSE 3000

# 1) Apply schema (idempotent CREATE TABLE IF NOT EXISTS)
# 2) Start the API, replacing the shell so signals (SIGTERM) reach node
ENTRYPOINT ["./docker-entrypoint.sh"]