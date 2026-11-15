package dbtest

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// A test that proves a request is stopped at a lock waits, on a condition and not on a clock, until the statement
// is waiting. The servers show it in different views (PostgreSQL's pg_stat_activity, CockroachDB's crdb_internal), and
// these helpers hide the difference.

// Querier is what a pool, a connection or a transaction of pgx offers.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func onCockroach(ctx context.Context, q Querier) (bool, error) {
	var version string
	if err := q.QueryRow(ctx, `SELECT version()`).Scan(&version); err != nil {
		return false, fmt.Errorf("ask the server its version: %w", err)
	}
	return strings.Contains(version, "CockroachDB"), nil
}

// WaitingForTable counts the transactions of the observer's database that wait right now for a lock on a row of the table,
// whatever statement waits. It finds the waiter in the server's own view of its locks, not in the text of a statement: a write
// stopped at a row may be at a plain read of it (CockroachDB makes a plain read of a row another transaction holds FOR UPDATE
// wait) or at the statement that locks it, and the text of either is the code's to change.
//
// PostgreSQL: a backend whose wait event is a lock, in pg_stat_activity, whose statement names the table (a plain read never
// waits there, so the waiting statement is the one that locks). CockroachDB: the requests that are not granted, in
// crdb_internal.cluster_locks, which lists a waiting plain read and a waiting locking read alike and names the table. Each test
// has a database of its own, so what waits in it is the test's.
func WaitingForTable(ctx context.Context, observer Querier, table string) (int, error) {
	cockroach, err := onCockroach(ctx, observer)
	if err != nil {
		return 0, err
	}
	var n int
	if cockroach {
		err = observer.QueryRow(ctx, `SELECT count(DISTINCT txn_id) FROM crdb_internal.cluster_locks WHERE NOT granted AND database_name = current_database() AND table_name = $1`, table).Scan(&n)
	} else {
		err = observer.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND datname = current_database() AND pid <> pg_backend_pid() AND query LIKE '%' || $1 || '%'`, table).Scan(&n)
	}
	if err != nil {
		return 0, fmt.Errorf("look for the transactions waiting for %s: %w", table, err)
	}
	return n, nil
}

// WaitingStatements counts the statements other than the observer's own whose text matches like (a SQL LIKE pattern)
// and that wait for a lock: on PostgreSQL the ones that wait on a lock now, on CockroachDB the ones that are running.
// It matches text: a test that proves a write is stopped at a row uses WaitingForTable.
func WaitingStatements(ctx context.Context, observer Querier, like string) (int, error) {
	cockroach, err := onCockroach(ctx, observer)
	if err != nil {
		return 0, err
	}
	var n int
	if cockroach {
		err = observer.QueryRow(ctx, `SELECT count(*) FROM crdb_internal.cluster_queries WHERE query LIKE $1 AND query NOT LIKE '%cluster_queries%'`, like).Scan(&n)
	} else {
		err = observer.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND datname = current_database() AND query LIKE $1 AND pid <> pg_backend_pid()`, like).Scan(&n)
	}
	if err != nil {
		return 0, fmt.Errorf("look for the waiting statements: %w", err)
	}
	return n, nil
}
