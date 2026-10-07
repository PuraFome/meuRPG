package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/internal/platform/testenv"
)

// testPool connects to the database in MEURPG_TEST_DATABASE_URL, or skips the
// test when the variable is unset, so `go test ./...` works without Docker.
//
// Start a throwaway CockroachDB with:
//
//	docker run -d --rm --name crdb -p 26257:26257 cockroachdb/cockroach:v26.3.2 start-single-node --insecure
//	export MEURPG_TEST_DATABASE_URL='postgresql://root@localhost:26257/defaultdb?sslmode=disable'
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := testenv.DatabaseURL(t)

	pool, err := NewPool(t.Context(), url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestPoolPing(t *testing.T) {
	t.Parallel()
	pool := testPool(t)

	if err := pool.Ping(t.Context()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	var appName string
	if err := pool.QueryRow(t.Context(), "SHOW application_name").Scan(&appName); err != nil {
		t.Fatalf("SHOW application_name: %v", err)
	}
	if appName != applicationName {
		t.Errorf("application_name = %q, want %q", appName, applicationName)
	}
}

// TestInTxRetriesRealConflict provokes a genuine serialization failure and
// checks that InTx recovers from it:
//
//  1. The transaction reads a counter.
//  2. Meanwhile, another connection increments the same counter and commits.
//  3. The transaction then writes a value based on its (now stale) read.
//
// Under SERIALIZABLE isolation CockroachDB cannot commit step 3, so it
// aborts with SQLSTATE 40001. InTx runs fn again, the second attempt reads
// the fresh value, and nothing is lost.
func TestInTxRetriesRealConflict(t *testing.T) {
	t.Parallel()
	pool := testPool(t)
	ctx := t.Context()

	table := pgx.Identifier{fmt.Sprintf("retry_test_%d", time.Now().UnixNano())}.Sanitize()
	mustExec(t, pool, "CREATE TABLE "+table+" (id INT PRIMARY KEY, n INT NOT NULL)")
	t.Cleanup(func() {
		// t.Context() is already canceled during cleanup.
		mustExec(t, pool, "DROP TABLE IF EXISTS "+table)
	})
	mustExec(t, pool, "INSERT INTO "+table+" (id, n) VALUES (1, 0)")

	attempts := 0
	err := InTx(ctx, pool, func(tx pgx.Tx) error {
		attempts++

		var n int
		if err := tx.QueryRow(ctx, "SELECT n FROM "+table+" WHERE id = 1").Scan(&n); err != nil {
			return fmt.Errorf("read counter: %w", err)
		}

		if attempts == 1 {
			// A concurrent writer, outside our transaction.
			if _, err := pool.Exec(ctx, "UPDATE "+table+" SET n = n + 1 WHERE id = 1"); err != nil {
				return fmt.Errorf("concurrent update: %w", err)
			}
		}

		if _, err := tx.Exec(ctx, "UPDATE "+table+" SET n = $1 WHERE id = 1", n+10); err != nil {
			return fmt.Errorf("write counter: %w", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTx() error = %v", err)
	}

	t.Logf("fn ran %d times", attempts)
	if attempts < 2 {
		t.Errorf("fn ran %d time(s), want at least 2 (the first attempt must hit a 40001)", attempts)
	}

	var n int
	if err := pool.QueryRow(ctx, "SELECT n FROM "+table+" WHERE id = 1").Scan(&n); err != nil {
		t.Fatalf("read final counter: %v", err)
	}
	// 1 from the concurrent writer + 10 from our transaction. A lost update
	// would leave 10.
	if n != 11 {
		t.Errorf("counter = %d, want 11", n)
	}
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}
