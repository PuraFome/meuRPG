#!/bin/bash
# SessionStart hook of the cloud sessions that work on the fork (vfraga/meuRPG). It lives on the
# fork's main only; the train build drops .claude/ so it never reaches PuraFome/meuRPG.
#
# A VM keeps its files when it is restored but not its processes, so every start and resume brings up
# the VM's PostgreSQL 16 (port 5432, user root without a password, SERIALIZABLE, fsync off, and the
# max_connections and max_pred_locks_per_transaction of CI's start-postgres.sh) and hands the session
# MEURPG_TEST_DATABASE_URL through CLAUDE_ENV_FILE, the file Claude Code reads for the session's
# environment. The tests that need CockroachDB itself (backend/cockroach-only-tests.txt) run on a
# CockroachDB the session starts when it needs one: `make db-test-cockroach-start` (localhost:26258).
# The hook also installs the pre-push hook (.claude/hooks/pre-push), which runs CI's linters on what a
# work branch changed before it is pushed. Outside a cloud session it does nothing.
[ "${CLAUDE_CODE_REMOTE:-}" = "true" ] || exit 0
cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0

LOG=/tmp/meurpg-session-start.log
URL='postgresql://root@127.0.0.1:5432/postgres?sslmode=disable'
SUDO=
[ "$(id -u)" = 0 ] || SUDO='sudo -n'
PGCONF=/etc/postgresql/16/main

{
  date
  hooks="$(git rev-parse --git-common-dir)/hooks"
  mkdir -p "$hooks" && install -m 755 .claude/hooks/pre-push "$hooks/pre-push"
  if ! grep -q '^host all root 127.0.0.1/32 trust' $PGCONF/pg_hba.conf; then
    $SUDO sed -i '1i host all root 127.0.0.1/32 trust' $PGCONF/pg_hba.conf
  fi
  pg_lsclusters | grep -q ' online ' || $SUDO pg_ctlcluster 16 main start
  for _ in $(seq 1 30); do pg_isready -q -h 127.0.0.1 -p 5432 && break; sleep 1; done
  # The settings are applied (and the server restarted) only when they are not in force yet.
  if [ "$(psql "$URL" -Atc 'SHOW max_pred_locks_per_transaction' 2>/dev/null)" != 256 ]; then
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
} >>"$LOG" 2>&1

if psql "$URL" -Atc 'SELECT 1' >/dev/null 2>&1; then
  [ -n "${CLAUDE_ENV_FILE:-}" ] && echo "export MEURPG_TEST_DATABASE_URL='$URL'" >>"$CLAUDE_ENV_FILE"
  echo "MeuRPG: the test PostgreSQL 16 is up on 127.0.0.1:5432 (MEURPG_TEST_DATABASE_URL='$URL'). Run database tests under flock /tmp/meurpg-test.lock. A test on backend/cockroach-only-tests.txt needs CockroachDB: run make db-test-cockroach-start and point MEURPG_TEST_DATABASE_URL at localhost:26258 for it."
else
  echo "MeuRPG: the test PostgreSQL did not start (log: $LOG). Start it with the commands in the hook (.claude/hooks/cloud-session-start.sh) before any database test."
fi
exit 0
