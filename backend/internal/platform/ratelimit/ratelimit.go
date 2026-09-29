// Package ratelimit limits how often something can happen, per client and
// overall, in memory.
//
// The limits live in each server instance's memory, so with N instances on
// Cloud Run the effective limits are up to N times larger. That is fine for
// what they protect against (one client, or a small botnet, filling the
// database), and it keeps the database out of the hot path of an attack.
//
// The client keys (IP addresses) are personal data. They stay only in
// memory, and are never logged. A client that stops sending requests is
// forgotten once its bucket is full again, within two refill periods
// (about two minutes for the sign-in limit), even if no other request ever
// comes (see docs/privacidade.md).
package ratelimit

import (
	"math"
	"sync"
	"time"
)

// Rate is a token bucket: it allows Burst events at once, then one more
// every Every.
type Rate struct {
	Burst int
	Every time.Duration
}

// Config configures a Limiter.
type Config struct {
	// PerClient limits each client key.
	PerClient Rate
	// Global limits all clients together.
	Global Rate
	// MaxClients caps how many client keys are tracked at once, so that a
	// flood of distinct addresses cannot exhaust memory. When the table is
	// full even after dropping idle clients, new clients are refused.
	MaxClients int
	// Now returns the current time. Nil means time.Now.
	Now func() time.Time
}

// Limiter enforces a per-client limit and a global one. It is safe for
// concurrent use.
type Limiter struct {
	cfg Config
	now func() time.Time

	mu        sync.Mutex
	global    bucket
	clients   map[string]*bucket
	lastSweep time.Time
	// sweepScheduled is true while a timer will sweep the clients; it
	// stops rescheduling itself once none is left, so an idle Limiter
	// holds no timer.
	sweepScheduled bool
}

// New returns a Limiter. It panics on a Rate with no Burst or no Every, a
// programming error.
func New(cfg Config) *Limiter {
	for _, r := range []Rate{cfg.PerClient, cfg.Global} {
		if r.Burst < 1 || r.Every <= 0 {
			panic("ratelimit: every Rate needs Burst >= 1 and Every > 0")
		}
	}
	l := &Limiter{cfg: cfg, now: cfg.Now, clients: map[string]*bucket{}}
	if l.now == nil {
		l.now = time.Now
	}
	now := l.now()
	l.global = bucket{tokens: float64(cfg.Global.Burst), last: now}
	l.lastSweep = now
	return l
}

// Allow reports whether one more event from the client with this key may
// happen now. When it may not, retryAfter says how long to wait.
//
// The client's own limit is checked first, so a single client that is over
// its limit never uses up the global budget of everybody else.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastSweep) >= l.idleAfter() {
		l.sweep(now)
	}

	client, found := l.clients[key]
	if !found {
		if len(l.clients) >= l.cfg.MaxClients {
			l.sweep(now)
		}
		if len(l.clients) >= l.cfg.MaxClients {
			return false, l.idleAfter()
		}
		client = &bucket{tokens: float64(l.cfg.PerClient.Burst), last: now}
		l.clients[key] = client
		if !l.sweepScheduled {
			l.sweepScheduled = true
			time.AfterFunc(l.idleAfter(), l.scheduledSweep)
		}
	}

	if ok, wait := client.take(now, l.cfg.PerClient); !ok {
		return false, wait
	}
	if ok, wait := l.global.take(now, l.cfg.Global); !ok {
		client.tokens++ // refund: the global limit refused, not this client's
		return false, wait
	}
	return true, 0
}

// idleAfter is how long a client needs to refill its bucket completely.
// After that, forgetting it changes nothing.
func (l *Limiter) idleAfter() time.Duration {
	return time.Duration(l.cfg.PerClient.Burst) * l.cfg.PerClient.Every
}

// scheduledSweep runs on a timer, so idle clients are forgotten even when
// no request comes to trigger a sweep.
func (l *Limiter) scheduledSweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(l.now())
	if len(l.clients) > 0 {
		time.AfterFunc(l.idleAfter(), l.scheduledSweep)
	} else {
		l.sweepScheduled = false
	}
}

// sweep forgets every client whose bucket is full again: forgetting it
// changes nothing, and keeps client keys in memory for minutes at most.
func (l *Limiter) sweep(now time.Time) {
	for key, b := range l.clients {
		if b.refill(now, l.cfg.PerClient) >= float64(l.cfg.PerClient.Burst) {
			delete(l.clients, key)
		}
	}
	l.lastSweep = now
}

// bucket is a token bucket. tokens is a float so the refill is smooth.
type bucket struct {
	tokens float64
	last   time.Time
}

// refill adds the tokens earned since the last call, up to the burst, and
// returns the current count.
func (b *bucket) refill(now time.Time, r Rate) float64 {
	if elapsed := now.Sub(b.last); elapsed > 0 {
		b.tokens = math.Min(float64(r.Burst), b.tokens+float64(elapsed)/float64(r.Every))
		b.last = now
	}
	return b.tokens
}

// take spends one token, or says how long until one is available.
func (b *bucket) take(now time.Time, r Rate) (bool, time.Duration) {
	if b.refill(now, r) >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) * float64(r.Every))
}
