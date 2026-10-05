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
