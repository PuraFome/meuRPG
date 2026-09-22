#!/bin/sh
# Startup entrypoint for the meuRPG backend container.
# Runs an idempotent DB migration, then execs the API so it receives signals.
set -e

echo "[entrypoint] applying database migration (idempotent)…"
node dist/db/migrate.js

echo "[entrypoint] starting API on :${API_PORT:-3000}"
exec node dist/main.js