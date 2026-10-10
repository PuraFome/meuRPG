#!/usr/bin/env bash
# Splits the backend's Go tests into the groups the `go-db` CI job runs in
# parallel, each against its own CockroachDB. Run it from backend/.
#
#   go-db-shard.sh <group>        print the group's packages, one per line
#   go-db-shard.sh --run <group>  print the group's `-run` regex (empty unless
#                                 the group is a part of the play package)
#   go-db-shard.sh --check        fail unless the groups together are exactly
#                                 `go list ./...` (nothing skipped, nothing
#                                 twice) and the play parts together are
#                                 exactly the play package's tests
#
# The heavy packages are named by hand, from the per-package times of the
# 10/10/2026 run (play 920 s, leaktest 630 s, rules/dungeon 598 s and rules
# 476 s, maps/images 471 s, campaigns 309 s, campaignpackage 253 s, identity
# 157 s, with the groups' packages sharing four vCPUs). The group "rest" is
# everything not named here, so a NEW package lands in "rest" on its own and
# is never skipped. If a group gets too slow, move a package up.
#
# The play package alone took 920 s, so it is split by test: play-1 to play-3
# each run the tests whose name hashes (cksum) to their part, with
# `-run '^(TestA|TestB)$'`. The hash is stable: adding a test never moves the
# others, and a NEW test lands in a part by itself. --check proves the parts
# are exactly the tests `go test -list` prints (Test, Fuzz and Example).
set -euo pipefail

module=github.com/PuraFome/meuRPG/backend
play_parts=3

case_packages() {
  case "$1" in
    play-[1-9]) echo "$module/internal/play" ;;
    maps) echo "$module/internal/maps" ;;
    characters) echo "$module/internal/characters $module/migrations $module/internal/progression" ;;
    # The CPU-bound pure packages, kept off the runners of the database ones.
    pure) echo "$module/internal/rules $module/internal/rules/dungeon $module/internal/rules/puzzle" ;;
    leak) echo "$module/internal/leaktest" ;;
    camp) echo "$module/internal/campaigns $module/internal/campaignpackage" ;;
    rest) echo "" ;;
    *) echo "unknown group: $1" >&2; return 1 ;;
  esac
}

# play-2 and play-3 name the same package as play-1: only play-1 counts here.
named_groups="play-1 maps characters pure leak camp"

all_packages() { go list ./... | sort; }

named_packages() { for g in $named_groups; do case_packages "$g" | tr ' ' '\n'; done | sort; }

# The play package's tests (Test*, Fuzz*, Example*), sorted, one per line.
# `go test -list` compiles the test binary and runs no test.
play_tests() {
  # --check lists once and passes the list on through PLAY_TESTS.
  if [ -n "${PLAY_TESTS:-}" ]; then echo "$PLAY_TESTS"; return; fi
  go test -list '.*' "$module/internal/play" | grep -E '^(Test|Fuzz|Example)[A-Za-z0-9_]*$' | sort
}

# Part N (1-based) of the tests on stdin: those whose cksum mod parts is N-1.
part_of() {
  local n=$1 name sum
  while IFS= read -r name; do
    sum=$(printf '%s' "$name" | cksum | cut -d' ' -f1)
    if [ $((sum % play_parts + 1)) -eq "$n" ]; then echo "$name"; fi
  done
}

run_regex() {
  local tests
  tests=$(play_tests | part_of "$1" | paste -sd'|' -)
  if [ -z "$tests" ]; then
    echo "::error::play part $1 has no tests" >&2
    return 1
  fi
  echo "^($tests)\$"
}

if [ "${1:-}" = "--run" ]; then
  case "${2:?usage: go-db-shard.sh --run <group>}" in
    play-[1-9]) run_regex "${2#play-}" ;;
    *) case_packages "$2" >/dev/null; echo "" ;;
  esac
  exit 0
fi

if [ "${1:-}" = "--check" ]; then
  all=$(all_packages)
  named=$(named_packages)
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
  # Every play part runs the play package, and nothing else.
  for i in $(seq 1 "$play_parts"); do
    if [ "$(case_packages "play-$i")" != "$(case_packages play-1)" ]; then
      echo "::error::play-$i must name only the play package" >&2
      exit 1
    fi
  done
  # The parts together are exactly the package's tests, each once.
  tests=$(play_tests)
  export PLAY_TESTS=$tests
  parts=$(for i in $(seq 1 "$play_parts"); do echo "$tests" | part_of "$i"; done | sort)
  if [ -z "$tests" ] || [ "$parts" != "$tests" ]; then
    echo "::error::the play parts are not exactly the play package's tests" >&2
    diff <(echo "$tests") <(echo "$parts") >&2 || true
    exit 1
  fi
  for i in $(seq 1 "$play_parts"); do run_regex "$i" >/dev/null; done
  echo "OK: $(echo "$all" | wc -l | tr -d ' ') packages, every one in exactly one group; $(echo "$tests" | wc -l | tr -d ' ') play tests, every one in exactly one of $play_parts parts."
  exit 0
fi

group=${1:?usage: go-db-shard.sh <group>|--run <group>|--check}
if [ "$group" = rest ]; then
  comm -23 <(all_packages) <(named_packages)
else
  case_packages "$group" | tr ' ' '\n'
fi
