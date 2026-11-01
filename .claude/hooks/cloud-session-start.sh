#!/bin/bash
# SessionStart hook of the cloud sessions that work on the fork (vfraga/meuRPG). It lives on the
# fork's main only; the train build drops .claude/ so it never reaches PuraFome/meuRPG.
#
# The environment's setup script installs Docker, the CockroachDB image and the native cockroach
# binary. A VM keeps its files when it is restored but not its processes, so every start and resume
# brings up dockerd and the in-memory test CockroachDB (`make db-test-start`, localhost:26258) and
# hands the session MEURPG_TEST_DATABASE_URL. It also installs the pre-push hook (.claude/hooks/pre-push),
# which runs CI's linters on what a work branch changed before it is pushed. Outside a cloud session it
# does nothing.
[ "${CLAUDE_CODE_REMOTE:-}" = "true" ] || exit 0
cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0

LOG=/tmp/meurpg-session-start.log
URL='postgresql://root@localhost:26258/defaultdb?sslmode=disable'
SUDO=
[ "$(id -u)" = 0 ] || SUDO='sudo -n'

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
  make db-test-start
} >>"$LOG" 2>&1

if cockroach sql --insecure --host=localhost:26258 -e 'SELECT 1' >/dev/null 2>&1; then
  [ -n "${CLAUDE_ENV_FILE:-}" ] && echo "export MEURPG_TEST_DATABASE_URL='$URL'" >>"$CLAUDE_ENV_FILE"
  echo "MeuRPG: the test CockroachDB is up on localhost:26258 (MEURPG_TEST_DATABASE_URL='$URL'). Run database tests under flock /tmp/meurpg-test.lock."
else
  echo "MeuRPG: the test CockroachDB did not start (log: $LOG). Start Docker and run make db-test-start before any database test."
fi
exit 0
