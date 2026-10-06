package play

import (
	"testing"
	"time"
)

// gateClock drives puzzleGate without sleeping: timers fire, in order, when the
// test moves the clock past them.
type gateClock struct {
	t      time.Time
	timers []gateTimer
}

type gateTimer struct {
	at time.Time
	f  func()
}

func (c *gateClock) now() time.Time { return c.t }

func (c *gateClock) after(d time.Duration, f func()) {
	c.timers = append(c.timers, gateTimer{at: c.t.Add(d), f: f})
}

// advance moves the clock by d, firing every timer due on the way at its own time.
func (c *gateClock) advance(d time.Duration) {
	target := c.t.Add(d)
	for {
		next := -1
		for i, tm := range c.timers {
			if !tm.at.After(target) && (next < 0 || tm.at.Before(c.timers[next].at)) {
				next = i
			}
		}
		if next < 0 {
			break
		}
		tm := c.timers[next]
		c.timers = append(c.timers[:next], c.timers[next+1:]...)
		c.t = tm.at
		tm.f()
	}
	c.t = target
}

// The hint gate's contract (MR-038): the first change goes at once, the changes
// within the interval merge into one hint sent when it ends, never two hints of a
// puzzle closer than the interval, and the last change is never left unannounced.
// The clock is fake, so the test says the same on a fast machine and a loaded CI.
func TestPuzzleGate(t *testing.T) {
	const every = 250 * time.Millisecond
	c := &gateClock{t: time.Unix(1_000_000, 0)}
	g := &puzzleGate{now: c.now, after: c.after}
	sent := map[string][]time.Time{}
	fire := func(key string) { g.fire(key, every, func() { sent[key] = append(sent[key], c.t) }) }

	fire("p")
	if len(sent["p"]) != 1 {
		t.Fatalf("the first change sent %d hints, want 1 at once", len(sent["p"]))
	}
	for range 5 { // a burst inside the interval
		c.advance(20 * time.Millisecond)
		fire("p")
	}
	if len(sent["p"]) != 1 {
		t.Fatalf("a burst inside the interval sent %d hints, want it held", len(sent["p"]))
	}
	fire("q") // another puzzle never waits on this one
	if len(sent["q"]) != 1 {
		t.Errorf("another puzzle's first change sent %d hints, want 1 at once", len(sent["q"]))
	}
	c.advance(every)
	if got := sent["p"]; len(got) != 2 || got[1].Sub(got[0]) != every {
		t.Fatalf("after the interval: hints at %v, want the burst merged into one, %v after the first", got, every)
	}
	c.advance(time.Second)
	fire("p") // long after the last hint: at once
	if len(sent["p"]) != 3 {
		t.Fatalf("a change long after the last hint sent %d hints in all, want 3", len(sent["p"]))
	}

	// A steady stream, one change every 100 ms for 3 s.
	for range 30 {
		c.advance(100 * time.Millisecond)
		fire("p")
	}
	last := c.t
	c.advance(every)
	got := sent["p"]
	for i := 1; i < len(got); i++ {
		if gap := got[i].Sub(got[i-1]); gap < every {
			t.Errorf("hints %d and %d are %v apart, want at least %v", i-1, i, gap, every)
		}
	}
	if end := got[len(got)-1]; end.Before(last) {
		t.Errorf("the last hint went at %v, before the last change at %v: it was lost", end, last)
	}
	if n := len(got) - 3; n < 10 || n > 13 { // 3 s at one hint per 250 ms
		t.Errorf("the stream of 30 changes sent %d hints, want about 12", n)
	}
}
