#!/bin/bash
# SessionStart hook of the cloud sessions that work on the fork (vfraga/meuRPG). It lives on the
# fork's main only; the train build drops .claude/ so it never reaches PuraFome/meuRPG.
#
# A VM keeps its files when it is restored but not its processes, so every start and resume brings up
# three things:
#   - dockerd, which the e2e and `make up` need;
#   - the native cockroach binary, copied out of the CockroachDB image for a VM built before the
#     environment's setup script installed it (`make db-test-cockroach-start` needs it);
#   - the test database the backend tests use: the VM's PostgreSQL 16 (port 5432, user root without a
#     password, SERIALIZABLE, fsync off, and the max_connections and max_pred_locks_per_transaction of
#     CI's start-postgres.sh), which the base image installs. A VM without it gets the in-memory test
#     CockroachDB (`make db-test-cockroach-start`, localhost:26258) as before.
# The session's MEURPG_TEST_DATABASE_URL names that database; the hook writes it to CLAUDE_ENV_FILE, the
# file Claude Code reads for the session's environment. A test on backend/cockroach-only-tests.txt needs
# CockroachDB: the session starts it with `make db-test-cockroach-start` when it needs one.
# The hook also installs the pre-push hook (.claude/hooks/pre-push), which runs CI's linters on what a
# work branch changed before it is pushed. Outside a cloud session it does nothing.
[ "${CLAUDE_CODE_REMOTE:-}" = "true" ] || exit 0
cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0

LOG=/tmp/meurpg-session-start.log
PG_URL='postgresql://root@127.0.0.1:5432/postgres?sslmode=disable'
CRDB_URL='postgresql://root@localhost:26258/defaultdb?sslmode=disable'
PGCONF=/etc/postgresql/16/main
SUDO=
[ "$(id -u)" = 0 ] || SUDO='sudo -n'

have_postgres() { command -v pg_lsclusters >/dev/null && [ -d "$PGCONF" ]; }

start_postgres() {
  if ! grep -q '^host all root 127.0.0.1/32 trust' $PGCONF/pg_hba.conf; then
    $SUDO sed -i '1i host all root 127.0.0.1/32 trust' $PGCONF/pg_hba.conf
  fi
  pg_lsclusters | grep -q ' online ' || $SUDO pg_ctlcluster 16 main start
  for _ in $(seq 1 30); do pg_isready -q -h 127.0.0.1 -p 5432 && break; sleep 1; done
  # The settings are applied (and the server restarted) only when they are not in force yet.
  if [ "$(psql "$PG_URL" -Atc 'SHOW max_pred_locks_per_transaction' 2>/dev/null)" != 256 ]; then
    $SUDO su postgres -c "psql -qAt" <<'SQL'
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'root') THEN CREATE ROLE root SUPERUSER LOGIN; END IF;
END $$;
ALTER SYSTEM SET default_transaction_isolation = 'serializable';
ALTER SYSTEM SET fsync = off;
ALTER SYSTEM SET synchronous_commit = off;
ALTER SYSTEM SET full_page_writes = off;
ALTER SYSTEM SET max_connections = 1000;
ALTER SYSTEM SET max_pred_locks_per_transaction = 256;
SQL
    $SUDO pg_ctlcluster 16 main restart
    for _ in $(seq 1 30); do pg_isready -q -h 127.0.0.1 -p 5432 && break; sleep 1; done
  fi
}

{
  date
  hooks="$(git rev-parse --git-common-dir)/hooks"
  mkdir -p "$hooks" && install -m 755 .claude/hooks/pre-push "$hooks/pre-push"
  if ! docker info >/dev/null 2>&1; then
    $SUDO nohup dockerd >/tmp/dockerd.log 2>&1 &
    for _ in $(seq 1 30); do docker info >/dev/null 2>&1 && break; sleep 1; done
  fi
  # A VM built before the setup script installed the binary still has the image: copy it out.
  if ! command -v cockroach >/dev/null; then
    img=$(docker images -q cockroachdb/cockroach | head -1)
    if [ -n "$img" ] && cid=$(docker create "$img"); then
      $SUDO docker cp "$cid":/cockroach/cockroach /usr/local/bin/cockroach
      docker rm "$cid" >/dev/null
    fi
  fi
  if have_postgres; then start_postgres; else make db-test-cockroach-start; fi
} >>"$LOG" 2>&1

if have_postgres && psql "$PG_URL" -Atc 'SELECT 1' >/dev/null 2>&1; then
  URL=$PG_URL
  MSG="the test PostgreSQL 16 is up on 127.0.0.1:5432. A test on backend/cockroach-only-tests.txt needs CockroachDB: run make db-test-cockroach-start and point MEURPG_TEST_DATABASE_URL at localhost:26258 for it"
elif ! have_postgres && cockroach sql --insecure --host=localhost:26258 -e 'SELECT 1' >/dev/null 2>&1; then
  URL=$CRDB_URL
  MSG="this VM has no PostgreSQL 16, so the test CockroachDB is up on localhost:26258 (make db-test-cockroach-start); make test-postgres needs a PostgreSQL (make db-test-start)"
else
  echo "MeuRPG: the test database did not start (log: $LOG). Start PostgreSQL (.claude/hooks/cloud-session-start.sh) or make db-test-cockroach-start before any database test."
  exit 0
fi
[ -n "${CLAUDE_ENV_FILE:-}" ] && echo "export MEURPG_TEST_DATABASE_URL='$URL'" >>"$CLAUDE_ENV_FILE"
echo "MeuRPG: $MSG (MEURPG_TEST_DATABASE_URL='$URL'). Run database tests under flock /tmp/meurpg-test.lock."
exit 0
