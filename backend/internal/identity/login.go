package identity

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/PuraFome/meuRPG/backend/internal/platform/ratelimit"
)

const (
	// loginCookieName ties a sign-in to the browser that started it: it
	// holds the same random state that goes to the provider. Without it,
	// an attacker could start a sign-in with their own account and trick a
	// victim's browser into finishing it ("login CSRF").
	loginCookieName = "__Host-meurpg_login"

	// loginStateLifetime is how long the user has to finish signing in at
	// the provider.
	loginStateLifetime = 10 * time.Minute

	// maxReturnToLength keeps return_to from bloating the login state row.
	maxReturnToLength = 1024
)

// loginRateLimit caps GET /auth/login, because every hit writes a login
// state row to the database. The limits are per server instance (in
// memory, see package ratelimit):
//
//   - per client IP: 20 at once, then one every 3 seconds (20 a minute).
//     That is plenty for a whole table of players behind one Wi-Fi, and
//     stops one client from filling the table.
//   - overall: 200 at once, then 2 a second (120 a minute), which bounds
//     the rows a botnet can write: at most 7,200 an hour per instance,
//     deleted by the row TTL within the next hour.
var loginRateLimit = ratelimit.Config{
	PerClient:  ratelimit.Rate{Burst: 20, Every: 3 * time.Second},
	Global:     ratelimit.Rate{Burst: 200, Every: 500 * time.Millisecond},
	MaxClients: 10_000,
}

// handleLogin starts a sign-in: GET /auth/login?return_to=/path.
//
// It stores a login state (PKCE verifier, nonce, return_to) under the hash
// of a random state, puts the state in a short-lived cookie, and redirects
// the browser to the provider.
func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	setAuthHeaders(w)
	ctx := r.Context()

	// The limit comes before anything else, so a refused request costs
	// nothing. The client IP is not logged (docs/privacidade.md); the
	// request log line already shows the 429.
	if ok, wait := s.loginLimiter.Allow(ratelimit.ClientKey(r, s.behindCloudRun)); !ok {
		// Retry-After is in whole seconds (RFC 9110, section 10.2.3).
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(wait.Seconds())))))
		http.Error(w, "too many sign-in attempts, please wait a moment and try again", http.StatusTooManyRequests)
		return
	}

	returnTo, ok := safeReturnTo(r.URL.Query().Get("return_to"))
	if !ok {
		s.logger.WarnContext(ctx, "sign-in rejected", "reason", "unsafe_return_to")
		http.Error(w, "return_to must be a path on this site, like /campanhas", http.StatusBadRequest)
		return
	}

	p, err := s.providers.get(ctx)
	if err != nil {
		s.logger.WarnContext(ctx, "sign-in unavailable: OIDC discovery failed", "error", err)
		http.Error(w, "sign-in is temporarily unavailable, please try again later", http.StatusServiceUnavailable)
		return
	}

	state, stateHash := newSecret()
	nonce, _ := newSecret()
	verifier := oauth2.GenerateVerifier()

	now := s.now()
	err = s.store.SaveLoginState(ctx, LoginState{
		StateHash:    stateHash,
		CodeVerifier: verifier,
		Nonce:        nonce,
		ReturnTo:     returnTo,
		CreatedAt:    now,
		ExpiresAt:    now.Add(loginStateLifetime),
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "sign-in unavailable: cannot save the login state", "error", err)
		http.Error(w, "sign-in is temporarily unavailable, please try again later", http.StatusServiceUnavailable)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     loginCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   int(loginStateLifetime.Seconds()),
		Secure:   true,
		HttpOnly: true,
		// Lax is what lets the cookie come back on the provider's
		// top-level redirect to /auth/callback. Strict would drop it.
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, p.authCodeURL(state, nonce, verifier), http.StatusFound)
}

// loginError is a failed callback: the HTTP status for the browser, and a
// short reason for the logs. Neither ever includes a token, code, cookie,
// state or e-mail.
type loginError struct {
	status int
	reason string
	err    error
}

