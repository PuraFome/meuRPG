#!/usr/bin/env bash
# Starts the CockroachDB the Go integration tests run against: one node, insecure, in memory, published on
# 127.0.0.1:26257, with the background work a long-lived database needs turned off.
# The `go-db-shard` jobs of backend.yml (the groups behind the `go-db` check) run `go test -race` against it, one group of
# packages each, and so do those of `repeat-changed`: on a train, on main, nightly and by hand.
# The composite action `test-database` starts it for them (docs/ci.md, "Which database the tests run on").
# `flake-check` and `nightly-hunt` call this script directly.
# The image is the one backend.yml names (COCKROACH_IMAGE there), so the digest lives in one place.
# Usage: start-cockroach.sh   (from anywhere; needs docker)
set -euo pipefail

workflow="$(dirname "$0")/../workflows/backend.yml"
image=$(grep -m1 -E '^[[:space:]]*COCKROACH_IMAGE:' "$workflow" | sed -E 's/^[[:space:]]*COCKROACH_IMAGE:[[:space:]]*//')
if [ -z "$image" ]; then
  echo "::error::COCKROACH_IMAGE not found in $workflow; this script no longer matches it." >&2
  exit 1
fi

docker run --detach --name cockroach --publish 127.0.0.1:26257:26257 \
  "$image" start-single-node --insecure \
  --store=type=mem,size=4GiB --cache=512MiB --max-sql-memory=2GiB
for _ in $(seq 1 60); do
  if docker exec cockroach cockroach sql --insecure --host=localhost:26257 -e 'SELECT 1' >/dev/null 2>&1; then
    docker exec cockroach cockroach sql --insecure --host=localhost:26257 \
      -e "SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false" \
      -e "SET CLUSTER SETTING kv.range_merge.queue.enabled = false" \
      -e "SET CLUSTER SETTING jobs.retention_time = '15s'" \
      -e "SET CLUSTER SETTING diagnostics.reporting.enabled = false" \
      -e "ALTER RANGE default CONFIGURE ZONE USING gc.ttlseconds = 600"
    echo "CockroachDB is ready."
    exit 0
  fi
  sleep 2
done
echo "::error::CockroachDB was not ready after 120 seconds." >&2
docker logs cockroach >&2
exit 1
