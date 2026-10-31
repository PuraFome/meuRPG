package traffic

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

// clock is a time the test moves.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time          { return c.now }
func (c *clock) advance(d time.Duration) { c.now = c.now.Add(d) }

func TestAnInstanceThatNeverReceivedACallIsNeverIdle(t *testing.T) {
	t.Parallel()
	c := &clock{now: time.Unix(1_000_000, 0)}
	m := New(3*time.Minute, c.Now)
	c.advance(24 * time.Hour)
	if m.Idle() {
		t.Fatal("a cold instance is idle")
	}
}

func TestIdleOnceTheLimitPassedSinceTheLastCall(t *testing.T) {
	t.Parallel()
	c := &clock{now: time.Unix(1_000_000, 0)}
	m := New(3*time.Minute, c.Now)
	m.Note()
	for _, step := range []struct {
		advance time.Duration
		idle    bool
	}{
		{2*time.Minute + 59*time.Second, false},
		{time.Second, true}, // exactly the limit
		{time.Hour, true},
	} {
		c.advance(step.advance)
		if got := m.Idle(); got != step.idle {
			t.Fatalf("after %v more: Idle() = %v, want %v", step.advance, got, step.idle)
		}
	}
	m.Note() // traffic comes back
	if m.Idle() {
		t.Fatal("Idle() after a new call")
	}
}

// Only a unary call is traffic: a stream is what the idle instance holds.
func TestOnlyUnaryCallsCount(t *testing.T) {
	t.Parallel()
	c := &clock{now: time.Unix(1_000_000, 0)}
	m := New(time.Minute, c.Now)
	ic := m.Interceptor()

	stream := ic.WrapStreamingHandler(func(context.Context, connect.StreamingHandlerConn) error { return nil })
	_ = stream(t.Context(), nil)
	c.advance(time.Hour)
	if m.Idle() {
		t.Fatal("a stream counted as a call: the instance has not received a unary one")
	}

	called := false
	unary := ic.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return connect.NewResponse(&emptypb.Empty{}), nil
	})
	if _, err := unary(t.Context(), nil); err != nil || !called {
		t.Fatalf("the interceptor did not pass the call on: called=%v err=%v", called, err)
	}
	if m.Idle() {
		t.Fatal("Idle() right after a unary call")
	}
	c.advance(time.Minute)
	if !m.Idle() {
		t.Fatal("not idle a minute after the unary call")
	}
}
