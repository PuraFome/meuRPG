package ratelimit

import "time"

// Policy is the set of limiters that protect the server from one client
// doing too much (docs/architecture.md#abuse-limits). All of them live
// in this instance's memory: with a second instance every limit would be
// per instance, and a shared store (Redis, or the database) would be needed
// to keep them exact. The server runs one instance (max-instances 1), so
// the limits are exact today.
//
// The numbers are far above what a table does: a master, six players, a map
// editor painting and a session page reading. They only stop a script. The
// multiplier scales every one of them (RATE_LIMIT_MULTIPLIER): the e2e suite
// runs many accounts' requests from one IP and raises it.
type Policy struct {
	// IP limits every API request (Connect RPCs, images, uploads) by client
	// IP, before the session is looked up: a request with no session, or with
	// a made-up cookie, still costs a database read.
	IP *Limiter
	// RPC limits the signed-in user's Connect calls and stream opens.
	RPC *Limiter
	// Download limits a user's image, thumbnail and fog-tile downloads (the
	// tiles have their own, stricter limit on cache misses: maps/tiles.go).
	Download *Limiter
	// Upload limits a user's image uploads: each one decodes up to 40
	// megapixels.
	Upload *Limiter
	// Package limits a user's calls that read or write a whole campaign (start
	// an export, begin, preview and create an import, download an export), and
	// PackageParts the parts of an upload.
	Package, PackageParts *Limiter
}

// NewPolicy builds the limiters. multiplier is 1 in production; it must be
// positive.
func NewPolicy(multiplier float64) Policy {
	return Policy{
		// A table behind one NAT (a master and six players, each with a few
		// requests a second at a busy moment) sits below 100 a second.
		IP: New(scaled(Config{
			PerClient:  Rate{Burst: 600, Every: 10 * time.Millisecond}, // 100/s
			Global:     Rate{Burst: 2000, Every: 2 * time.Millisecond}, // 500/s
			MaxClients: 10000,
		}, multiplier)),
		RPC: New(scaled(Config{
			PerClient:  Rate{Burst: 200, Every: 25 * time.Millisecond}, // 40/s
			Global:     Rate{Burst: 2000, Every: 2 * time.Millisecond},
			MaxClients: 10000,
		}, multiplier)),
		// The gallery lists up to 300 thumbnails at once, and a fog map opens
		// with a few dozen tiles.
		Download: New(scaled(Config{
			PerClient:  Rate{Burst: 500, Every: 20 * time.Millisecond}, // 50/s
			Global:     Rate{Burst: 2000, Every: 2 * time.Millisecond},
			MaxClients: 10000,
		}, multiplier)),
		// A master sends a handful of images in a row; ten a minute is a lot.
		Upload: New(scaled(Config{
			PerClient:  Rate{Burst: 20, Every: 6 * time.Second},
			Global:     Rate{Burst: 50, Every: time.Second},
			MaxClients: 10000,
		}, multiplier)),
		// A master exports or imports a campaign a few times in a session, and each call
		// reads or writes the whole campaign: six in a minute is plenty.
		Package: New(scaled(Config{
			PerClient:  Rate{Burst: 6, Every: 10 * time.Second}, //nolint:mnd // the policy\'s numbers
			Global:     Rate{Burst: 20, Every: 5 * time.Second}, //nolint:mnd // the policy\'s numbers
			MaxClients: 10000,                                   //nolint:mnd // as the other limiters
		}, multiplier)),
		// 200 MiB goes up in about forty parts of 5 MiB; a fast line sends them in seconds.
		PackageParts: New(scaled(Config{
			PerClient:  Rate{Burst: 60, Every: 500 * time.Millisecond},  //nolint:mnd // the policy\'s numbers
			Global:     Rate{Burst: 200, Every: 100 * time.Millisecond}, //nolint:mnd // the policy\'s numbers
			MaxClients: 10000,                                           //nolint:mnd // as the other limiters
		}, multiplier)),
	}
}

// scaled makes every rate in c m times as generous: m times the burst, and
// the same count of events in 1/m of the time.
func scaled(c Config, m float64) Config {
	if m <= 0 {
		m = 1
	}
	for _, r := range []*Rate{&c.PerClient, &c.Global} {
		r.Burst = max(1, int(float64(r.Burst)*m))
		r.Every = max(time.Nanosecond, time.Duration(float64(r.Every)/m))
	}
	return c
}
