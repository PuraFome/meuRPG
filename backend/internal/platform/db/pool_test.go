package db

import (
	"testing"
)

// TestNewPoolMaxConns pins the pool size: the default when the connection
// string says nothing, and the string's own pool_max_conns when it does. The
// pool connects lazily, so no database is needed.
func TestNewPoolMaxConns(t *testing.T) {
	const base = "postgresql://root@localhost:26257/defaultdb?sslmode=disable"
	for _, tc := range []struct {
		name string
		url  string
		want int32
	}{
		{"no setting", base, defaultMaxConns},
		{"set in the URL", base + "&pool_max_conns=3", 3},
		{"set to one", base + "&pool_max_conns=1", 1},
		{"set in key=value form", "host=localhost user=root dbname=defaultdb sslmode=disable pool_max_conns=7", 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, err := NewPool(t.Context(), tc.url)
			if err != nil {
				t.Fatalf("NewPool() error = %v", err)
			}
			defer pool.Close()
			if got := pool.Config().MaxConns; got != tc.want {
				t.Errorf("MaxConns = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestNewPoolSessionTimeouts: the defaults go out as runtime parameters (in
// milliseconds), and a value in the URL wins. No database is needed.
func TestNewPoolSessionTimeouts(t *testing.T) {
	const base = "postgresql://root@localhost:26257/defaultdb?sslmode=disable"
	for _, tc := range []struct {
		name      string
		url       string
		statement string
		idleInTxn string
	}{
		{"defaults", base, "30000", "60000"},
		{"set in the URL", base + "&statement_timeout=5000&idle_in_transaction_session_timeout=7000", "5000", "7000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, err := NewPool(t.Context(), tc.url)
			if err != nil {
				t.Fatalf("NewPool() error = %v", err)
			}
			defer pool.Close()
			params := pool.Config().ConnConfig.RuntimeParams
			if params["statement_timeout"] != tc.statement || params["idle_in_transaction_session_timeout"] != tc.idleInTxn {
				t.Errorf("RuntimeParams = %v, want statement_timeout %s and idle_in_transaction_session_timeout %s", params, tc.statement, tc.idleInTxn)
			}
		})
	}
}