func (e *loginError) Error() string {
	if e.err == nil {
		return e.reason
	}
	return e.reason + ": " + e.err.Error()
}

func (e *loginError) Unwrap() error { return e.err }

// badLogin is a failure caused by the request or the provider's answer.
func badLogin(reason string, err error) *loginError {
	return &loginError{status: http.StatusBadRequest, reason: reason, err: err}
}

// unavailable is a failure on our side (database, provider unreachable).
func unavailable(reason string, err error) *loginError {
	return &loginError{status: http.StatusServiceUnavailable, reason: reason, err: err}
}

// handleCallback finishes a sign-in: GET /auth/callback?code=...&state=...
//
// It is a GET that creates a session, so http.CrossOriginProtection lets it
// through; state (checked against the cookie and the database), PKCE and
// nonce are what protect it.
func (s *Service) handleCallback(w http.ResponseWriter, r *http.Request) {
	setAuthHeaders(w)
	// The login cookie is single use, whatever happens next.
	http.SetCookie(w, expiredCookie(loginCookieName))

	returnTo, token, session, err := s.completeLogin(r)
	if err != nil {
		le, ok := errors.AsType[*loginError](err)
		if !ok {
			le = unavailable("internal", err)
		}
		attrs := []any{"reason", le.reason}
		if le.err != nil {
			attrs = append(attrs, "error", le.err)
		}
		level := slog.LevelWarn
		if le.status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		s.logger.Log(r.Context(), level, "sign-in failed", attrs...)

		message := "sign-in failed, please try again"
		if le.status >= http.StatusInternalServerError {
			message = "sign-in is temporarily unavailable, please try again later"
		}
		http.Error(w, message, le.status)
		return
	}

	http.SetCookie(w, sessionCookie(token, session.CreatedAt, session.ExpiresAt))
	s.logger.InfoContext(r.Context(), "sign-in succeeded")

	// returnTo passed safeReturnTo in handleLogin before it was stored;
	// checking again costs nothing and keeps this redirect safe on its own.
	safe, ok := safeReturnTo(returnTo)
	if !ok {
		safe = "/"
	}
	// 303: the browser follows with a GET, whatever brought it here.
	http.Redirect(w, r, safe, http.StatusSeeOther) //nolint:gosec // G710: safe is a path on this site (safeReturnTo)
}

