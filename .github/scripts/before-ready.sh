#!/usr/bin/env bash
# Runs the Go tests that a branch's own change makes stale: those that compare with a
# golden, a hash, a frozen table or a stored fixture, and the inventory tests that list
# the service methods, the error reasons, the authorization matrices and the event kinds.
# A session runs it before it writes `ready:` (docs/testing.md#before-ready):
#
#   make before-ready
#
# The tests, their packages and the command that regenerates each file are in one place,
# backend/internal/archtest/before_ready.txt. On a failure the script prints that command
# next to the failed test.
#
# By default it runs only the lines of the list that the change can affect: the working tree
# against the merge base with origin/main, and the packages and inputs each line reads
# (backend/cmd/beforeready). A change that reaches none of them (web, docs) runs nothing and
# exits 0. When the change cannot be compared, every line runs.
#
# Environment:
#   MEURPG_TEST_DATABASE_URL  required: the tests that need a database fail without it
#   BEFORE_READY_JOBS         packages tested at once (default 4)
#   BEFORE_READY_LIST         another list file, to try a few tests (same format)
#   BEFORE_READY_ALL          1: run every line of the list, whatever the change
#   BEFORE_READY_ONLY         an extended regex: test only the packages whose directory
#                             matches it (a rerun of what failed, e.g. 'internal/rules$'). It
#                             narrows the selected lines, or every line with BEFORE_READY_ALL=1
#
# Bash 3.2 compatible (macOS's /bin/bash): no associative arrays, no mapfile, no wait -n.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
list="${BEFORE_READY_LIST:-$root/backend/internal/archtest/before_ready.txt}"
tab="$(printf '\t')"

# The tab-separated `run` lines of the list: package, regex, regenerate command.
run_lines() { awk -F'\t' '$1 == "run" { print $2 "\t" $3 "\t" $4 }' "$list"; }

# The -run regex of one package: the regexes of its lines joined by `|`.
regex_of() { run_lines | awk -F'\t' -v pkg="$1" '$1 == pkg { printf "%s(%s)", sep, $2; sep = "|" }'; }

# One package: the mode xargs calls, so the packages run side by side and each keeps its
# own output.
if [ "${1:-}" = "--one" ]; then
  out="$3/$(echo "$2" | tr '/' '_').out"
  cd "$root/backend"
  if go test -count=1 -timeout 45m -run "$(regex_of "$2")" "./$2" >"$out" 2>&1; then
    echo 0 >"$out.status"
  else
    echo 1 >"$out.status"
  fi
  exit 0
fi

[ -r "$list" ] || { echo "before-ready: cannot read $list" >&2; exit 2; }
only="${BEFORE_READY_ONLY:-.}"
echo | grep -E -- "$only" >/dev/null 2>&1 || [ $? -eq 1 ] || { echo "before-ready: BEFORE_READY_ONLY '$only' is not a valid regex" >&2; exit 2; }
run_lines | cut -f1 | sort -u | grep -E -- "$only" >/dev/null || { echo "before-ready: BEFORE_READY_ONLY matches no package of the list" >&2; exit 2; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
started="$(date +%s)"

# The lines to run: the change decides (see the header), BEFORE_READY_ALL=1 takes them all. The
# children of --one read the chosen list through BEFORE_READY_LIST.
all_flag=""
[ "${BEFORE_READY_ALL:-}" = 1 ] && all_flag="-all"
if (cd "$root/backend" && go run ./cmd/beforeready -list "$list" -out "$tmp/selected.txt" $all_flag); then
  list="$tmp/selected.txt"
else
  echo "before-ready: the selection of the lines failed: running every line" >&2
fi
export BEFORE_READY_LIST="$list"

packages="$(run_lines | cut -f1 | sort -u | grep -E -- "$only" || true)"
if [ -z "$packages" ]; then
  [ "$only" = . ] || echo "before-ready: nothing to run: BEFORE_READY_ONLY '$only' matches none of the selected packages (BEFORE_READY_ALL=1 selects every line)"
  exit 0
fi
count="$(echo "$packages" | wc -l | tr -d ' ')"

if [ -z "${MEURPG_TEST_DATABASE_URL:-}" ]; then
  url="${TEST_DATABASE_URL:-postgresql://root@localhost:26258/defaultdb?sslmode=disable}"
  cat >&2 <<EOF
before-ready: MEURPG_TEST_DATABASE_URL is not set. Most of these tests need the test
database, and without it they would skip and pass for nothing. Start it and point the tests
at it:

  make db-test-start
  export MEURPG_TEST_DATABASE_URL='$url'
  make before-ready

(docs/testing.md#integration-tests; any other CockroachDB works the same way.)
EOF
  exit 2
fi
# A test that needs the database fails instead of skipping when it is missing.
export MEURPG_REQUIRE_DB=1

echo "before-ready: running the golden, frozen, fixture and inventory tests of $count packages (backend/internal/archtest/before_ready.txt)"
echo "$packages" | xargs -P "${BEFORE_READY_JOBS:-4}" -I{} bash "${BASH_SOURCE[0]}" --one {} "$tmp"

# regen_for prints the command that rewrites the file of one failed test: the one of the
# first `run` line of its package whose regex matches the name.
regen_for() {
  run_lines | awk -F'\t' -v pkg="$1" '$1 == pkg { print $2 "\t" $3 }' | while IFS="$tab" read -r re cmd; do
    if echo "$2" | grep -Eq -- "$re"; then
      echo "${cmd//\{test\}/$2}"
      break
    fi
  done
}

failed=0
for pkg in $packages; do
  out="$tmp/$(echo "$pkg" | tr '/' '_').out"
  status="$(cat "$out.status" 2>/dev/null || echo missing)"
  if [ "$status" = missing ]; then
    failed=1
    echo
    echo "=== FAILED: $pkg: the test run left no result (the script was interrupted, or go could not start): run it again."
    continue
  fi
  if [ "$status" = 0 ]; then
    grep -E '^(ok|\?)' "$out" || true
    continue
  fi
  failed=1
  echo
  echo "=== FAILED: $pkg"
  cat "$out"
  names="$(grep -E '^--- FAIL: ' "$out" | awk '{print $3}' | sort -u || true)"
  if [ -z "$names" ]; then
    echo "--- $pkg failed without a failed test (it did not build, panicked or timed out): read the output above."
    continue
  fi
  for name in $names; do
    cmd="$(regen_for "$pkg" "$name")"
    echo "--- $pkg $name"
    if [ -n "$cmd" ]; then
      echo "    regenerate, only if the change is intended: $cmd"
    else
      echo "    backend/internal/archtest/before_ready.txt has no regenerate command for it"
    fi
  done
done

seconds=$(($(date +%s) - started))
if [ "$failed" -ne 0 ]; then
  cat <<EOF

before-ready: FAILED after ${seconds}s. A failure here means the branch changed what the test pins.
If the change is intended, run the command printed under the test, then:
  - READ the diff of every file it rewrote (git diff) before you commit it: a regenerated
    file that nobody read pins whatever the code does, a bug included.
  - Run a NEW golden test with -count=5, because a Go map's order is random:
    cd backend && go test ./internal/<package> -run '^<Test>\$' -count=5
  - Run make before-ready again.
If the change is not intended, fix the code.
EOF
  exit 1
fi
echo "before-ready: ok in ${seconds}s"
