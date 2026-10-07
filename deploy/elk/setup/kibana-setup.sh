#!/usr/bin/env bash
# One-shot (service `kibana-setup`): the data view logs-meurpg-* and the saved searches.
# Idempotent: the import overwrites objects with the same ids (see kibana-objects.ndjson).
set -euo pipefail

KIBANA=http://kibana:5601
FILE=/setup/kibana-objects.ndjson

# kb METHOD PATH [curl args...]: an authenticated call as the superuser (password from the environment, never printed).
kb() {
  local method=$1 path=$2
  shift 2
  printf 'user = "elastic:%s"\n' "$ELASTIC_PASSWORD" |
    curl -sS --fail-with-body -K - -X "$method" -H 'kbn-xsrf: true' "$KIBANA$path" "$@"
}

echo "kibana-setup: waiting for Kibana"
for i in $(seq 1 60); do
  if kb GET /api/status >/dev/null 2>&1; then break; fi
  [ "$i" -lt 60 ] || { echo "kibana-setup: Kibana did not answer in 5 minutes" >&2; exit 1; }
  sleep 5
done

echo "kibana-setup: importing the data view and the saved searches"
out=$(kb POST '/api/saved_objects/_import?overwrite=true' -F file=@"$FILE")
case "$out" in
  *'"success":true'*) ;;
  *) echo "kibana-setup: import failed: $out" >&2; exit 1 ;;
esac

echo "kibana-setup: default data view"
kb POST /api/data_views/default -H 'Content-Type: application/json' \
  -d '{"data_view_id":"meurpg-logs","force":true}' >/dev/null

echo "kibana-setup: done"
