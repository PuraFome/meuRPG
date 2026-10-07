#!/usr/bin/env bash
# One-shot (service `certs`): makes the CA and the TLS certificates once, into the `certs` volume.
# The public CA is also copied to deploy/elk/certs/ca.crt so logs.sh can verify the server.
# Safe to run again: existing certificates are kept (delete the volume to rotate them).
set -euo pipefail

CERTS=/certs
cd "$CERTS"

BIN=/usr/share/elasticsearch/bin/elasticsearch-certutil

if [ ! -f ca/ca.crt ]; then
  echo "certs: creating the CA"
  "$BIN" ca --silent --pem --days 3650 --out "$CERTS/ca.zip"
  unzip -q "$CERTS/ca.zip" -d "$CERTS"
fi

if [ ! -f es01/es01.crt ]; then
  echo "certs: creating the Elasticsearch and Kibana certificates"
  cat > "$CERTS/instances.yml" <<'YML'
instances:
  - name: es01
    dns: [es01, localhost]
    ip: [127.0.0.1]
  - name: kibana
    dns: [kibana, localhost]
    ip: [127.0.0.1]
YML
  "$BIN" cert --silent --pem --days 1095 --in "$CERTS/instances.yml" \
    --ca-cert "$CERTS/ca/ca.crt" --ca-key "$CERTS/ca/ca.key" --out "$CERTS/certs.zip"
  unzip -q "$CERTS/certs.zip" -d "$CERTS"
fi

# Elasticsearch and Kibana run as uid 1000, group 0.
chown -R 1000:0 "$CERTS"
find "$CERTS" -type d -exec chmod 750 {} +
find "$CERTS" -type f -exec chmod 640 {} +

# The public CA only (never a key) for the host.
if [ -d /host-certs ]; then
  cp "$CERTS/ca/ca.crt" /host-certs/ca.crt
  chmod 644 /host-certs/ca.crt
fi
echo "certs: done"
