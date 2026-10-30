package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func newInProcess(t *testing.T) *InProcess {
	t.Helper()
	p := NewInProcess(slog.New(slog.DiscardHandler))
	t.Cleanup(p.Cancel)
	return p
}

func TestInProcessRunsATrackedGoroutine(t *testing.T) {
	t.Parallel()
	p := newInProcess(t)
	release := make(chan struct{})
	var got atomic.Value
	p.Register("work", func(_ context.Context, payload json.RawMessage) error {
		<-release
		got.Store(string(payload))
		return nil
	}, Options{})
	if err := p.Enqueue(t.Context(), Job{Kind: "work", Payload: map[string]string{"id": "x"}}); err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	begin := time.Now()
	p.Wait(short)
	if time.Since(begin) < 25*time.Millisecond {
		t.Error("Wait returned while the task ran")
	}
	close(release)
	p.Wait(t.Context())
	if got.Load() != `{"id":"x"}` {
		t.Errorf("payload = %v", got.Load())
	}
}

func TestInProcessNamedTasksAreUniqueAndUnknownKindsFail(t *testing.T) {
	t.Parallel()
	p := newInProcess(t)
	p.Register("work", noop, Options{})
	job := Job{Kind: "work", Payload: struct{}{}, Name: "once"}
	if err := p.Enqueue(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	if err := p.Enqueue(t.Context(), job); !errors.Is(err, ErrExists) {
		t.Errorf("second = %v, want ErrExists", err)
	}
	if err := p.Enqueue(t.Context(), Job{Kind: "nope", Payload: struct{}{}}); err == nil {
		t.Error("unknown kind accepted")
	}
	p.Wait(t.Context())
}

func TestInProcessEveryRunsAtStartAndAgain(t *testing.T) {
	t.Parallel()
	p := newInProcess(t)
	var runs atomic.Int32
	p.Every("sweep", 20*time.Millisecond, func(context.Context, json.RawMessage) error { runs.Add(1); return nil }, Options{})
	ctx, cancel := context.WithCancel(t.Context())
	p.Start(ctx)
	p.Start(ctx) // twice is once
	deadline := time.Now().Add(5 * time.Second)
	for runs.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if runs.Load() < 3 {
		t.Fatalf("runs = %d, want 3 or more", runs.Load())
	}
	cancel()
	p.Wait(t.Context())
}

func TestInProcessDelaysAndPanicsAreContained(t *testing.T) {
	t.Parallel()
	p := newInProcess(t)
	var ranAt atomic.Int64
	p.Register("later", func(context.Context, json.RawMessage) error { ranAt.Store(time.Now().UnixNano()); return nil }, Options{})
	p.Register("boom", func(context.Context, json.RawMessage) error { panic("boom") }, Options{})
	due := time.Now().Add(60 * time.Millisecond)
	if err := p.Enqueue(t.Context(), Job{Kind: "later", Payload: struct{}{}, At: due}); err != nil {
		t.Fatal(err)
	}
	if err := p.Enqueue(t.Context(), Job{Kind: "boom", Payload: struct{}{}}); err != nil {
		t.Fatal(err)
	}
	p.Wait(t.Context())
	if ranAt.Load() < due.UnixNano() {
		t.Error("a task ran before its time")
	}
}

func TestPermanent(t *testing.T) {
	t.Parallel()
	base := errors.New("x")
	if !IsPermanent(Permanent(base)) || !errors.Is(Permanent(base), base) || IsPermanent(base) || Permanent(nil) != nil {
		t.Error("Permanent / IsPermanent")
	}
}
