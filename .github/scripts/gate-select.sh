#!/usr/bin/env bash
# Decides whether a part of the gate runs for a pull request that is not a train, from the files it changes
# (docs/ci.md, "What runs when").
#
#   gate-select.sh <output> <ERE> [--ignore-md]
#
# Appends `<output>=true|false` to $GITHUB_OUTPUT (stdout without it): true when a changed file matches <ERE>
# (--ignore-md: not counting *.md files). Needs the whole history (the diff against the base branch).
#
# Only the workflows' `DIFF` expression decides who gets here: a push to main, the nightly, a manual run and a train
# never do, so this script does not know them.
#
# Environment: BASE_REF (github.base_ref).
set -euo pipefail

output="${1:?the name of the output}"
ere="${2:?the regex of the files that select it}"
ignore_md=false
[ "${3:-}" = --ignore-md ] && ignore_md=true
out="${GITHUB_OUTPUT:-/dev/stdout}"

changed=$(git -c core.quotePath=false diff --name-only --no-renames "origin/${BASE_REF:?BASE_REF}...HEAD")
if [ "$ignore_md" = true ]; then changed=$(grep -v '\.md$' <<<"$changed" || true); fi
if grep -Eq "$ere" <<<"$changed"; then selected=true; else selected=false; fi
echo "${output}=${selected}" >>"$out"
echo "${output}=${selected}" >&2
