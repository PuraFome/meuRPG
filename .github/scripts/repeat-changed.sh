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

# One chunk per line, as compact JSON (a regex keeps its backslashes that way): {label, field, regex, file}.
# For Playwright `field` is the colour scheme and `file` the spec, relative to e2e/; for Go `field` is the package.
case "$kind" in
  playwright)
    mapfile -t chunks < <(jq -c '.playwright[] | . as $t | ("light", "dark") | {label: ($t.title + " (" + . + ")"), field: ., regex: $t.grep, file: ($t.file | sub("^e2e/"; ""))}' "$targets")
    ;;
  go)
    mapfile -t chunks < <(jq -c '.go | group_by(.pkg)[] | {label: (.[0].pkg + ": " + (map(.test) | join(", "))), field: .[0].pkg, regex: ("^(" + (map(.test) | join("|")) + ")$"), file: ""}' "$targets")
    ;;
  *)
    echo "unknown kind: $kind" >&2
    exit 2
    ;;
esac

passed=(); notrun=(); cut=(); failed=()
left=${#chunks[@]}
first=yes
for chunk in "${chunks[@]}"; do
  label=$(jq -r .label <<<"$chunk")
  field=$(jq -r .field <<<"$chunk")
  regex=$(jq -r .regex <<<"$chunk")
  file=$(jq -r .file <<<"$chunk")
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

  # /auth/login lets one address 40 requests at once (20 sign-ins) and refills one every 3 s, and every chunk signs in
  # again in its setup: RESET_STACK_CMD (the workflow restarts the api) gives each chunk after the first a fresh bucket.
  if [ -n "${RESET_STACK_CMD:-}" ] && [ "$first" = no ]; then
    echo "::group::reset the stack before ${label}"
    bash -c "$RESET_STACK_CMD" || echo "::warning::the reset failed; the login rate limit may be spent"
    echo "::endgroup::"
  fi
  first=no

  echo "::group::${label}"
  if [ "$kind" = playwright ]; then
    E2E_COLOR_SCHEME="$field" timeout --kill-after=15 "$limit" \
      npx playwright test "$file" --grep "$regex" --repeat-each="$repeats" --max-failures=1
  else
    timeout --kill-after=15 "$limit" \
      go test -race -count="$repeats" -failfast -timeout 30m -run "$regex" "$field"
  fi
  code=$?
  echo "::endgroup::"
  case "$code" in
    0) passed+=("$label") ;;
    124 | 137) cut+=("$label") ;;
    *)
      failed+=("$label")
      # The next chunk's run empties test-results/ and playwright-report/: keep this one's traces and report.
      if [ "$kind" = playwright ]; then
        keep="kept/${#failed[@]}"
        mkdir -p "$keep" && cp -r test-results playwright-report "$keep"/ 2>/dev/null || true
      fi
      ;;
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
