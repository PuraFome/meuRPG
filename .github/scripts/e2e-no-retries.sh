#!/usr/bin/env bash
# Fails unless e2e/playwright.config.ts sets `retries: 0` and nothing else sets retries.
# A retry hides a flaky test (docs/ci.md, "Flaky tests"): the fix is the test or the app.
# Usage: e2e-no-retries.sh [config]   (default: e2e/playwright.config.ts)
set -euo pipefail

config="${1:-e2e/playwright.config.ts}"

# Every line that mentions retries, comments aside.
lines=$(grep -nE '^[^/]*\bretries\b' "$config" || true)
count=$(grep -c . <<<"$lines" || true)

if [ "$count" -ne 1 ] || ! grep -Eq '^[0-9]+:[[:space:]]*retries:[[:space:]]*0,?[[:space:]]*$' <<<"$lines"; then
  echo "::error file=${config}::retries must be set once, to 0 (no retries: a flaky test is fixed, not retried). Found:" >&2
  echo "${lines:-<nothing>}" >&2
  exit 1
fi
echo "retries is 0"
