#!/usr/bin/env bash
# Decides whether a part of the gate runs for this event (docs/ci.md, "What runs when").
#
#   gate-select.sh <output> <ERE> [--ignore-md]
#
# Appends `<output>=true|false` to $GITHUB_OUTPUT (stdout without it). Needs the whole history
# on a pull request (the diff against the base branch).
#
#   - not a pull request (a push to main, the nightly, a manual run): true.
#   - a pull request from a refactor/train-* branch: true, whatever it changes. A train runs the full gate, so a
#     docs-only train is judged like any other.
#   - any other pull request: true when a changed file matches <ERE> (--ignore-md: not counting *.md files).
#
# Environment: EVENT (github.event_name), BASE_REF (github.base_ref), HEAD_REF (github.head_ref).
set -euo pipefail

output="${1:?the name of the output}"
ere="${2:?the regex of the files that select it}"
ignore_md=false
[ "${3:-}" = --ignore-md ] && ignore_md=true
out="${GITHUB_OUTPUT:-/dev/stdout}"

selected=true
if [ "${EVENT:-}" = pull_request ]; then
  if [[ "${HEAD_REF:-}" == refactor/train-* ]]; then
    selected=true
  else
    changed=$(git -c core.quotePath=false diff --name-only --no-renames "origin/${BASE_REF:?BASE_REF}...HEAD")
    if [ "$ignore_md" = true ]; then changed=$(grep -v '\.md$' <<<"$changed" || true); fi
    if grep -Eq "$ere" <<<"$changed"; then selected=true; else selected=false; fi
  fi
fi
echo "${output}=${selected}" >>"$out"
echo "${output}=${selected}" >&2
