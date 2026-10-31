// Package traffic tells whether this server instance still receives the app's
// unary calls.
//
// After a deploy the load balancer sends every new call to the new revision,
// while the pages opened before it keep the streams they hold on the old
// instance. The old instance's live hub never hears of what happens on the
// new one, so those pages see nothing new until they reload. An instance that
// has received unary calls and then stops receiving them is that old one: its
// streams end, and each app reconnects to the active revision.
//
// A visible page calls the server every 30 seconds (the session notice), so
// the active instance is never idle. An instance that never received a call (a
// cold start) is not idle either.
package traffic

import (
	"context"
	"sync"
	"time"

	"connectrpc.com/connect"
)

// Meter remembers when the last unary call arrived. Its zero value is not
// usable: call New.
type Meter struct {
	now   func() time.Time
	limit time.Duration

	mu   sync.Mutex
	seen bool
	last time.Time
}

// New returns a Meter that calls an instance idle once limit has passed since
// its last unary call. now is the clock (time.Now in the server).
func New(limit time.Duration, now func() time.Time) *Meter {
	return &Meter{now: now, limit: limit}
}

// Note records a unary call.
func (m *Meter) Note() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen, m.last = true, m.now()
}

// Idle reports whether the instance received unary calls and then went the
// whole limit without one.
func (m *Meter) Idle() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.seen && m.now().Sub(m.last) >= m.limit
}

// Interceptor returns a Connect interceptor that notes every unary call. A
// stream does not count: it is what the idle instance keeps open.
func (m *Meter) Interceptor() connect.Interceptor { return interceptor{m} }

type interceptor struct{ m *Meter }

func (i interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		i.m.Note()
		return next(ctx, req)
	}
}

func (interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}
