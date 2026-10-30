// Package tasks runs the server's detached work: what keeps going after the call that
// asked for it has answered (an image being made, an export being written, the sweep of
// what expired).
//
// On Cloud Run the CPU is billed per request, so it is throttled while no request is in
// flight, and a goroutine that outlives its call starves. In production each piece of
// detached work is therefore a Cloud Tasks task: Cloud Tasks calls the service back
// (POST /internal/tasks/<kind>, with an OIDC token) and Cloud Run gives the CPU for that
// request. Everywhere else (local runs, tests, CI's e2e, and whenever TASKS_* is not set)
// the same handler runs in a tracked goroutine of this process.
//
// Delivery is at least once and a payload holds IDs only, never free text or personal data,
// so every Handler is idempotent: it claims its work with a conditional update, and a
// duplicate, or work already done, returns nil and does nothing.
package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Kind names a kind of task. It is part of the task's URL (/internal/tasks/<kind>), so it
// is lowercase letters, digits and dashes.
type Kind string

// Handler does the work of one task. The answer decides what Cloud Tasks does next:
//   - nil: done (or already done, or nothing to do): the task is deleted.
//   - Permanent(err): a failure a retry cannot help; the handler has recorded it in its own
//     row the way failures are recorded, and the task is deleted (2xx).
//   - any other error: a retry can help: 5xx, and Cloud Tasks tries again under the queue's
//     retry settings.
type Handler func(ctx context.Context, payload json.RawMessage) error

// Options of a kind of task.
type Options struct {
	// Deadline is how long one delivery may run (the dispatch deadline of the task, and the
	// context's deadline of the handler). Cloud Run's request timeout is 35 minutes, the most.
	Deadline time.Duration
}

// Job is one task to enqueue.
type Job struct {
	Kind Kind
	// Payload is JSON with IDs only.
	Payload any
	// Name, when set, makes the task unique: a second Enqueue with the same name is
	// ErrExists (the task already exists or ran lately), which callers treat as success.
	Name string
	// At, when set and in the future, is when the task is first delivered.
	At time.Time
}

// ErrExists is returned by Enqueue for a named task that already exists.
var ErrExists = errors.New("tasks: the task already exists")

// Runner registers the handlers and enqueues the tasks. Both implementations (in-process and
// Cloud Tasks) satisfy it, and both run a handler the same way.
type Runner interface {
	// Register adds the handler of a kind. Call it before Start; a kind twice panics.
	Register(kind Kind, h Handler, opts Options)
	// Enqueue schedules a task and returns once it is accepted, not when it has run.
	Enqueue(ctx context.Context, job Job) error
	// Every makes h run on the kind every interval, from Start until ctx ends (in-process:
	// a ticker; Cloud Tasks: a self-scheduling chain of named tasks, see Sweeps in the
	// package documentation of the Cloud Tasks runner).
	Every(kind Kind, interval time.Duration, h Handler, opts Options)
	// Start begins the scheduled kinds. It does not block.
	Start(ctx context.Context)
	// Wait waits for the running handlers of this process, up to ctx: the shutdown (and a
	// test's cleanup) uses it, so nothing writes after the pool closes.
	Wait(ctx context.Context)
}

type permanent struct{ err error }

func (p permanent) Error() string { return p.err.Error() }
func (p permanent) Unwrap() error { return p.err }

// Permanent marks err as one a retry cannot help.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanent{err}
}

// IsPermanent says whether err was marked with Permanent.
func IsPermanent(err error) bool {
	_, ok := errors.AsType[permanent](err)
	return ok
}
