#!/usr/bin/env bash
# Query MeuRPG's API logs in Elasticsearch from the terminal (bash + curl + jq). One line per log entry:
#   time  LEVEL  req=<request id>  message  <procedure | event | path>  code=  status=  <ms>  error=
#
# Usage: deploy/elk/logs.sh <command> [args]
#   errors   [since]            level error, or a 5xx response              (default since: 1h)
#   request  <request id>       everything one request logged, oldest first (all retained logs)
#   campaign <campaign id> [since]
#   user     <user id> [since]
#   search   '<query>' [since]  Lucene query_string, e.g. 'meurpg.procedure:*CreateCampaign AND NOT meurpg.code:ok'
#                               (and / or / not in lower case work too)
#   tail     [query]            the last 10 lines, then polls every 2 s and prints only new ones
#   health                      cluster status and how many log lines are stored
#   sample                      appends 4 sample lines (now) to ~/.meurpg/run-sample/api.log, for Filebeat to ship
#   simulate                    runs the same 4 lines through the ingest pipeline and prints the ECS documents
# `since` is a number and a unit: 30s, 15m, 2h, 1d, 1w, or `all`.
#
# Reads deploy/elk/.env (READER_PASSWORD: the read-only user meurpg_reader; without it, ELASTIC_PASSWORD) and the CA
# deploy/elk/certs/ca.crt, which `make elk-up` creates. Overrides: ES_URL, ES_CA, LOGS_SIZE (default 200),
# LOGS_JSON=1 (raw _source JSON instead of one line per entry), MEURPG_HOME.
# Fields: contract in the backend log contract, backend/internal/platform/logging; ECS mapping in deploy/elk/setup/pipeline.json.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
INDEX='logs-meurpg-*'
SIZE=${LOGS_SIZE:-200}

# envval NAME: the value from the environment or, failing that, from deploy/elk/.env (never sourced).
envval() {
  local v=${!1:-}
  if [ -z "$v" ] && [ -f "$HERE/.env" ]; then
    v=$(grep -E "^$1=" "$HERE/.env" | tail -n 1 | cut -d= -f2- || true)
  fi
  printf '%s' "$v"
}

