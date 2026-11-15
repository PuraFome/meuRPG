#!/usr/bin/env bash
# Starts the CockroachDB the Go integration tests run against: one node, insecure, in memory, published on
# 127.0.0.1:26257, with the background work a long-lived database needs turned off.
# The `go-db-shard` jobs of backend.yml (the groups behind the `go-db` check) run `go test -race` against it, one group of
# packages each, and so do those of `repeat-changed`: on a train, on main, nightly and by hand.
# The composite action `test-database` starts it for them (docs/ci.md, "Which database the tests run on").
# `flake-check` and `nightly-hunt` call this script directly.
# The image is the one backend.yml pins by digest (COCKROACH_IMAGE there). A `go-db` job runs the copy its run pulled once and
# saved (TEST_COCKROACH_IMAGE, which the step that starts the database passes from the output of the action load-database-image) and never pulls; `flake-check` and `nightly-hunt` pull
# the pinned image themselves (docs/ci.md, "How the test database image reaches the jobs").
# Usage: start-cockroach.sh   (from anywhere; needs docker)
set -euo pipefail

image=$("$(dirname "$0")/database-image.sh" ref cockroach)
pull=()
if [ -n "${TEST_COCKROACH_IMAGE:-}" ]; then
  image=$TEST_COCKROACH_IMAGE
  pull=(--pull=never)
fi
echo "CockroachDB image: $image (${pull[*]:-pulled by docker run})"

# The tmpfs is the node's external I/O directory: a test database is a RESTORE of a backup of the template kept there (nodelocal).
docker run ${pull[@]+"${pull[@]}"} --detach --name cockroach --publish 127.0.0.1:26257:26257 \
  --tmpfs /cockroach-io:rw,mode=1777 \
  "$image" start-single-node --insecure \
  --store=type=mem,size=4GiB --cache=512MiB --max-sql-memory=2GiB --external-io-dir=/cockroach-io
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
