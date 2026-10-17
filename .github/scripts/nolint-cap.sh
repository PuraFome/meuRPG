#!/usr/bin/env bash
# The number of //nolint directives in backend/ may not grow past the number committed in
# .github/nolint-cap: a suppression is a debt, and the count only goes down.
#
#   nolint-cap.sh           check the count against the cap (CI runs this)
#   nolint-cap.sh --lower   write the cap down to the current count, when it is lower
#
# It counts every //nolint directive of the hand-written Go files of backend/, tests included
# (generated files, "// Code generated ... DO NOT EDIT.", and testdata/ are left out, as in
# go-complexity.sh). A directive is the text //nolint followed by a colon, a space or the end of the
# line; nolintlint (backend/.golangci.yml) already makes every one name its linter and say why.
#
# Lowering the cap is always fine: when the count is under it, the check passes and says so, and
# `--lower` (or an edit of the file) records it. Raising it is an edit of .github/nolint-cap that
# the reviewer sees: do it only for a suppression that cannot be avoided, and say why in the PR.
#
# Bash 3.2 compatible (macOS's /bin/bash): no associative arrays, no mapfile.
set -euo pipefail

mode=check
case "${1:-}" in
  "") ;;
  --lower) mode=lower ;;
  *)
    echo "usage: nolint-cap.sh [--lower]" >&2
    exit 2
    ;;
esac

repo=$(cd "$(dirname "$0")/../.." && pwd)
cap_file="$repo/.github/nolint-cap"
cd "$repo/backend"

if [ ! -f "$cap_file" ]; then
  echo "::error::$cap_file is missing: it holds the number of //nolint directives the backend may have" >&2
  exit 2
fi
cap=$(tr -d '[:space:]' < "$cap_file")
case "$cap" in
  '' | *[!0-9]*)
    echo "::error::.github/nolint-cap must hold one whole number, found: $cap" >&2
    exit 2
    ;;
esac

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# The hand-written Go files, relative to backend/ (the list go-complexity.sh uses, with the tests).
find . -name testdata -prune -o -name '*.go' -print | sed 's|^\./||' | LC_ALL=C sort > "$tmp/all"
xargs grep -L -E '^// Code generated .* DO NOT EDIT\.$' < "$tmp/all" > "$tmp/files" || true
if [ ! -s "$tmp/files" ]; then
  echo "::error::nolint-cap.sh found no hand-written Go file under backend/" >&2
  exit 2
fi

# One line per directive: "<file>:<line>:<text>".
xargs grep -n -o -E '//nolint([:[:space:]]|$)' < "$tmp/files" > "$tmp/directives" || true
count=$(wc -l < "$tmp/directives" | tr -d '[:space:]')

echo "//nolint directives in backend/: $count (cap $cap)"

if [ "$count" -gt "$cap" ]; then
  echo "::error file=.github/nolint-cap::backend/ has $count //nolint directives, over the cap of $cap. Fix the finding instead of suppressing it; if a suppression cannot be avoided, raise .github/nolint-cap in this PR and say why."
  echo "Directives by file (the ones you added are in your diff):"
  cut -d: -f1 "$tmp/directives" | LC_ALL=C sort | uniq -c | LC_ALL=C sort -rn | head -20
  exit 1
fi

if [ "$count" -lt "$cap" ]; then
  if [ "$mode" = lower ]; then
    echo "$count" > "$cap_file"
    echo "Lowered .github/nolint-cap from $cap to $count."
  else
    echo "::notice file=.github/nolint-cap::The count ($count) is under the cap ($cap): lower .github/nolint-cap to $count (nolint-cap.sh --lower)."
  fi
fi
