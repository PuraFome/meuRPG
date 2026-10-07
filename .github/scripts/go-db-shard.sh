#!/usr/bin/env bash
# Splits the backend's Go packages into the groups the `go-db` CI job runs in
# parallel, each against its own CockroachDB. Run it from backend/.
#
#   go-db-shard.sh <group>   print the group's packages, one per line
#   go-db-shard.sh --check   fail unless the groups together are exactly
#                            `go list ./...` (nothing skipped, nothing twice)
#
# The heavy packages are named by hand, from the per-package times of the
# 07/10/2026 run (play 509 s, maps 499 s, migrations 258 s, characters 203 s,
# progression 135 s, identity 112 s, with all of them sharing one database). The group "rest"
# is everything not named here, so a NEW package lands in "rest" on its own
# and is never skipped. If a group gets too slow, move a package up.
set -euo pipefail

module=github.com/PuraFome/meuRPG/backend

case_packages() {
  case "$1" in
    play) echo "$module/internal/play $module/internal/play/live $module/internal/identity" ;;
    maps) echo "$module/internal/maps" ;;
    characters) echo "$module/internal/characters $module/migrations $module/internal/progression" ;;
    rest) echo "" ;;
    *) echo "unknown group: $1" >&2; return 1 ;;
  esac
}

named_groups="play maps characters"

all_packages() { go list ./... | sort; }

if [ "${1:-}" = "--check" ]; then
  all=$(all_packages)
  named=$(for g in $named_groups; do case_packages "$g" | tr ' ' '\n'; done | sort)
  # A named package that no longer exists (renamed, deleted) would silently
  # drop out of its group and run in "rest" instead: say so.
  missing=$(comm -13 <(echo "$all") <(echo "$named"))
  if [ -n "$missing" ]; then
    echo "::error::go-db-shard.sh names packages that do not exist:" >&2
    echo "$missing" >&2
    exit 1
  fi
  dup=$(echo "$named" | uniq -d)
  if [ -n "$dup" ]; then
    echo "::error::go-db-shard.sh puts a package in two groups:" >&2
    echo "$dup" >&2
    exit 1
  fi
  union=$( { echo "$named"; "$0" rest; } | sort)
  if [ "$union" != "$all" ]; then
    echo "::error::the go-db groups are not exactly 'go list ./...'" >&2
    diff <(echo "$all") <(echo "$union") >&2 || true
    exit 1
  fi
  echo "OK: $(echo "$all" | wc -l | tr -d ' ') packages, every one in exactly one group."
  exit 0
fi

group=${1:?usage: go-db-shard.sh <group>|--check}
if [ "$group" = rest ]; then
  named=$(for g in $named_groups; do case_packages "$g" | tr ' ' '\n'; done | sort)
  comm -23 <(all_packages) <(echo "$named")
else
  case_packages "$group" | tr ' ' '\n'
fi
