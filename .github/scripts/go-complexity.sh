#!/usr/bin/env bash
# No hand-written Go function of backend/ may pass the complexity limits.
# Measures every hand-written, non-test Go file and fails on any function
# over a limit, naming each one with its file and line.
#
#   go-complexity.sh    check every function (CI runs this)
#
# The limits are those of backend/.golangci.yml:
#   cyclomatic       gocyclo v0.6.0: complexity above 15 fails
#   cognitive        gocognit v1.2.0: complexity above 15 fails
#   maintainability  golangci-lint v2.14.0's maintidx: index below 20 fails
#
# Why it exists: golangci-lint's ratchet judges only the lines a change
# touches, and these linters report on the `func` line, so a change to the
# body of an old function never trips them. This script judges every body.
# gocyclo and gocognit run outside golangci-lint, so a //nolint does not hide
# a function from them. Generated files (Go's "// Code generated ... DO NOT
# EDIT." line: backend/gen and the sqlc packages) are left out, as
# golangci-lint leaves them out.
#
# Bash 3.2 compatible (macOS's /bin/bash): no associative arrays, no mapfile.
set -euo pipefail

# The limits: keep them equal to gocyclo.min-complexity and
# gocognit.min-complexity in backend/.golangci.yml. maintidx's (under: 20)
# is read from that file by golangci-lint itself.
cyclo_over=15
cognit_over=15

# Every tool pinned by its module version. golangci-lint v2.14.0 bundles
# gocognit v1.2.1, whose scoring code (gocognit.go) is the same as v1.2.0's. Keep golangci_version equal to the
# Makefile's and the golangci-lint-action's in .github/workflows/backend.yml.
gocyclo_pkg=github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
gocognit_pkg=github.com/uudashr/gocognit/cmd/gocognit@v1.2.0
golangci_version=2.14.0

if [ $# -gt 0 ]; then
  echo "usage: go-complexity.sh" >&2
  exit 2
fi

repo=$(cd "$(dirname "$0")/../.." && pwd)
cd "$repo/backend"

command -v jq >/dev/null || { echo "::error::go-complexity.sh needs jq" >&2; exit 2; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# The hand-written, non-test Go files, relative to backend/. grep -L lists
# the files WITHOUT the generated line; its exit status only says whether it
# listed any (and differs between grep versions), so the list is what counts.
find . -name testdata -prune -o -name '*.go' ! -name '*_test.go' -print | sed 's|^\./||' | LC_ALL=C sort > "$tmp/all"
xargs grep -L -E '^// Code generated .* DO NOT EDIT\.$' < "$tmp/all" > "$tmp/files" || true
if [ ! -s "$tmp/files" ]; then
  echo "::error::go-complexity.sh found no hand-written Go file under backend/" >&2
  exit 2
fi
files=()
while IFS= read -r f; do files+=("$f"); done < "$tmp/files"

# Without -over, gocyclo and gocognit list every function and exit 0, so a
# non-zero exit is a real failure; the limits are applied below. Each output
# line is "<score> <package> <function> <file:line:column>".
go run "$gocyclo_pkg" "${files[@]}" > "$tmp/gocyclo"
go run "$gocognit_pkg" "${files[@]}" > "$tmp/gocognit"

# maintidx only runs inside golangci-lint: its own command is built on a 2021
# golang.org/x/tools that cannot load today's packages. CI's lint step puts
# the pinned binary on PATH; anywhere else without it, `go run` builds it (a
# minute or two the first time, as for `make lint`).
if command -v golangci-lint >/dev/null && golangci-lint version 2>/dev/null | grep -q "version $golangci_version "; then
  golangci=(golangci-lint)
else
  golangci=(go run "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$golangci_version")
fi
"${golangci[@]}" run --enable-only=maintidx --tests=false --issues-exit-code=0 \
  --max-issues-per-linter=0 --max-same-issues=0 --show-stats=false \
  --output.json.path="$tmp/maintidx.json" ./...

# One line per function over a limit: "<metric>\t<score>\t<function>\t<file:line>".
# maintidx prints the bare name; the receiver comes from the `func` line it
# quotes, written the way gocyclo writes it ("(*Service).Name").
{
  awk -v metric=cyclomatic -v over="$cyclo_over" 'BEGIN { OFS = "\t" }
    $1 > over { f = $3; for (i = 4; i < NF; i++) f = f " " $i; pos = $NF; sub(/:[0-9]+$/, "", pos); print metric, $1, f, pos }' "$tmp/gocyclo"
  awk -v metric=cognitive -v over="$cognit_over" 'BEGIN { OFS = "\t" }
    $1 > over { f = $3; for (i = 4; i < NF; i++) f = f " " $i; pos = $NF; sub(/:[0-9]+$/, "", pos); print metric, $1, f, pos }' "$tmp/gocognit"
  jq -r '.Issues[]? | select(.FromLinter == "maintidx")
    | ((.Text | capture("Function name: (?<name>[^,]+),.* Maintainability Index: (?<mi>-?[0-9]+)"))
       // error("go-complexity.sh cannot read this maintidx message: \(.Text)")) as $m
    | [(.SourceLines[0] // "") | match("^func \\((?:\\w+ )?(\\*?\\w+)") | .captures[0].string] as $recv
    | ["maintainability", $m.mi, ((if ($recv | length) > 0 then "(" + $recv[0] + ")." else "" end) + $m.name), "\(.Pos.Filename):\(.Pos.Line)"]
    | @tsv' "$tmp/maintidx.json"
} > "$tmp/raw"

# The functions over a limit, by metric and file, each with a CI annotation
# on its line.
LC_ALL=C sort -t "$(printf '\t')" -k1,1 -k4,4 "$tmp/raw" > "$tmp/over"
echo "Functions over the complexity limits (cyclomatic > $cyclo_over, cognitive > $cognit_over, maintainability < 20):"
for m in cyclomatic cognitive maintainability; do
  echo "  $m: $(grep -c "^$m	" "$tmp/over" || true) functions over the limit"
done
if [ ! -s "$tmp/over" ]; then
  echo "OK: every function is within cyclomatic $cyclo_over, cognitive $cognit_over and maintainability 20."
  exit 0
fi
echo
echo "Split or simplify these functions (see docs/ci.md, \"Complexity limits\"):"
while IFS=$'\t' read -r metric score fn at; do
  dir=${at%/*}
  [ "$dir" = "$at" ] && dir=.
  echo "  $metric $dir $fn $score ($at)"
  echo "::error file=backend/${at%:*},line=${at##*:}::$metric $score: $fn is over the limit (cyclomatic $cyclo_over, cognitive $cognit_over, maintainability 20)"
done < "$tmp/over"
exit 1
