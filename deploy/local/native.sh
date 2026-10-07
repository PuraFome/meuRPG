#!/usr/bin/env bash
# The local stack without Docker (`make up LOCAL_STACK=native`, a Mac).
#
# The API and devidp run as processes on this machine, against the native
# CockroachDB on localhost:26257 (`make db-native-start`), with the same
# settings as deploy/local/compose.yaml: the app on http://localhost:8080 and
# devidp on http://idp.localhost:9090. Docker is not needed at all, so its
# Linux VM (several GB on a Mac) can stay off, and that memory goes to the
# in-memory test databases instead. CI keeps using Docker.
#
# Usage: deploy/local/native.sh up | down | logs | status
#
# Each process writes a PID file and a log under the stack's run folder
# (~/.meurpg/run by default). `down` stops exactly those PIDs, nothing else.
#
# A second stack, for a second e2e run at the same time, takes other ports
# and its own database, run folder and images, so the two never share test
# accounts or data:
#   API_PORT=8180 IDP_PORT=9190 DB_NAME=meurpg_2 deploy/local/native.sh up
# and its tests run with E2E_BASE_URL=http://localhost:8180
# E2E_IDP_ORIGIN=http://idp.localhost:9190.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
API_PORT=${API_PORT:-8080}
IDP_PORT=${IDP_PORT:-9090}
DB_NAME=${DB_NAME:-meurpg}
if [ "$API_PORT" = 8080 ]; then
  RUN_DIR=${MEURPG_RUN_DIR:-$HOME/.meurpg/run}
  # Uploaded images live outside the repo, like the Docker volume; the two
  # are separate folders, so images uploaded in one stack don't show in the
  # other.
  BLOB_DIR=${BLOB_DIR:-$HOME/.meurpg/images}
else
  RUN_DIR=${MEURPG_RUN_DIR:-$HOME/.meurpg/run-$API_PORT}
  BLOB_DIR=${BLOB_DIR:-$HOME/.meurpg/images-$API_PORT}
fi
BIN_DIR=$RUN_DIR/bin
DATABASE_URL=${DATABASE_URL:-postgresql://root@localhost:26257/$DB_NAME?sslmode=disable}
WEB_DIR=$ROOT/web/dist/web/browser

# The same issuer as compose.yaml, so the browser, the API and the saved e2e
# sessions all agree. macOS's resolver sends any *.localhost name to the
# loopback address (RFC 6761); Go uses it by default on a Mac (its pure-Go
# resolver, GODEBUG=netdns=go, does not, and neither does Linux by default).
ISSUER=http://idp.localhost:$IDP_PORT
CLIENT_ID=meurpg-local
CLIENT_SECRET=meurpg-local-secret
REDIRECT_URL=http://localhost:$API_PORT/auth/callback

# is_running NAME: true when NAME's PID file names a live process.
is_running() {
  local pid_file=$RUN_DIR/$1.pid
  [ -f "$pid_file" ] && kill -0 "$(cat "$pid_file")" 2>/dev/null
}

# stop NAME: stops the process in NAME's PID file, if it is alive.
stop() {
  local pid_file=$RUN_DIR/$1.pid
  if is_running "$1"; then
    local pid
    pid=$(cat "$pid_file")
    kill "$pid"
    while kill -0 "$pid" 2>/dev/null; do sleep 0.2; done
    echo "$1 stopped"
  fi
  rm -f "$pid_file"
}

# port_busy PORT: true when something on this machine listens on PORT.
port_busy() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1
}

# wait_for URL NAME SECONDS: polls URL until it answers 200.
wait_for() {
  local url=$1 name=$2 seconds=$3 i
  for ((i = 0; i < seconds * 5; i++)); do
    if curl -sf -o /dev/null "$url"; then return 0; fi
    if ! is_running "$name"; then
      echo "$name exited; the end of $RUN_DIR/$name.log:" >&2
      tail -20 "$RUN_DIR/$name.log" >&2
      return 1
    fi
    sleep 0.2
  done
  echo "$name did not answer $url in ${seconds}s; the end of $RUN_DIR/$name.log:" >&2
  tail -20 "$RUN_DIR/$name.log" >&2
  return 1
}

