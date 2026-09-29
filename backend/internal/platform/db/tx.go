package db

import (
	"context"
	"time"

	"github.com/cockroachdb/cockroach-go/v2/crdb"
	crdbpgx "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5"
	"github.com/jackc/pgx/v5"
)

// TxStarter is anything that can begin a transaction. Both *pgxpool.Pool and
// *pgx.Conn satisfy it, and so can a fake in tests.
type TxStarter = crdbpgx.Conn

// defaultRetryPolicy waits 10ms, 20ms, 40ms... (capped at 500ms) between
// attempts and gives up after 8 retries (about 1.6s of waiting in total).
// Backing off, instead of retrying immediately, gives the transaction we
// collided with time to finish.
var defaultRetryPolicy crdb.RetryPolicy = &crdb.ExpBackoffRetryPolicy{
	RetryLimit: 8,
	BaseDelay:  10 * time.Millisecond,
	MaxDelay:   500 * time.Millisecond,
}

// InTx runs fn inside a transaction and commits it when fn returns nil.
//
// Why retries: CockroachDB always runs transactions with SERIALIZABLE
// isolation. When two transactions touch the same rows concurrently, it may
// abort one of them with SQLSTATE 40001 ("restart transaction") instead of
// letting them produce an inconsistent result. The fix is simply to run the
// transaction again, and InTx does that for you, using the official
// cockroach-go helper.
//
// Rules for fn, because it may run more than once:
//   - Talk to the database only through tx.
//   - No side effects outside the database (sending e-mails, calling other
//     APIs, publishing events). Do those after InTx returns.
//   - When adding context to an error, wrap it with %w
//     (fmt.Errorf("insert character: %w", err)); otherwise InTx cannot see
//     the 40001 code and will not retry.
//
// Any other error rolls the transaction back and is returned as is.
func InTx(ctx context.Context, conn TxStarter, fn func(pgx.Tx) error) error {
	return inTx(ctx, conn, defaultRetryPolicy, fn)
}

// inTx is InTx with a configurable retry policy, so tests can use a policy
// without delays.
func inTx(ctx context.Context, conn TxStarter, policy crdb.RetryPolicy, fn func(pgx.Tx) error) error {
	ctx = crdb.WithRetryPolicy(ctx, policy)
	return crdbpgx.ExecuteTx(ctx, conn, pgx.TxOptions{}, fn)
}
