#!/usr/bin/env bash
# The package backend/internal/play may not grow past the number of lines committed in
# .github/play-cap: logic that does not need the store, the transport or the Service goes to a
# subpackage of play (docs/architecture), so the package every session edits stops being the
# place where each feature lands.
#
#   play-cap.sh           check the count against the cap (CI runs this)
#   play-cap.sh --lower   write the cap down to the current count, when it is lower
#
# It counts the lines (wc -l) of the hand-written, non-test Go files directly in backend/internal/play
# (generated files, "// Code generated ... DO NOT EDIT.", and testdata/ are left out, as in
# go-complexity.sh; the subpackages of play are other packages and count for nothing here). It prints the files and the functions (lines that start with `func `) too,
# and caps neither.
#
# Lowering the cap is always fine: when the count is under it, the check passes and says so, and
# `--lower` (or an edit of the file) records it. Raising it is an edit of .github/play-cap that the
# reviewer sees, with one sentence in the PR body on why the lines cannot go to a subpackage.
# When two PRs of a train both edit the cap, the merge recounts (`play-cap.sh --lower` on the merged
# tree, or the count it prints as the new number): it never adds the two edits.
#
# Bash 3.2 compatible (macOS's /bin/bash): no associative arrays, no mapfile.
set -euo pipefail

mode=check
case "${1:-}" in
  "") ;;
  --lower) mode=lower ;;
  *)
    echo "usage: play-cap.sh [--lower]" >&2
    exit 2
    ;;
esac

repo=$(cd "$(dirname "$0")/../.." && pwd)
cap_file="$repo/.github/play-cap"
cd "$repo/backend"

if [ ! -f "$cap_file" ]; then
  echo "::error::$cap_file is missing: it holds the number of lines backend/internal/play may have" >&2
  exit 2
fi
cap=$(tr -d '[:space:]' < "$cap_file")
case "$cap" in
  '' | *[!0-9]*)
    echo "::error::.github/play-cap must hold one whole number, found: $cap" >&2
    exit 2
    ;;
esac

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# The hand-written, non-test Go files directly in the package, relative to backend/.
find internal/play -maxdepth 1 -name '*.go' ! -name '*_test.go' -print | LC_ALL=C sort > "$tmp/all"
if [ ! -s "$tmp/all" ]; then
  echo "::error::play-cap.sh found no Go file in backend/internal/play" >&2
  exit 2
fi
xargs grep -L -E '^// Code generated .* DO NOT EDIT\.$' < "$tmp/all" > "$tmp/files" || true
if [ ! -s "$tmp/files" ]; then
  echo "::error::play-cap.sh found no hand-written Go file in backend/internal/play" >&2
  exit 2
fi

files=$(wc -l < "$tmp/files" | tr -d '[:space:]')
count=$(xargs cat < "$tmp/files" | wc -l | tr -d '[:space:]')
funcs=$(xargs grep -h -c '^func ' < "$tmp/files" | awk '{ n += $1 } END { print n + 0 }')

echo "backend/internal/play: $count lines in $files files, $funcs functions (cap $cap lines)"

if [ "$count" -gt "$cap" ]; then
  echo "::error file=.github/play-cap::backend/internal/play has $count lines, over the cap of $cap. Put the logic that needs no store, transport or Service in a subpackage of play in this PR; if the lines have to stay, raise .github/play-cap in this PR and say why in the description."
  echo "Largest files (yours are in your diff):"
  xargs wc -l < "$tmp/files" | grep -v ' total$' | LC_ALL=C sort -rn | head -10
  exit 1
fi

if [ "$count" -lt "$cap" ]; then
  if [ "$mode" = lower ]; then
    echo "$count" > "$cap_file"
    echo "Lowered .github/play-cap from $cap to $count."
  else
    echo "::notice file=.github/play-cap::The count ($count) is under the cap ($cap): lower .github/play-cap to $count (play-cap.sh --lower)."
  fi
fi
