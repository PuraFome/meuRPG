#!/usr/bin/env bash
# Opens, or adds to, the `ci-red` issue of a workflow whose run outside a pull request failed (docs/ci.md, "What runs
# when"). One issue per title: another open ci-red issue is a different red.
#
#   ci-red-report.sh [<body file>]
#
# Without a body file the body says that the workflow failed on the commit, with the link to the run.
# Environment: GH_TOKEN, GH_REPO, WORKFLOW (github.workflow), SHA (github.sha), RUN_URL; TITLE (default
# "CI vermelho: $WORKFLOW na main").
set -euo pipefail

: "${WORKFLOW:?WORKFLOW}" "${SHA:?SHA}" "${RUN_URL:?RUN_URL}"
title="${TITLE:-CI vermelho: $WORKFLOW na main}"
body_file="${1:-}"
if [ -z "$body_file" ]; then
  body_file=$(mktemp)
  trap 'rm -f "$body_file"' EXIT
  printf 'O workflow %s falhou na main.\n\nCommit: %s\nExecução: %s\n' "$WORKFLOW" "$SHA" "$RUN_URL" >"$body_file"
fi

gh label create ci-red --force --color B60205 --description "CI vermelho na main" || true
# The exact title (read by jq from the environment, so a quote or a backslash in it cannot break the filter), not a
# prefix or the first ci-red issue: [.[] | select(...)][0] is the first match, or none.
number=$(TITLE="$title" gh issue list --label ci-red --state open --limit 200 --json number,title \
  --jq '[.[] | select(.title == env.TITLE)][0].number // empty')
if [ -n "$number" ]; then
  gh issue comment "$number" --body-file "$body_file"
else
  gh issue create --label ci-red --title "$title" --body-file "$body_file"
fi
