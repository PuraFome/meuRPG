#!/usr/bin/env bash
# Starts the PostgreSQL the Go integration tests run against in a preflight (docs/ci.md, "Which database the tests run
# on"): one server, no password, published on 127.0.0.1:5432, data in memory.
# SERIALIZABLE is the default isolation, as CockroachDB has it, and durability is off, as the test CockroachDB has it.
# Usage: start-postgres.sh   (from anywhere; needs docker)
set -euo pipefail

# The image is the one backend.yml pins by digest (POSTGRES_IMAGE there), from Amazon's public mirror of the official
# image because Docker Hub's anonymous pull limit hits GitHub's shared runners. A `go-db` job runs the copy its run
# pulled once and saved (TEST_POSTGRES_IMAGE, set by the action load-database-image) and never pulls; any other caller
# pulls the pinned image itself (docs/ci.md, "How the test database image reaches the jobs").
image=$("$(dirname "$0")/database-image.sh" ref postgres)
pull=()
if [ -n "${TEST_POSTGRES_IMAGE:-}" ]; then
  image=$TEST_POSTGRES_IMAGE
  pull=(--pull=never)
fi
echo "PostgreSQL image: $image (${pull[*]:-pulled by docker run})"

# max_connections and max_pred_locks_per_transaction size the server's bookkeeping of serializable transactions,
# which is shared by all its databases: with too little, many test binaries at once make it summarize the old ones and
# cancel statements with 40001 that no test conflicts with.
docker run ${pull[@]+"${pull[@]}"} --detach --name postgres --publish 127.0.0.1:5432:5432 \
  --env POSTGRES_USER=root --env POSTGRES_HOST_AUTH_METHOD=trust --env POSTGRES_DB=postgres \
  --tmpfs /var/lib/postgresql/data:rw \
  "$image" postgres \
  -c default_transaction_isolation=serializable \
  -c fsync=off -c synchronous_commit=off -c full_page_writes=off \
  -c max_connections=1000 \
  -c max_pred_locks_per_transaction=256
for _ in $(seq 1 60); do
  if docker exec postgres pg_isready -U root -d postgres >/dev/null 2>&1 \
    && docker exec postgres psql -U root -d postgres -Atc 'SELECT 1' >/dev/null 2>&1; then
    echo "PostgreSQL is ready."
    exit 0
  fi
  sleep 1
done
echo "::error::PostgreSQL was not ready after 60 seconds." >&2
docker logs postgres >&2
exit 1