// completeLogin runs every check of the callback, in order, and starts the
// session. It returns where to send the browser and the new session.
func (s *Service) completeLogin(r *http.Request) (returnTo, token string, session Session, err error) {
	ctx := r.Context()
	query := r.URL.Query()

	// 1. The state in the URL must be the one in this browser's cookie.
	// This check comes before any database access, so a stranger who
	// learned a state (the URL goes to the platform's request logs) cannot
	// burn it for the real user.
	state := query.Get("state")
	cookieState, _ := cookieValue(r.Header, loginCookieName)
	if state == "" || cookieState == "" {
		return "", "", Session{}, badLogin("missing_state", nil)
	}
	if subtle.ConstantTimeCompare([]byte(state), []byte(cookieState)) != 1 {
		return "", "", Session{}, badLogin("state_mismatch", nil)
	}
	stateHash, ok := hashSecret(state)
	if !ok {
		return "", "", Session{}, badLogin("malformed_state", nil)
	}

	// 2. The state must exist and be fresh. Taking it deletes it, so a
	// callback URL works once.
	now := s.now()
	login, err := s.store.TakeLoginState(ctx, stateHash, now)
	if errors.Is(err, ErrNotFound) {
		return "", "", Session{}, badLogin("unknown_or_expired_state", nil)
	}
	if err != nil {
		return "", "", Session{}, unavailable("store_error", err)
	}

	// 3. The provider may report an error instead of a code, e.g. when the
	// user cancels. Its error_description is not logged or shown: it is
	// free text from the query string.
	if providerErr := query.Get("error"); providerErr != "" {
		return "", "", Session{}, badLogin("provider_error", errors.New(oauthErrorCode(providerErr)))
	}
	code := query.Get("code")
	if code == "" {
		return "", "", Session{}, badLogin("missing_code", nil)
	}

	p, err := s.providers.get(ctx)
	if err != nil {
		return "", "", Session{}, unavailable("discovery_failed", err)
	}

	// 4. Trade the code for tokens, proving with the PKCE verifier that we
	// started this flow.
	rawIDToken, err := p.exchange(ctx, code, login.CodeVerifier)
	if err != nil {
		if re, ok := errors.AsType[*oauth2.RetrieveError](err); ok {
			// Log only the OAuth error code, not the response body.
			return "", "", Session{}, badLogin("token_exchange_rejected", errors.New(oauthErrorCode(re.ErrorCode)))
		}
		return "", "", Session{}, &loginError{status: http.StatusBadGateway, reason: "token_exchange_failed", err: err}
	}

	// 5. Verify the ID token: signature, iss, aud, exp, nonce, azp, sub.
	id, err := p.verify(ctx, rawIDToken, login.Nonce)
	if err != nil {
		return "", "", Session{}, badLogin("invalid_id_token", err)
	}

	userID, err := s.store.UpsertUser(ctx, ExternalIdentity{Issuer: id.Issuer, Subject: id.Subject, Email: id.Email})
	if err != nil {
		return "", "", Session{}, unavailable("store_error", err)
	}

	// A browser that signs in again gets a new session (never the old
	// token, which rules out session fixation), and the old one is revoked
	// instead of lingering until it expires.
	if old, ok := cookieValue(r.Header, SessionCookieName); ok {
		s.revokeQuietly(ctx, old)
	}

	// The 30 days count from now, whatever auth_time says: it is recorded,
	// not trusted (see verifiedIdentity).
	token, session, err = s.startSession(ctx, userID, now, id.AuthTime)
	if err != nil {
		return "", "", Session{}, unavailable("store_error", err)
	}
	return login.ReturnTo, token, session, nil
}

// revokeQuietly revokes the session behind a cookie value, if there is one.
// Failing here must not fail the new sign-in, so errors are only logged.
func (s *Service) revokeQuietly(ctx context.Context, token string) {
	old, err := s.lookupSession(ctx, token)
	if err != nil {
		if !errors.Is(err, errNoSession) {
			s.logger.WarnContext(ctx, "cannot look up the previous session", "error", err)
		}
		return
	}
	if err := s.store.RevokeSession(ctx, old.ID); err != nil {
		s.logger.WarnContext(ctx, "cannot revoke the previous session", "error", err)
	}
}

// oauthErrorPattern matches OAuth error codes (RFC 6749, section 4.1.2.1),
// which are safe to log. Anything else is replaced.
var oauthErrorPattern = regexp.MustCompile(`^[a-z_]{1,64}$`)

func oauthErrorCode(code string) string {
	if oauthErrorPattern.MatchString(code) {
		return code
	}
	return "unrecognized_error_code"
}

// safeReturnTo accepts only a path on this site, so /auth/login cannot be
// used as an open redirect to another site. Empty means the home page.
//
// Browsers are lenient with URLs, so the checks are strict: "//host" and
// "/\host" are other sites to a browser, and tabs or newlines are dropped
// before parsing ("/\t/host" becomes "//host").
func safeReturnTo(raw string) (string, bool) {
	if raw == "" {
		return "/", true
	}
	if len(raw) > maxReturnToLength || raw[0] != '/' || strings.HasPrefix(raw, "//") {
		return "", false
	}
	for _, c := range raw {
		if c == '\\' || c < 0x20 || c == 0x7f {
			return "", false
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil || u.Opaque != "" {
		return "", false
	}
	return raw, true
}

// setAuthHeaders sets the headers every /auth/* response needs. The
// callback URL carries the authorization code and state in its query, and
// no-referrer keeps them out of the Referer header of whatever comes next.
func setAuthHeaders(w http.ResponseWriter) {
	setNoStore(w)
	w.Header().Set("Referrer-Policy", "no-referrer")
}
