// Package db connects to CockroachDB through pgx and holds the two patterns
// every repository needs: a tuned connection pool (NewPool) and transactions
// that retry automatically when CockroachDB asks for it (InTx).
package db

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgconn/ctxwatch"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool defaults. Each one is applied only when the connection string does
// not set it, so production can tune the pool without a code change, e.g.
// postgresql://...?pool_max_conns=10&connect_timeout=10
const (
	// Fail fast when the database is unreachable instead of hanging a
	// request for pgx's default of two minutes.
	defaultConnectTimeout = 5 * time.Second

	// How long a canceled call waits for the server to honor the cancel
	// request before the connection's deadline is forced.
	cancelDeadlineFallback = 5 * time.Second

	// Connections are recycled after MaxConnLifetime (pgx default: 1h). The
	// jitter spreads those reconnects out in time, so a CockroachDB node
	// that restarts gets its share of connections back gradually instead of
	// all at once. CockroachDB's docs recommend it for connection pools.
	defaultMaxConnLifetimeJitter = 5 * time.Minute

	// How many connections the pool opens at most when the connection string
	// has no pool_max_conns. pgx's own default is max(4, NumCPU), which is 4 on
	// the 1-vCPU Cloud Run instance. Why 10 (docs/operations/reference.md#database-connections): production is one instance (max-instances = 1) and a table is
	// one master and up to six players, each request a short transaction, so a
	// handful are in flight at once; 10 leaves room for the live streams'
	// periodic reads and a burst, and is far under CockroachDB's guidance of
	// about four connections per vCPU of the cluster. It is a safety net, not the
	// fix: no code path may hold a connection (a transaction) while it asks the
	// pool for a second one, whatever the size, and the tests run every pool at
	// one connection to prove it (platform/dbtest).
	defaultMaxConns = 10

	// A statement that runs longer than this is canceled by the database (SQLSTATE
	// 57014), and a transaction left open and idle longer than the second one is
	// closed. Both default to "off" in CockroachDB, and the Cloud Run request
	// timeout is 35 minutes, so without them one stuck statement or a transaction
	// abandoned halfway would hold one of the 10 connections, and its locks, for
	// that long. Every statement the API runs is short (a table's worth of rows by
	// primary key, or a small scan); the slow work (drawing a dungeon, decoding an
	// image, calling the model) is Go code between statements and never inside a
	// transaction. The migrations run in cmd/migrate, which has its own connection
	// and no timeout, because a backfill may take long (docs/operations/reference.md#database-connections).
	defaultStatementTimeout         = 30 * time.Second
	defaultIdleInTransactionTimeout = 60 * time.Second

	// Shows up in CockroachDB's SQL activity pages, which makes it easy to
	// tell the API's queries apart from a human using the SQL shell.
	applicationName = "meurpg"
)

// NewPool returns a pgx connection pool for databaseURL.
//
// The pool connects lazily: this function only validates the URL and does
// not talk to the database. Call Ping to check connectivity.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return NewPoolWith(ctx, databaseURL, nil)
}

// NewPoolWith is NewPool with a last say on the configuration: tweak, when
// not nil, runs after the defaults are applied. Tests use it to shrink the
// pool to one connection (see platform/dbtest); production uses NewPool.
func NewPoolWith(ctx context.Context, databaseURL string, tweak func(*pgxpool.Config)) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		// The parse error would echo the connection string, which contains
		// the password. pgx tries to redact it, but only on a best-effort
		// basis, so we return a fixed message instead.
		return nil, errors.New("DATABASE_URL is not a valid PostgreSQL connection string")
	}

	// pgx parses pool_max_conns out of the string and gives MaxConns its own default
	// (4) when it is missing, so the config cannot tell "set to 4" from "not set":
	// the string itself is asked.
	if !strings.Contains(databaseURL, "pool_max_conns") {
		cfg.MaxConns = defaultMaxConns
	}
	if cfg.ConnConfig.ConnectTimeout == 0 {
		cfg.ConnConfig.ConnectTimeout = defaultConnectTimeout
	}
	if cfg.MaxConnLifetimeJitter == 0 {
		cfg.MaxConnLifetimeJitter = defaultMaxConnLifetimeJitter
	}
	if _, ok := cfg.ConnConfig.RuntimeParams["application_name"]; !ok {
		cfg.ConnConfig.RuntimeParams["application_name"] = applicationName
	}

	// Session settings, in milliseconds, sent when each connection opens. A value
	// the URL already sets (...&statement_timeout=60000) wins, as for the rest.
	setSessionDefault(cfg, "statement_timeout", defaultStatementTimeout)
	setSessionDefault(cfg, "idle_in_transaction_session_timeout", defaultIdleInTransactionTimeout)

	// A canceled call must not touch the connection's read deadline. pgx's
	// default handler cancels by setting the deadline to now; when that
	// lands while the background reader of a slow write is parked on the
	// socket, its timeout error stays queued and the NEXT call on that pooled
	// connection fails at once with "timeout: read tcp ...: i/o timeout"
	// although its own context is alive (issue #710). Asking the server to
	// cancel leaves the deadline alone; it is only the fallback.
	cfg.ConnConfig.BuildContextWatcherHandler = func(c *pgconn.PgConn) ctxwatch.Handler {
		return &pgconn.CancelRequestContextWatcherHandler{
			Conn:               c,
			CancelRequestDelay: 0,
			DeadlineDelay:      cancelDeadlineFallback,
		}
	}

	if tweak != nil {
		tweak(cfg)
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

// setSessionDefault makes every new connection start with the session variable
// name set to d, unless the connection string already set it.
func setSessionDefault(cfg *pgxpool.Config, name string, d time.Duration) {
	if _, ok := cfg.ConnConfig.RuntimeParams[name]; !ok {
		cfg.ConnConfig.RuntimeParams[name] = strconv.FormatInt(d.Milliseconds(), 10)
	}
}
