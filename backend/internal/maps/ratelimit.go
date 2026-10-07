package maps

import (
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"

	"github.com/PuraFome/meuRPG/backend/internal/platform/ratelimit"
)

// The per-user limits of the image routes (platform/ratelimit.Policy): the
// downloads (images, thumbnails, fog tiles) and the uploads. The fog tiles
// have a second, stricter limit on the tiles that must be rendered
// (tiles.go): this one counts every request, cache hit or not, because each
// costs three reads of the database to check the session and the membership.

// routeLimit is one limiter and the notifier that logs its refusals.
type routeLimit struct {
	limiter *ratelimit.Limiter
	notify  *ratelimit.Notifier
}

type routeLimits struct{ download, upload *routeLimit }

func newRouteLimits(download, upload *ratelimit.Limiter, logger *slog.Logger) routeLimits {
	wrap := func(l *ratelimit.Limiter, name string) *routeLimit {
		if l == nil {
			return nil
		}
		return &routeLimit{limiter: l, notify: ratelimit.NewNotifier(logger, name)}
	}
	return routeLimits{download: wrap(download, "image downloads"), upload: wrap(upload, "image uploads")}
}

// limitUser applies limit to the signed-in user. It runs after withSession.
// A caller with no session passes: the IP limit already covered it, and the
// handler refuses it.
func (s *Service) limitUser(sessions Sessions, limit *routeLimit, next http.Handler) http.Handler {
	if limit == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := sessions.UserID(r.Context())
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		if ok, wait := limit.limiter.Allow(userID); !ok {
			limit.notify.Hit(r.Context()) // the context carries the user's ID on the line
			s.writeError(w, r, errRateLimited(wait))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// errRateLimited is the 429 for a user over a route's limit. Its reason,
// RATE_LIMITED, tells the app it is not the full gallery (QUOTA).
func errRateLimited(wait time.Duration) *httpError {
	return &httpError{
		status: http.StatusTooManyRequests, code: connect.CodeResourceExhausted, reason: "RATE_LIMITED",
		message: "too many requests, please wait a moment and try again", retryAfter: ratelimit.RetryAfterSeconds(wait),
	}
}