# build_web: builds the Angular app when its build is missing or older than
# any source file. compose's `up --build` rebuilds it every time; here an
# unchanged app costs nothing.
build_web() {
  local stamp=$WEB_DIR/index.html
  if [ ! -d "$ROOT/web/node_modules" ]; then
    (cd "$ROOT/web" && npm ci --ignore-scripts)
  fi
  if [ -f "$stamp" ] && [ -z "$(find "$ROOT/web/src" "$ROOT/web/angular.json" "$ROOT/web/package-lock.json" \
    -newer "$stamp" -print -quit)" ]; then
    echo "web: the build is up to date"
    return
  fi
  echo "web: building (npm run build)"
  (cd "$ROOT/web" && npm run build)
}

up() {
  if ! cockroach sql --insecure --host=localhost:26257 -e 'SELECT 1' >/dev/null 2>&1; then
    echo "No CockroachDB on localhost:26257: run \`make db-native-start\` first." >&2
    exit 1
  fi
  cockroach sql --insecure --host=localhost:26257 -e "CREATE DATABASE IF NOT EXISTS $DB_NAME" >/dev/null
  # A running native stack is replaced; anything else on the ports is not ours.
  stop api
  stop devidp
  for port in "$API_PORT" "$IDP_PORT"; do
    if port_busy "$port"; then
      echo "Port $port is in use (the Docker stack? \`make down\` stops it)." >&2
      exit 1
    fi
  done

  mkdir -p "$BIN_DIR" "$BLOB_DIR"
  echo "go: building the API, migrate and devidp"
  (
    cd "$ROOT/backend"
    go build -o "$BIN_DIR/api" \
      -ldflags "-X main.version=dev -X main.commit=$(git -C "$ROOT" rev-parse --short HEAD)" ./cmd/api
    go build -o "$BIN_DIR/migrate" ./cmd/migrate
    go build -o "$BIN_DIR/devidp" ./cmd/devidp
  )
  build_web

  echo "migrate: up"
  DATABASE_URL=$DATABASE_URL "$BIN_DIR/migrate" up

  # nohup and a PID file per process: they outlive this script, and `down`
  # stops exactly them.
  DEVIDP_ISSUER=$ISSUER DEVIDP_LISTEN=127.0.0.1:$IDP_PORT DEVIDP_CLIENT_ID=$CLIENT_ID \
    DEVIDP_CLIENT_SECRET=$CLIENT_SECRET DEVIDP_REDIRECT_URIS=$REDIRECT_URL \
    nohup "$BIN_DIR/devidp" >"$RUN_DIR/devidp.log" 2>&1 &
  echo $! >"$RUN_DIR/devidp.pid"
  wait_for "http://localhost:$IDP_PORT/.well-known/openid-configuration" devidp 30

  DATABASE_URL=$DATABASE_URL PORT=$API_PORT LISTEN_HOST=127.0.0.1 LOG_LEVEL=${LOG_LEVEL:-debug} \
    OIDC_ISSUER=$ISSUER OIDC_CLIENT_ID=$CLIENT_ID OIDC_CLIENT_SECRET=$CLIENT_SECRET \
    OIDC_REDIRECT_URL=$REDIRECT_URL OIDC_MAX_AGE=1h \
    BLOB_DIR=$BLOB_DIR WEB_DIR=$WEB_DIR IMAGE_GENERATOR=fake RATE_LIMIT_MULTIPLIER=10 \
    nohup "$BIN_DIR/api" >"$RUN_DIR/api.log" 2>&1 &
  echo $! >"$RUN_DIR/api.pid"
  wait_for "http://localhost:$API_PORT/readyz" api 60

  echo "The app is on http://localhost:$API_PORT (devidp: $ISSUER, database $DB_NAME). Logs: make logs LOCAL_STACK=native. Stop: make down LOCAL_STACK=native (with the same API_PORT)"
}

down() {
  stop api
  stop devidp
}

logs() {
  tail -n 50 -F "$RUN_DIR/api.log" "$RUN_DIR/devidp.log"
}

status() {
  for name in api devidp; do
    if is_running "$name"; then
      echo "$name: running (PID $(cat "$RUN_DIR/$name.pid"))"
    else
      echo "$name: stopped"
    fi
  done
}

case "${1:-}" in
up) up ;;
down) down ;;
logs) logs ;;
status) status ;;
*)
  echo "usage: $0 up | down | logs | status" >&2
  exit 2
  ;;
esac
