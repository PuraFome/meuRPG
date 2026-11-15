package dbtest

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkCopyOfTheTemplate times the making of a test database from the template: the cost every database test pays once per
// pool. Each iteration takes a database no earlier one gave back, so none is reused. Run it against the server the tests use:
//
//	cd backend && MEURPG_TEST_DATABASE_URL=... go test ./internal/platform/dbtest -run '^$' -bench CopyOfTheTemplate -benchtime 5x
func BenchmarkCopyOfTheTemplate(b *testing.B) {
	NewPool(b, "meurpg_bench_warm") // the template is built (once) before the clock starts
	b.ResetTimer()
	for i := range b.N {
		start := time.Now()
		NewPool(b, fmt.Sprintf("meurpg_bench_%d", i))
		b.ReportMetric(time.Since(start).Seconds(), "s/database")
	}
}
