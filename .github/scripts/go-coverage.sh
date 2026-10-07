#!/usr/bin/env bash
# Merges the coverage profiles the go-db shards wrote, prints the total and a
# per-package table (Markdown, for $GITHUB_STEP_SUMMARY), and enforces a floor
# on the pure rules packages. Run it from backend/.
#
#   go-coverage.sh <profile>...
#
# Each shard runs its packages with -coverpkg=./..., so every profile lists
# every block of the module; a block counts as covered if ANY shard ran it
# (counts are added, and only "> 0" matters). Nothing is uploaded anywhere.
#
# The floor: the packages in FLOOR_PACKAGES (pure code, no database, at 92 to
# 99 % on 07/10/2026) must stay at or above FLOOR_PERCENT. Everything else is
# reported without a floor. formula (81 %) and srd51 (generated data, no
# tests) are left out on purpose; add formula when its tests catch up.
set -euo pipefail

module=github.com/PuraFome/meuRPG/backend
FLOOR_PERCENT=${FLOOR_PERCENT:-90}
FLOOR_PACKAGES=${FLOOR_PACKAGES:-"internal/rules internal/rules/combat internal/rules/grid internal/rules/vision internal/rules/dungeon internal/rules/encounter internal/rules/puzzle"}

if [ "$#" -eq 0 ]; then
  echo "usage: go-coverage.sh <profile>..." >&2
  exit 2
fi

merged=$(mktemp)
trap 'rm -f "$merged"' EXIT

# Merge: "file:range numStatements count" lines, summed per block.
{
  echo "mode: atomic"
  awk 'FNR == 1 && /^mode:/ { next }
       { key = $1 " " $2; count[key] += $3 }
       END { for (k in count) print k, count[k] }' "$@" | sort
} > "$merged"

total=$(go tool cover -func="$merged" | awk '/^total:/ { print $3 }')

# Per package: statements covered / statements total.
table=$(awk -v module="$module" '
  /^mode:/ { next }
  {
    split($1, a, ":")
    file = a[1]
    sub("^" module "/", "", file)
    n = split(file, parts, "/")
    pkg = parts[1]
    for (i = 2; i < n; i++) pkg = pkg "/" parts[i]
    stmts[pkg] += $2
    if ($3 > 0) hit[pkg] += $2
  }
  END {
    for (p in stmts) printf "%s %d %d %.1f\n", p, hit[p], stmts[p], 100 * hit[p] / stmts[p]
  }' "$merged" | sort)

summary() {
  echo "### Go coverage (all go-db shards merged)"
  echo
  echo "Total: **$total** of statements, with the integration tests (\`-coverpkg=./...\`)."
  echo
  echo "| Package | Covered | Statements | % | Floor |"
  echo "| --- | ---: | ---: | ---: | --- |"
  while read -r pkg hit stmts pct; do
    floor=""
    for f in $FLOOR_PACKAGES; do
      if [ "$f" = "$pkg" ]; then floor=">= ${FLOOR_PERCENT}%"; fi
    done
    echo "| \`$pkg\` | $hit | $stmts | $pct | $floor |"
  done <<< "$table"
}

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  summary >> "$GITHUB_STEP_SUMMARY"
else
  summary
fi

fail=0
for f in $FLOOR_PACKAGES; do
  pct=$(awk -v p="$f" '$1 == p { print $4 }' <<< "$table")
  if [ -z "$pct" ]; then
    echo "::error::coverage floor: package $f has no coverage data (renamed? update FLOOR_PACKAGES in go-coverage.sh)" >&2
    fail=1
  elif awk -v a="$pct" -v b="$FLOOR_PERCENT" 'BEGIN { exit !(a < b) }'; then
    echo "::error::coverage of $f is $pct %, below the floor of $FLOOR_PERCENT %" >&2
    fail=1
  fi
done
exit $fail
