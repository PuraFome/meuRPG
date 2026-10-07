package ratelimit

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Notifier logs that a limit was hit, at WARN, but at most once per
// interval: a script hitting a limit thousands of times a second must not
// fill the log (and the bill). The line carries what the request's context
// already carries (request id, and user id for a signed-in user), plus the
// limiter's name and how many hits the interval swallowed. It never carries
// the client's IP address (docs/privacy.md).
type Notifier struct {
	logger   *slog.Logger
	name     string
	interval time.Duration
	now      func() time.Time

	mu         sync.Mutex
	last       time.Time
	suppressed int
}

// NewNotifier returns a Notifier for the limiter called name. A nil logger
// means slog.Default().
func NewNotifier(logger *slog.Logger, name string) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{logger: logger, name: name, interval: 10 * time.Second, now: time.Now}
}

// Hit records that a request was refused.
func (n *Notifier) Hit(ctx context.Context) {
	n.mu.Lock()
	now := n.now()
	if !n.last.IsZero() && now.Sub(n.last) < n.interval {
		n.suppressed++
		n.mu.Unlock()
		return
	}
	suppressed := n.suppressed
	n.last, n.suppressed = now, 0
	n.mu.Unlock()
	n.logger.WarnContext(ctx, "rate limit hit",
		slog.String("limiter", n.name), slog.Int("suppressed_since_last_line", suppressed))
}
