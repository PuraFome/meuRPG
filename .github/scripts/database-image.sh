#!/usr/bin/env bash
# The test database images of CI (docs/ci.md, "Which database the tests run on"): where they are pinned, and how the
# `go-db` jobs get them without each pulling from a registry.
#
#   database-image.sh ref  postgres|cockroach      prints the image the workflow pins, by digest
#   database-image.sh pull postgres|cockroach OUT  pulls it once, tags it meurpg-test-db:<engine> and saves it to OUT (.tar.gz)
#   database-image.sh load FILE                    loads a saved image, prints the local tag
#
# The pins are POSTGRES_IMAGE and COCKROACH_IMAGE in the `env:` of backend.yml, the one place of each. A refusal of the
# registry (a data limit, a missing digest) fails `pull` with the registry's own message, once: there is no retry.
set -euo pipefail

workflow="$(dirname "$0")/../workflows/backend.yml"

pinned() {
  local name
  case "$1" in
    postgres) name=POSTGRES_IMAGE ;;
    cockroach) name=COCKROACH_IMAGE ;;
    *) echo "::error::unknown database '$1': postgres or cockroach" >&2; exit 2 ;;
  esac
  local image
  image=$(grep -m1 -E "^[[:space:]]*$name:" "$workflow" | sed -E "s/^[[:space:]]*$name:[[:space:]]*//") || true
  if [ -z "$image" ]; then
    echo "::error::$name not found in $workflow; this script no longer matches it." >&2
    exit 1
  fi
  case "$image" in
    *@sha256:*) ;;
    *) echo "::error::$name in $workflow is not pinned by digest: $image" >&2; exit 1 ;;
  esac
  echo "$image"
}

case "${1:-}" in
  ref)
    pinned "${2:?postgres or cockroach}"
    ;;
  pull)
    engine=${2:?postgres or cockroach}
    out=${3:?the file to save to}
    image=$(pinned "$engine")
    local_tag="meurpg-test-db:$engine"
    echo "Pulling $image (the only pull of this run for $engine)"
    if ! docker pull --quiet "$image" >/dev/null; then
      echo "::error::The registry refused the pull of $image (a data limit or a missing digest). Nothing was retried; run the job again when the registry allows it." >&2
      exit 1
    fi
    docker tag "$image" "$local_tag"
    docker save "$local_tag" | gzip -1 >"$out"
    ls -l "$out"
    ;;
  load)
    file=${2:?the saved image}
    loaded=$(docker load --input "$file")
    echo "$loaded"
    echo "${loaded##*: }"
    ;;
  *)
    echo "usage: database-image.sh ref|pull|load ..." >&2
    exit 2
    ;;
esac