ES_URL=${ES_URL:-https://localhost:$(envval ES_PORT | grep . || echo 9200)}
CA=${ES_CA:-$HERE/certs/ca.crt}
ELASTIC_PASSWORD=$(envval ELASTIC_PASSWORD)
READER_PASSWORD=$(envval READER_PASSWORD)
if [ -n "$READER_PASSWORD" ]; then
  READ_USER=meurpg_reader
  READ_PASS=$READER_PASSWORD
else
  READ_USER=elastic
  READ_PASS=$ELASTIC_PASSWORD
fi

die() { echo "logs.sh: $*" >&2; exit 1; }
command -v jq >/dev/null || die "jq is required"
command -v curl >/dev/null || die "curl is required"

# call USER PASS METHOD PATH [BODY]: the password goes through a curl config on stdin, never the command line.
call() {
  local user=$1 pass=$2 method=$3 path=$4 body=${5:-}
  [ -n "$pass" ] || die "no password: run make elk-up (it creates deploy/elk/.env)"
  [ -f "$CA" ] || die "no CA at $CA: run make elk-up and wait for the certs service"
  local args=(-sS --cacert "$CA" -K - -X "$method" -H 'Content-Type: application/json' "$ES_URL$path")
  [ -z "$body" ] || args+=(--data-binary "$body")
  local out
  out=$(printf 'user = "%s:%s"\n' "$user" "$pass" | curl "${args[@]}") ||
    die "cannot reach Elasticsearch at $ES_URL (is the stack up? make elk-up)"
  if jq -e '.error' >/dev/null 2>&1 <<<"$out"; then
    die "Elasticsearch: $(jq -r '.error | (.reason // .type // .)' <<<"$out" | head -n 1)"
  fi
  printf '%s' "$out"
}
read_api() { call "$READ_USER" "$READ_PASS" "$@"; }
admin_api() { call elastic "$ELASTIC_PASSWORD" "$@"; }

# The one-line format. Input: an array of documents (hits or _source objects).
FORMAT='
def lv: (((.log.level // "-") | ascii_upcase) + "       ")[0:7];
def ms: ((.event.duration / 100000 | round) / 10 | tostring) + "ms";
.[] | (._source // .) | [
  ((.["@timestamp"] // "-")[0:23]),
  lv,
  ("req=" + (.http.request.id // "-")),
  (.message // "-"),
  (.meurpg.procedure // .meurpg.event // .url.path // "-"),
  (if .meurpg.code then "code=" + .meurpg.code else empty end),
  (if .http.response.status_code then "status=" + (.http.response.status_code | tostring) else empty end),
  (if .event.duration then ms else empty end),
  (if .error.message then "error=" + .error.message else empty end)
] | join("  ") | gsub("[\r\n]+"; " ")'

print_hits() { # reads an array of hits on stdin
  if [ -n "${LOGS_JSON:-}" ]; then jq -c '.[] | (._source // .)'; else jq -r "$FORMAT"; fi
}

check_since() { [ "$1" = all ] || [[ "$1" =~ ^[0-9]+[smhdw]$ ]] || die "bad since '$1' (use 30s, 15m, 2h, 1d, 1w or all)"; }
check_id() { [[ "$1" =~ ^[A-Za-z0-9-]+$ ]] || die "bad id '$1'"; }

# body FILTER_JSON SINCE ORDER SIZE: a search body with the filter, the time range and the order.
body() {
  jq -nc --argjson f "$1" --arg since "$2" --arg order "$3" --argjson size "$4" '{
    size: $size,
    sort: [{"@timestamp": {order: $order}}],
    query: {bool: {filter: ([$f] + (if $since == "all" then [] else [{range: {"@timestamp": {gte: ("now-" + $since)}}}] end))}}
  }'
}

qs() { # a query_string filter; lower-case and/or/not become operators
  local q=$1
  if command -v perl >/dev/null; then q=$(perl -pe 's/(?<![\w.:"])(and|or|not)(?![\w.:"])/\U$1/g' <<<"$q"); fi
  jq -nc --arg q "$q" '{query_string: {query: $q, default_operator: "AND", lenient: true}}'
}
term() { jq -nc --arg f "$1" --arg v "$2" '{term: {($f): $v}}'; }

run() { # FILTER SINCE: newest SIZE entries, printed oldest first
  local resp
  resp=$(read_api POST "/$INDEX/_search" "$(body "$1" "$2" desc "$SIZE")")
  jq -c '.hits.hits | reverse' <<<"$resp" | print_hits
}

cmd_tail() {
  local query=${1:-*} resp state cursor
  local f
  f=$(qs "$query")
  # State: the highest event.ingested seen (epoch ms) and the entries seen in the last 5 s, so a
  # line that becomes visible a moment after a later one is still printed, and none twice.
  # shellcheck disable=SC2016 # jq program: the $ are jq variables
  update='([.hits.hits[] | {id: ._id, t: .sort[0]}] + $old) as $all
    | ([$all[].t] + [$cur] | max) as $m
    | {cursor: $m, seen: ([$all[] | select(.t >= $m - 5000)] | unique_by(.id))}'
  resp=$(read_api POST "/$INDEX/_search" "$(jq -nc --argjson f "$f" --argjson size 10 \
    '{size: $size, sort: [{"event.ingested": "desc"}], query: {bool: {filter: [$f]}}}')")
  jq -c '.hits.hits | reverse' <<<"$resp" | print_hits
  state=$(jq -c --argjson old '[]' --argjson cur "$(( $(date +%s) * 1000 ))" "$update" <<<"$resp")
  echo "-- following (Ctrl-C to stop) --" >&2
  while sleep 2; do
    cursor=$(jq '.cursor' <<<"$state")
    resp=$(read_api POST "/$INDEX/_search" "$(jq -nc --argjson f "$f" --argjson from "$((cursor - 5000))" \
      '{size: 500, sort: [{"event.ingested": "asc"}], query: {bool: {filter: [$f, {range: {"event.ingested": {gte: $from}}}]}}}')")
    jq -c --argjson seen "$(jq -c '.seen' <<<"$state")" \
      '[.hits.hits[] | select((._id | IN($seen[].id)) | not)]' <<<"$resp" | print_hits
    state=$(jq -c --argjson old "$(jq -c '.seen' <<<"$state")" --argjson cur "$cursor" "$update" <<<"$resp")
  done
}

sample_lines() { # four lines of the contract, stamped now
  local now rid=0123456789abcdef0123456789abcdef user=11111111-1111-4111-8111-111111111111
  local camp=22222222-2222-4222-8222-222222222222
  now=$(date -u +%Y-%m-%dT%H:%M:%S.123456789Z)
  jq -nc --arg t "$now" --arg r "$rid" --arg u "$user" --arg c "$camp" '
    {time: $t, severity: "INFO", message: "http request", service: "meurpg-api", version: "sample",
     request_id: $r, user_id: $u, campaign_id: $c, method: "POST",
     path: "/meurpg.campaigns.v1.CampaignService/CreateCampaign", status: 200, duration_ms: 12.5},
    {time: $t, severity: "DEBUG", message: "rpc", service: "meurpg-api", version: "sample",
     request_id: $r, user_id: $u, campaign_id: $c,
     procedure: "/meurpg.campaigns.v1.CampaignService/CreateCampaign", code: "ok", stream: false, duration_ms: 11.9},
    {time: $t, severity: "ERROR", message: "rpc", service: "meurpg-api", version: "sample",
     request_id: $r, user_id: $u, campaign_id: $c,
     procedure: "/meurpg.play.v1.PlayService/StartSession", code: "internal", stream: false,
     duration_ms: 750.2, error: "sample failure", reason: "SAMPLE",
     "logging.googleapis.com/trace": "projects/sample/traces/0123456789abcdef0123456789abcdef"},
    {time: $t, severity: "DEBUG", message: "event", service: "meurpg-api", version: "sample",
     request_id: $r, user_id: $u, campaign_id: $c, event: "session.started", session_id: "33333333-3333-4333-8333-333333333333"}'
}

cmd=${1:-}
[ $# -eq 0 ] || shift
case "$cmd" in
  errors)
    since=${1:-1h}; check_since "$since"
    run "$(qs 'log.level:error OR http.response.status_code:>=500')" "$since" ;;
  request)
    [ $# -ge 1 ] || die "usage: logs.sh request <request id>"; check_id "$1"
    resp=$(read_api POST "/$INDEX/_search" "$(body "$(term http.request.id "$1")" all asc 1000)")
    jq -c '.hits.hits' <<<"$resp" | print_hits ;;
  campaign)
    [ $# -ge 1 ] || die "usage: logs.sh campaign <campaign id> [since]"; check_id "$1"; since=${2:-1h}; check_since "$since"
    run "$(term meurpg.campaign_id "$1")" "$since" ;;
  user)
    [ $# -ge 1 ] || die "usage: logs.sh user <user id> [since]"; check_id "$1"; since=${2:-1h}; check_since "$since"
    run "$(term user.id "$1")" "$since" ;;
  search)
    [ $# -ge 1 ] || die "usage: logs.sh search '<query>' [since]"; since=${2:-1h}; check_since "$since"
    run "$(qs "$1")" "$since" ;;
  tail)
    cmd_tail "${1:-*}" ;;
  health)
    h=$(admin_api GET /_cluster/health)
    n=$(read_api POST "/$INDEX/_count" '{}' | jq -r '.count')
    echo "cluster=$(jq -r '.status' <<<"$h") log_lines=$n" ;;
  sample)
    dir=${MEURPG_HOME:-$HOME/.meurpg}/run-sample
    mkdir -p "$dir"
    sample_lines >>"$dir/api.log"
    echo "appended 4 lines to $dir/api.log; request id 0123456789abcdef0123456789abcdef" ;;
  simulate)
    docs=$(sample_lines | jq -Rsc 'split("\n") | map(select(length > 0)) | {docs: map({_source: {message: .}})}')
    admin_api POST /_ingest/pipeline/meurpg-logs/_simulate "$docs" | jq -c '.docs[] | .doc._source // .error' ;;
  ""|-h|--help|help)
    sed -n '2,/^set -euo/p' "${BASH_SOURCE[0]}" | sed '$d' | sed 's/^# \{0,1\}//' ;;
  *) die "unknown command '$cmd' (try: logs.sh help)" ;;
esac
