#!/usr/bin/env bash
# Runs the tests flake-targets.mjs found, REPEATS times each, inside a time budget (docs/ci.md, "Repeat of the changed tests").
#
#   repeat-changed.sh playwright <targets.json>   from the e2e/ directory, in both colour schemes
#   repeat-changed.sh go <targets.json>           from the backend/ directory, with -race
#
# Each target (one Playwright test, one Go package) is a chunk. A chunk stops at its first failure (--max-failures=1,
# -failfast), so a chunk that is cut off by the budget had no failure up to then: running out of time passes, and the
# job summary says what did not run. A failure fails the script, after the other chunks have run. The summary goes to
# $GITHUB_STEP_SUMMARY (stdout without it).
set -uo pipefail

kind="${1:?playwright or go}"
targets="${2:?the targets.json from flake-targets.mjs}"
repeats="${REPEATS:-10}"
budget="${BUDGET_SECONDS:-900}"
summary="${GITHUB_STEP_SUMMARY:-/dev/stdout}"
deadline=$(( $(date +%s) + budget ))

# One chunk per line: a label, the scheme (Playwright) or "-", and the regex.
case "$kind" in
  playwright)
    mapfile -t chunks < <(jq -r '.playwright[] | .grep as $g | .title as $t | ("light", "dark") | [($t + " (" + . + ")"), ., $g] | @tsv' "$targets")
    ;;
  go)
    mapfile -t chunks < <(jq -r '.go | group_by(.pkg)[] | [(.[0].pkg + ": " + (map(.test) | join(", "))), .[0].pkg, ("^(" + (map(.test) | join("|")) + ")$")] | @tsv' "$targets")
    ;;
  *)
    echo "unknown kind: $kind" >&2
    exit 2
    ;;
esac

passed=(); notrun=(); cut=(); failed=()
left=${#chunks[@]}
for chunk in "${chunks[@]}"; do
  IFS=$'\t' read -r label field regex <<<"$chunk"
  now=$(date +%s)
  remaining=$(( deadline - now ))
  if [ "$remaining" -lt "${MIN_SECONDS:-20}" ]; then
    notrun+=("$label")
    left=$(( left - 1 ))
    continue
  fi
  # A fair share: twice the average of what is left, so one slow chunk cannot eat the others' time.
  limit=$(( remaining * 2 / (left > 0 ? left : 1) ))
  [ "$limit" -gt "$remaining" ] && limit=$remaining
  left=$(( left - 1 ))

  echo "::group::${label}"
  if [ "$kind" = playwright ]; then
    E2E_COLOR_SCHEME="$field" timeout --kill-after=15 "$limit" \
      npx playwright test --grep "$regex" --repeat-each="$repeats" --max-failures=1 --reporter=list
  else
    timeout --kill-after=15 "$limit" \
      go test -race -count="$repeats" -failfast -timeout 30m -run "$regex" "$field"
  fi
  code=$?
  echo "::endgroup::"
  case "$code" in
    0) passed+=("$label") ;;
    124 | 137) cut+=("$label") ;;
    *) failed+=("$label") ;;
  esac
done

{
  echo "## Repeat of the changed tests (${kind}, ${repeats} runs each, budget ${budget} s)"
  echo
  echo "- Passed: ${#passed[@]}"
  echo "- Failed: ${#failed[@]}"
  echo "- Cut off by the time budget, no failure before it: ${#cut[@]}"
  echo "- Not run, the budget was spent: ${#notrun[@]}"
  for l in "${failed[@]}"; do echo "  - FAILED: ${l}"; done
  for l in "${cut[@]}"; do echo "  - cut off: ${l}"; done
  for l in "${notrun[@]}"; do echo "  - not run: ${l}"; done
} >>"$summary"

if [ "${#failed[@]}" -gt 0 ]; then
  echo "::error::${#failed[@]} changed test(s) failed when repeated (docs/ci.md, \"Flaky tests\"): a flaky test is a bug in the test or the app." >&2
  exit 1
fi
exit 0
