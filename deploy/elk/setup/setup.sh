#!/usr/bin/env bash
# One-shot (service `setup`): everything Elasticsearch needs before logs arrive.
# Idempotent: every call is a PUT (or a create-if-missing), so running it again changes nothing.
#   1. waits for Elasticsearch;
#   2. sets the kibana_system password;
#   3. roles and users: meurpg_filebeat (write only) and meurpg_reader (read only), on logs-meurpg-*;
#   4. the ILM policy meurpg-logs (delete after 14 days), the ingest pipeline meurpg-logs and the
#      index template logs-meurpg;
#   5. the data stream logs-meurpg-default.
# Needs ELASTIC_PASSWORD, KIBANA_PASSWORD, FILEBEAT_PASSWORD and READER_PASSWORD in the environment.
set -euo pipefail

ES=https://es01:9200
CA=/certs/ca/ca.crt
SETUP=/setup

for v in ELASTIC_PASSWORD KIBANA_PASSWORD FILEBEAT_PASSWORD READER_PASSWORD; do
  value=${!v:-}
  [ -n "$value" ] || { echo "setup: $v is empty" >&2; exit 1; }
  # The passwords go into JSON bodies below: refuse characters that would need escaping.
  if [[ "$value" =~ [^A-Za-z0-9._~+=-] ]]; then
    echo "setup: $v has characters outside A-Z a-z 0-9 . _ ~ + = -" >&2
    exit 1
  fi
done

# es METHOD PATH [curl args...]: an authenticated call as the superuser; fails on a 4xx/5xx and shows the body.
es() {
  local method=$1 path=$2
  shift 2
  local out
  out=$(printf 'user = "elastic:%s"\n' "$ELASTIC_PASSWORD" |
    curl -sS --fail-with-body --cacert "$CA" -K - -X "$method" \
      -H 'Content-Type: application/json' "$ES$path" "$@" 2>&1) || {
    echo "setup: $method $path failed: $out" >&2
    return 1
  }
  printf '%s\n' "$out"
}

echo "setup: waiting for Elasticsearch"
for i in $(seq 1 60); do
  if es GET / >/dev/null 2>&1; then break; fi
  [ "$i" -lt 60 ] || { echo "setup: Elasticsearch did not answer in 5 minutes" >&2; exit 1; }
  sleep 5
done

echo "setup: kibana_system password"
es POST /_security/user/kibana_system/_password -d "{\"password\":\"$KIBANA_PASSWORD\"}" >/dev/null

echo "setup: roles and users"
es PUT /_security/role/meurpg_writer --data-binary @"$SETUP/role-writer.json" >/dev/null
es PUT /_security/role/meurpg_reader --data-binary @"$SETUP/role-reader.json" >/dev/null
es PUT /_security/user/meurpg_filebeat \
  -d "{\"password\":\"$FILEBEAT_PASSWORD\",\"roles\":[\"meurpg_writer\"],\"full_name\":\"MeuRPG Filebeat (write only)\"}" >/dev/null
es PUT /_security/user/meurpg_reader \
  -d "{\"password\":\"$READER_PASSWORD\",\"roles\":[\"meurpg_reader\"],\"full_name\":\"MeuRPG log reader (read only)\"}" >/dev/null

echo "setup: ILM policy (delete after 14 days), ingest pipeline, index template"
es PUT /_ilm/policy/meurpg-logs --data-binary @"$SETUP/ilm.json" >/dev/null
es PUT /_ingest/pipeline/meurpg-logs --data-binary @"$SETUP/pipeline.json" >/dev/null
es PUT /_index_template/logs-meurpg --data-binary @"$SETUP/template.json" >/dev/null

echo "setup: data stream logs-meurpg-default"
if ! es GET /_data_stream/logs-meurpg-default >/dev/null 2>&1; then
  es PUT /_data_stream/logs-meurpg-default >/dev/null
fi

echo "setup: done"
