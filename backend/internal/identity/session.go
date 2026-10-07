package identity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/secret"
)

const (
	// SessionCookieName is the session cookie. The __Host- prefix makes the
	// browser accept it only with Secure, Path=/ and no Domain, so it is
	// bound to exactly this host: a sibling subdomain cannot set or
	// overwrite it (ADR-0002).
	SessionCookieName = "__Host-meurpg_session"

	// SessionLifetime is the absolute lifetime of a session. NIST SP
	// 800-63B-4, section 2.1.3 (AAL1): reauthentication SHALL be required at
	// least every 30 days. Using a session does not extend it.
	SessionLifetime = 30 * 24 * time.Hour

	// DefaultSessionIdleTimeout is how long a session may go unused before
	// it stops working (ASVS 5.0 V7.3.1). The table plays about once a week,
	// so 14 days forgives two missed sessions, and a laptop forgotten at a
	// friend's house stops working long before the 30 days are up. Set
	// SESSION_IDLE_TIMEOUT to change it (docs/operations.md).
	DefaultSessionIdleTimeout = 14 * 24 * time.Hour

	// sessionTouchEvery is how often a session's last use is written: at
	// most once per this interval, so a busy session is not a write per
	// request. The idle timeout is days; ten minutes of slack is nothing.
	sessionTouchEvery = 10 * time.Minute
)

// Session is a valid, unexpired sign-in session.
type Session struct {
	ID        string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
	// LastUsedAt is the last use the store recorded; it can be up to
	// sessionTouchEvery behind the real one.
	LastUsedAt time.Time
}

// errNoSession means the request carries no valid session: no cookie, a
// malformed one, an unknown or revoked token, or an expired session.
var errNoSession = errors.New("no valid session")

// startSession creates a session for userID, ending SessionLifetime after
// now, and returns its token, which goes only into the cookie.
func (s *Service) startSession(ctx context.Context, userID string, now, authTime time.Time) (string, Session, error) {
	// Session tokens and login states are secrets like invite tokens: 256
	// random bits, and only their SHA-256 goes to the database.
	token, hash := secret.New()
	session, err := s.store.createSession(ctx, NewSession{
		TokenHash: hash,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionLifetime),
		AuthTime:  authTime,
	})
	if err != nil {
		return "", Session{}, fmt.Errorf("create session: %w", err)
	}
	return token, session, nil
}

// lookupSession returns the session for a cookie value, or errNoSession.
// Other errors mean the store could not answer.
func (s *Service) lookupSession(ctx context.Context, token string) (Session, error) {
	hash, ok := secret.Hash(token)
	if !ok {
		return Session{}, errNoSession
	}
	now := s.now()
	session, err := s.store.lookupSession(ctx, hash, now, now.Add(-s.idleTimeout))
	if errors.Is(err, ErrNotFound) {
		return Session{}, errNoSession
	}
	return session, err
}

// touchSession records a use of the session, at most once per
// sessionTouchEvery. It is best effort: a failed write only means the next
// request tries again, so it never fails the request. It runs outside any
// other transaction (the interceptor calls it before the handler starts).
func (s *Service) touchSession(ctx context.Context, session Session) {
	now := s.now()
	if now.Sub(session.LastUsedAt) < sessionTouchEvery {
		return // the common case: no database call at all
	}
	if _, err := s.store.touchSession(ctx, session.ID, now, now.Add(-sessionTouchEvery)); err != nil {
		s.logger.WarnContext(ctx, "cannot record the session's last use", "error", err)
	}
}

// sessionCookie is the cookie that carries a new session's token. It
// expires with the session, so the browser drops it on its own.
func sessionCookie(token string, now, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt.UTC(),
		MaxAge:   int(expiresAt.Sub(now).Seconds()),
		Secure:   true,
		HttpOnly: true, // JavaScript cannot read it, so XSS cannot steal it
		// Lax: the browser sends it when the user follows a link to the
		// app, but not on cross-site POSTs or subrequests (CSRF).
		SameSite: http.SameSiteLaxMode,
	}
}

// expiredCookie tells the browser to delete the named cookie. The
// attributes must match the ones it was set with, or a __Host- cookie is
// rejected.
func expiredCookie(name string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // sent as Max-Age=0: delete now
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// cookieValue returns the named cookie from request headers. It goes
// through http.Request because that parser is lenient: a malformed
// unrelated cookie does not hide ours, and HTTP/2 split Cookie headers work.
func cookieValue(header http.Header, name string) (string, bool) {
	c, err := (&http.Request{Header: header}).Cookie(name)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}

// setNoStore marks a response as not cacheable anywhere. Every identity
// response is about one user, or carries a secret in a header.
func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}
