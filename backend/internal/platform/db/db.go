// Package db connects to CockroachDB through pgx and holds the two patterns
// every repository needs: a tuned connection pool (NewPool) and transactions
// that retry automatically when CockroachDB asks for it (InTx).
package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool defaults. Each one is applied only when the connection string does
// not set it, so production can tune the pool without a code change, e.g.
// postgresql://...?pool_max_conns=10&connect_timeout=10
const (
	// Fail fast when the database is unreachable instead of hanging a
	// request for pgx's default of two minutes.
	defaultConnectTimeout = 5 * time.Second

	// Connections are recycled after MaxConnLifetime (pgx default: 1h). The
	// jitter spreads those reconnects out in time, so a CockroachDB node
	// that restarts gets its share of connections back gradually instead of
	// all at once. CockroachDB's docs recommend it for connection pools.
	defaultMaxConnLifetimeJitter = 5 * time.Minute

	// Shows up in CockroachDB's SQL activity pages, which makes it easy to
	// tell the API's queries apart from a human using the SQL shell.
	applicationName = "meurpg"
)

// NewPool returns a pgx connection pool for databaseURL.
//
// The pool connects lazily: this function only validates the URL and does
// not talk to the database. Call Ping to check connectivity.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		// The parse error would echo the connection string, which contains
		// the password. pgx tries to redact it, but only on a best-effort
		// basis, so we return a fixed message instead.
		return nil, errors.New("DATABASE_URL is not a valid PostgreSQL connection string")
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

	return pgxpool.NewWithConfig(ctx, cfg)
}
