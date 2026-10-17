package dbtest

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// errQueryFault is the cause of a query QueryFaults failed.
var errQueryFault = errors.New("query failed by dbtest.QueryFaults")

// fault is one armed query: it lets skip runs through, then fails.
type fault struct {
	skip int
	once bool // disarm after the failure
}

// QueryFaults is a pgx tracer that fails sqlc queries by name. A failed query
// gets a context that is already canceled, so pgx fails it before sending
// anything: the connection and the transaction stay usable, and the code under
// test goes down the path of any database error (rpcerr reads it as Canceled).
//
// It wraps the tracer the test pool already has, so the guards of this package
// keep watching the pool. Build the service's pool with it as the tracer:
//
//	faults := dbtest.NewQueryFaults(cfg.Pool.Config().ConnConfig.Tracer)
//	pool, err := db.NewPoolWith(ctx, url, func(pc *pgxpool.Config) { pc.ConnConfig.Tracer = faults })
type QueryFaults struct {
	inner pgx.QueryTracer
	mu    sync.Mutex
	armed map[string]fault
	hits  map[string]int
}

// NewQueryFaults wraps inner (nil is allowed), with nothing armed.
func NewQueryFaults(inner pgx.QueryTracer) *QueryFaults {
	return &QueryFaults{inner: inner, armed: map[string]fault{}, hits: map[string]int{}}
}

// Arm fails every run of the queries named (the sqlc name, as in
// "-- name: GetCampaign :one") until Disarm.
func (f *QueryFaults) Arm(names ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, n := range names {
		f.armed[n] = fault{}
	}
}

// ArmOnce lets skip runs of the query through and fails the next one only.
func (f *QueryFaults) ArmOnce(name string, skip int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.armed[name] = fault{skip: skip, once: true}
}

// Disarm clears everything armed.
func (f *QueryFaults) Disarm() {
	f.mu.Lock()
	defer f.mu.Unlock()
	clear(f.armed)
}

// Hits is how many runs of the query failed.
func (f *QueryFaults) Hits(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[name]
}

// Pending lists what is still armed, so a test sees a fault that never fired.
func (f *QueryFaults) Pending() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for name := range f.armed {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// shouldFail counts the run of the query and reports whether it fails.
func (f *QueryFaults) shouldFail(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.armed[name]
	if !ok {
		return false
	}
	if a.skip > 0 {
		a.skip--
		f.armed[name] = a
		return false
	}
	if a.once {
		delete(f.armed, name)
	}
	f.hits[name]++
	return true
}

// TraceQueryStart implements pgx.QueryTracer.
func (f *QueryFaults) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if f.inner != nil {
		ctx = f.inner.TraceQueryStart(ctx, conn, data)
	}
	rest, ok := strings.CutPrefix(data.SQL, "-- name: ")
	if !ok {
		return ctx
	}
	name, _, _ := strings.Cut(rest, " ")
	if !f.shouldFail(name) {
		return ctx
	}
	failed, cancel := context.WithCancelCause(ctx)
	cancel(errQueryFault)
	return failed
}

// TraceQueryEnd implements pgx.QueryTracer.
func (f *QueryFaults) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if f.inner != nil {
		f.inner.TraceQueryEnd(ctx, conn, data)
	}
}

// TraceAcquireStart implements pgxpool.AcquireTracer by passing on.
func (f *QueryFaults) TraceAcquireStart(ctx context.Context, pool *pgxpool.Pool, data pgxpool.TraceAcquireStartData) context.Context {
	if a, ok := f.inner.(pgxpool.AcquireTracer); ok {
		return a.TraceAcquireStart(ctx, pool, data)
	}
	return ctx
}

// TraceAcquireEnd implements pgxpool.AcquireTracer by passing on.
func (f *QueryFaults) TraceAcquireEnd(ctx context.Context, pool *pgxpool.Pool, data pgxpool.TraceAcquireEndData) {
	if a, ok := f.inner.(pgxpool.AcquireTracer); ok {
		a.TraceAcquireEnd(ctx, pool, data)
	}
}
