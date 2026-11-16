#!/usr/bin/env bash
# Prints why the test database of a `go-db` job may have stopped: the container's state (running or exited, killed for
# memory or not, exit code), the end of its log, and the runner's memory, disk and kernel kill messages. The job runs it
# when a step failed, so a test that could not reach the database explains itself in the job's log
# (docs/ci.md, "When the test database stops").
# Usage: database-evidence.sh <cockroach|postgres>   (the container name start-cockroach.sh or start-postgres.sh gave)
# It never fails: every command is best effort, and the job is already red.
set -uo pipefail

name=${1:?the container name: cockroach or postgres}

echo "::group::The test database container ($name)"
docker inspect --format 'status={{.State.Status}} oom_killed={{.State.OOMKilled}} exit_code={{.State.ExitCode}} started={{.State.StartedAt}} finished={{.State.FinishedAt}}' "$name" 2>&1 || true
docker stats --no-stream --format 'memory={{.MemUsage}} cpu={{.CPUPerc}}' "$name" 2>&1 || true
echo "::endgroup::"

echo "::group::The last lines of the database's log"
docker logs --tail 200 "$name" 2>&1 || true
echo "::endgroup::"

echo "::group::The runner's memory and disk"
free -m 2>&1 || true
df -h / /dev/shm 2>&1 || true
echo "::endgroup::"

echo "::group::Processes the kernel killed for memory"
sudo dmesg --ctime 2>/dev/null | grep -i -E 'out of memory|oom-kill|killed process' | tail -n 20 || echo "none in the kernel log"
echo "::endgroup::"
