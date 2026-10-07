// Package oidctest is a small OpenID Connect provider for tests and local
// development. It is not an identity provider for real people: anyone who
// reaches it can sign in as any of its test users, without a password.
//
// Two programs use it:
//
//   - the identity package's tests, in-process over httptest, with
//     AutoSignIn and the knobs in Knobs, which break one thing at a time
//     (the claims, the signing key, the discovery document);
//   - cmd/devidp, the provider behind the local stack and the Playwright
//     tests, which shows a login page listing the test users.
//
// It implements what MeuRPG's sign-in needs from a provider, strictly: the
// Authorization Code flow for one confidential client, PKCE with S256
// required (RFC 7636), state and nonce required, and RS256 ID tokens signed
// with a key generated at startup and published in the JWKS. max_age and
// prompt=login|none work against a browser session kept in a cookie, and
// every ID token carries auth_time (OpenID Connect Core 1.0, section 2).
package oidctest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

const (
	// codeLifetime is how long an authorization code can be exchanged.
	// RFC 6749, section 4.1.2, recommends at most 10 minutes; a test
	// client exchanges it within milliseconds.
	codeLifetime = time.Minute

	// idTokenLifetime is how long an ID token is valid.
	idTokenLifetime = time.Hour

	// sessionCookieName holds the provider's own browser session, which is
	// what max_age and prompt are measured against. It has nothing to do
	// with MeuRPG's session.
	sessionCookieName = "devidp_session"
)

// User is a test user the provider can sign in.
type User struct {
	// Subject is the sub claim: stable, so the user keeps the same MeuRPG
	// account across provider restarts.
	Subject string
	// Name is shown on the login page and sent as the name claim, which
	// MeuRPG never asks for nor stores.
	Name string
	// Email is sent as the email claim when not empty.
	Email string
	// EmailVerified is the email_verified claim: a bool, a string ("true"
	// or anything else, as some providers send) or nil to leave it out.
	EmailVerified any
}

// TestUsers returns the users cmd/devidp offers on its login page. Every
// call returns a fresh copy.
func TestUsers() []User {
	return []User{
		{Subject: "devidp-mestre", Name: "Mestre Teste", Email: "mestre@example.com", EmailVerified: true},
		{Subject: "devidp-jogador", Name: "Jogador Teste", Email: "jogador@example.com", EmailVerified: true},
		{Subject: "devidp-nao-verificado", Name: "E-mail Não Verificado", Email: "nao-verificado@example.com", EmailVerified: false},
		// Only the "sign out of other devices" test (e2e/tests/sessions.spec.ts)
		// uses this one: that action ends every other session of the user, so
		// it must never be an account other tests share.
		{Subject: "devidp-sessoes", Name: "Sessões Teste", Email: "sessoes@example.com", EmailVerified: true},
	}
}

// Config configures a Provider.
type Config struct {
	// Issuer is the provider's issuer URL, e.g. http://idp.localhost:9090.
	// Every endpoint lives under it.
	Issuer string

	// ClientID and ClientSecret are the only client's credentials.
	ClientID     string
	ClientSecret string

	// RedirectURIs are the registered redirect URIs. An authorization
	// request must use one of them exactly.
	RedirectURIs []string

	// Users are the users who can sign in. There must be at least one.
	Users []User

	// AutoSignIn skips the login page: the authorization endpoint signs
	// Users[0] in at once and redirects back. Tests use it, since they have
	// no browser to click with.
	AutoSignIn bool

	// Now returns the current time. Nil means time.Now.
	Now func() time.Time

	// Logger receives one line per sign-in. Nil means no logging.
	Logger *slog.Logger
}

// Knobs make the provider misbehave on purpose, one thing at a time. Tests
// change them with Provider.Tweak; cmd/devidp never does.
type Knobs struct {
	// Metadata overrides fields of the discovery document; a nil value
	// removes the field.
	Metadata map[string]any
	// MutateClaims, when set, edits the ID token claims before signing.
	MutateClaims func(claims map[string]any)
	// SigningKey, when set, signs ID tokens instead of the published key.
	SigningKey *rsa.PrivateKey
	// Unsigned makes the token endpoint return an "alg":"none" ID token.
	Unsigned bool
}

// Provider is an OpenID Connect provider. It is an http.Handler; serve it
// at the issuer URL.
type Provider struct {
	cfg      Config
	now      func() time.Time
	logger   *slog.Logger
	base     string // the issuer without a trailing slash, for endpoint URLs
	basePath string // the issuer's path, without a trailing slash
	key      *rsa.PrivateKey
	keyID    string
	handler  http.Handler

	mu            sync.Mutex
	users         []User
	knobs         Knobs
	codes         map[string]authCode
	sessions      map[string]browserSession
	lastAuthorize url.Values
}

// authRequest is a validated authorization request.
type authRequest struct {
	redirectURI string
	scope       string
	state       string
	nonce       string
	challenge   string
	maxAge      time.Duration // negative when max_age was not sent
	prompt      []string
}

// authCode is an issued authorization code, waiting for its exchange.
type authCode struct {
	request   authRequest
	user      User
	authTime  time.Time
	expiresAt time.Time
}

// browserSession is who is signed in at the provider in one browser.
type browserSession struct {
	subject  string
	authTime time.Time
}

// New returns a Provider with a fresh 2048-bit RSA signing key.
func New(cfg Config) (*Provider, error) {
	issuer, err := url.Parse(cfg.Issuer)
	switch {
	case err != nil || issuer.Host == "" || (issuer.Scheme != "http" && issuer.Scheme != "https"):
		return nil, fmt.Errorf("oidctest: the issuer must be an absolute http(s) URL, got %q", cfg.Issuer)
	case issuer.RawQuery != "" || issuer.Fragment != "" || issuer.User != nil:
		return nil, fmt.Errorf("oidctest: the issuer must not have a query, fragment or user info, got %q", cfg.Issuer)
	case cfg.ClientID == "" || cfg.ClientSecret == "":
		return nil, errors.New("oidctest: a client ID and a client secret are required")
	case len(cfg.RedirectURIs) == 0:
		return nil, errors.New("oidctest: at least one redirect URI is required")
	case len(cfg.Users) == 0:
		return nil, errors.New("oidctest: at least one user is required")
	}
	for _, uri := range cfg.RedirectURIs {
		if u, err := url.Parse(uri); err != nil || !u.IsAbs() || u.Fragment != "" {
			return nil, fmt.Errorf("oidctest: redirect URI %q must be an absolute URL without a fragment", uri)
		}
	}
	seen := map[string]bool{}
	for _, u := range cfg.Users {
		if u.Subject == "" || seen[u.Subject] {
			return nil, fmt.Errorf("oidctest: every user needs a unique, non-empty subject (got %q)", u.Subject)
		}
		seen[u.Subject] = true
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("oidctest: generate the signing key: %w", err)
	}
	// The key ID is the key's RFC 7638 thumbprint, so it changes with the
	// key and a client never mixes two keys up.
	thumbprint, err := (&jose.JSONWebKey{Key: &key.PublicKey}).Thumbprint(crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("oidctest: key thumbprint: %w", err)
	}

	p := &Provider{
		cfg:      cfg,
		now:      cfg.Now,
		logger:   cfg.Logger,
		base:     strings.TrimSuffix(cfg.Issuer, "/"),
		basePath: strings.TrimSuffix(issuer.Path, "/"),
		key:      key,
		keyID:    base64.RawURLEncoding.EncodeToString(thumbprint)[:16],
		users:    slices.Clone(cfg.Users),
		codes:    map[string]authCode{},
		sessions: map[string]browserSession{},
	}
	if p.now == nil {
		p.now = time.Now
	}
	if p.logger == nil {
		p.logger = slog.New(slog.DiscardHandler)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+p.basePath+"/.well-known/openid-configuration", p.handleDiscovery)
	mux.HandleFunc("GET "+p.basePath+"/jwks", p.handleJWKS)
	mux.HandleFunc("GET "+p.basePath+"/authorize", p.handleAuthorize)
	mux.HandleFunc("POST "+p.basePath+"/authorize", p.handleLoginForm)
	mux.HandleFunc("POST "+p.basePath+"/token", p.handleToken)
	// The login form is a POST that signs a browser in, so it gets the same
	// cross-origin protection as MeuRPG's own API. The token endpoint is
	// called server to server, without the headers this checks.
	p.handler = http.NewCrossOriginProtection().Handler(mux)
	return p, nil
}

// ServeHTTP implements http.Handler.
func (p *Provider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.handler.ServeHTTP(w, r)
}

// Issuer returns the issuer URL, as the discovery document reports it.
func (p *Provider) Issuer() string { return p.cfg.Issuer }

// User returns a copy of the i-th user.
func (p *Provider) User(i int) User {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.users[i]
}

// EditUser changes the i-th user; the change applies to the next sign-in.
func (p *Provider) EditUser(i int, edit func(u *User)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	edit(&p.users[i])
}

// Tweak changes the knobs; the change applies to the next request.
func (p *Provider) Tweak(edit func(k *Knobs)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	edit(&p.knobs)
}

// SetMetadata overrides one field of the discovery document (nil removes
// it). It is a shortcut for Tweak.
func (p *Provider) SetMetadata(key string, value any) {
	p.Tweak(func(k *Knobs) {
		if k.Metadata == nil {
			k.Metadata = map[string]any{}
		}
		k.Metadata[key] = value
	})
}

// LastAuthorize returns the query of the latest GET to the authorization
// endpoint.
func (p *Provider) LastAuthorize() url.Values {
	p.mu.Lock()
	defer p.mu.Unlock()
	return maps.Clone(p.lastAuthorize)
}

func (p *Provider) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	doc := map[string]any{
		"issuer":                                p.cfg.Issuer,
		"authorization_endpoint":                p.base + "/authorize",
		"token_endpoint":                        p.base + "/token",
		"jwks_uri":                              p.base + "/jwks",
		"response_types_supported":              []string{"code"},
		"response_modes_supported":              []string{"query"},
		"grant_types_supported":                 []string{"authorization_code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{string(jose.RS256)},
		"scopes_supported":                      []string{"openid", "email", "profile"},
		"claims_supported":                      []string{"iss", "sub", "aud", "exp", "iat", "auth_time", "nonce", "name", "email", "email_verified"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"prompt_values_supported":               []string{"none", "login"},
	}
	p.mu.Lock()
	for k, v := range p.knobs.Metadata {
		if v == nil {
			delete(doc, k)
		} else {
			doc[k] = v
		}
	}
	p.mu.Unlock()
	writeJSON(w, http.StatusOK, doc)
}

func (p *Provider) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &p.key.PublicKey,
		KeyID:     p.keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}})
}

// handleAuthorize is the authorization endpoint (GET). It signs the browser
// in at once when it can (AutoSignIn, or a provider session that satisfies
// max_age and prompt), and otherwise shows the login page.
func (p *Provider) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p.mu.Lock()
	p.lastAuthorize = maps.Clone(q)
	p.mu.Unlock()

	req, ok := p.parseAuthRequest(w, r, q)
	if !ok {
		return
	}

	now := p.now()
	if p.cfg.AutoSignIn {
		p.issueCode(w, r, req, p.User(0), now)
		return
	}
	if user, authTime, ok := p.currentSession(r); ok &&
		!slices.Contains(req.prompt, "login") &&
		(req.maxAge < 0 || now.Sub(authTime) <= req.maxAge) {
		p.issueCode(w, r, req, user, authTime)
		return
	}
	if slices.Contains(req.prompt, "none") {
		// OpenID Connect Core 1.0, section 3.1.2.6: prompt=none must not
		// show any page.
		p.redirectError(w, r, req, "login_required", "the user must sign in")
		return
	}
	p.renderLoginPage(w, q)
}

// handleLoginForm handles a click on the login page: it signs the chosen
// user in, starts a provider session and finishes the authorization.
func (p *Provider) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "malformed form", http.StatusBadRequest)
		return
	}
	req, ok := p.parseAuthRequest(w, r, r.PostForm)
	if !ok {
		return
	}
	user, ok := p.userBySubject(r.PostForm.Get("user"))
	if !ok {
		http.Error(w, "unknown test user", http.StatusBadRequest)
		return
	}

	now := p.now().Truncate(time.Second)
	sessionID := rand.Text()
	p.mu.Lock()
	p.sessions[sessionID] = browserSession{subject: user.Subject, authTime: now}
	p.mu.Unlock()
	// Secure only when the issuer is https: the local stack serves the
	// provider over plain http on a loopback name.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure follows the issuer's scheme, see above
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     p.basePath + "/",
		Secure:   strings.HasPrefix(p.cfg.Issuer, "https://"),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	p.logger.InfoContext(r.Context(), "test user signed in", "user", user.Name)
	p.issueCode(w, r, req, user, now)
}

var (
	// codeChallengePattern is a base64url SHA-256 (RFC 7636, section 4.2).
	codeChallengePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	// codeVerifierPattern is RFC 7636, section 4.1.
	codeVerifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
)

// parseAuthRequest validates an authorization request. When the client or
// the redirect URI is wrong it shows an error page, and never redirects
// (RFC 6749, section 4.1.2.1); any other problem goes back to the client as
// an error redirect. It returns false when it has already answered.
func (p *Provider) parseAuthRequest(w http.ResponseWriter, r *http.Request, q url.Values) (authRequest, bool) {
	if q.Get("client_id") != p.cfg.ClientID {
		http.Error(w, "unknown client_id", http.StatusBadRequest)
		return authRequest{}, false
	}
	if !slices.Contains(p.cfg.RedirectURIs, q.Get("redirect_uri")) {
		http.Error(w, "redirect_uri is not registered for this client", http.StatusBadRequest)
		return authRequest{}, false
	}

	req := authRequest{
		redirectURI: q.Get("redirect_uri"),
		scope:       q.Get("scope"),
		state:       q.Get("state"),
		nonce:       q.Get("nonce"),
		challenge:   q.Get("code_challenge"),
		maxAge:      -1,
		prompt:      strings.Fields(q.Get("prompt")),
	}
	fail := func(code, description string) (authRequest, bool) {
		p.redirectError(w, r, req, code, description)
		return authRequest{}, false
	}
	switch {
	case q.Get("response_type") != "code":
		return fail("unsupported_response_type", "only response_type=code is supported")
	case !slices.Contains(strings.Fields(req.scope), "openid"):
		return fail("invalid_scope", "the openid scope is required")
	case q.Get("code_challenge_method") != "S256" || !codeChallengePattern.MatchString(req.challenge):
		return fail("invalid_request", "PKCE with code_challenge_method=S256 is required")
	case req.state == "":
		return fail("invalid_request", "state is required")
	case req.nonce == "":
		return fail("invalid_request", "nonce is required")
	case slices.Contains(req.prompt, "none") && len(req.prompt) > 1:
		return fail("invalid_request", "prompt=none cannot be combined with other values")
	}
	if raw := q.Get("max_age"); raw != "" {
		seconds, err := strconv.ParseUint(raw, 10, 31)
		if err != nil {
			return fail("invalid_request", "max_age must be a number of seconds")
		}
		req.maxAge = time.Duration(seconds) * time.Second
	}
	return req, true
}

// currentSession returns the user signed in at the provider in this
// browser, if any.
func (p *Provider) currentSession(r *http.Request) (User, time.Time, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return User{}, time.Time{}, false
	}
	p.mu.Lock()
	session, ok := p.sessions[c.Value]
	p.mu.Unlock()
	if !ok {
		return User{}, time.Time{}, false
	}
	user, ok := p.userBySubject(session.subject)
	return user, session.authTime, ok
}

func (p *Provider) userBySubject(subject string) (User, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, u := range p.users {
		if u.Subject == subject {
			return u, true
		}
	}
	return User{}, false
}

// issueCode redirects back to the client with a new authorization code.
func (p *Provider) issueCode(w http.ResponseWriter, r *http.Request, req authRequest, user User, authTime time.Time) {
	now := p.now()
	code := rand.Text()
	p.mu.Lock()
	for c, issued := range p.codes {
		if !now.Before(issued.expiresAt) {
			delete(p.codes, c)
		}
	}
	p.codes[code] = authCode{request: req, user: user, authTime: authTime.Truncate(time.Second), expiresAt: now.Add(codeLifetime)}
	p.mu.Unlock()
	p.redirectBack(w, r, req, url.Values{"code": {code}, "state": {req.state}})
}

// redirectError sends an OAuth error back to the client (RFC 6749, section
// 4.1.2.1).
func (p *Provider) redirectError(w http.ResponseWriter, r *http.Request, req authRequest, code, description string) {
	params := url.Values{"error": {code}, "error_description": {description}}
	if req.state != "" {
		params.Set("state", req.state)
	}
	p.redirectBack(w, r, req, params)
}

func (p *Provider) redirectBack(w http.ResponseWriter, r *http.Request, req authRequest, params url.Values) {
	back, err := url.Parse(req.redirectURI)
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	q := back.Query()
	maps.Copy(q, params)
	back.RawQuery = q.Encode()
	status := http.StatusFound
	if r.Method == http.MethodPost {
		// 303: the browser follows with a GET, not a second POST.
		status = http.StatusSeeOther
	}
	http.Redirect(w, r, back.String(), status)
}

// handleToken is the token endpoint: the authorization_code grant, with
// client_secret_basic or client_secret_post, and PKCE S256.
func (p *Provider) handleToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if !p.authenticateClient(w, r) {
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" {
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type")
		return
	}

	now := p.now()
	p.mu.Lock()
	issued, ok := p.codes[r.PostForm.Get("code")]
	delete(p.codes, r.PostForm.Get("code")) // a code works once
	knobs := p.knobs
	p.mu.Unlock()

	verifier := r.PostForm.Get("code_verifier")
	sum := sha256.Sum256([]byte(verifier))
	switch {
	case !ok, !now.Before(issued.expiresAt), r.PostForm.Get("redirect_uri") != issued.request.redirectURI:
		oauthError(w, http.StatusBadRequest, "invalid_grant")
		return
	case !codeVerifierPattern.MatchString(verifier),
		// RFC 7636, section 4.6: BASE64URL(SHA256(code_verifier)) must
		// equal the challenge sent to the authorization endpoint.
		subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(issued.request.challenge)) != 1:
		oauthError(w, http.StatusBadRequest, "invalid_grant")
		return
	}

	claims := map[string]any{
		"iss":       p.cfg.Issuer,
		"sub":       issued.user.Subject,
		"aud":       p.cfg.ClientID,
		"exp":       now.Add(idTokenLifetime).Unix(),
		"iat":       now.Unix(),
		"nonce":     issued.request.nonce,
		"auth_time": issued.authTime.Unix(),
	}
	if issued.user.Name != "" {
		claims["name"] = issued.user.Name
	}
	if issued.user.Email != "" {
		claims["email"] = issued.user.Email
	}
	if issued.user.EmailVerified != nil {
		claims["email_verified"] = issued.user.EmailVerified
	}
	if knobs.MutateClaims != nil {
		knobs.MutateClaims(claims)
	}
	idToken, err := p.sign(claims, knobs)
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": rand.Text(),
		"token_type":   "Bearer",
		"expires_in":   int(idTokenLifetime.Seconds()),
		"scope":        issued.request.scope,
		"id_token":     idToken,
	})
}

// authenticateClient checks the client credentials, sent with HTTP Basic
// or in the form (RFC 6749, section 2.3.1). It answers and returns false
// when they are wrong.
func (p *Provider) authenticateClient(w http.ResponseWriter, r *http.Request) bool {
	clientID, secret, basic := r.BasicAuth()
	if basic {
		// Basic credentials are form-encoded before base64.
		var err1, err2 error
		clientID, err1 = url.QueryUnescape(clientID)
		secret, err2 = url.QueryUnescape(secret)
		if err1 != nil || err2 != nil || r.PostForm.Has("client_secret") {
			oauthError(w, http.StatusBadRequest, "invalid_request")
			return false
		}
	} else {
		clientID, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if clientID != p.cfg.ClientID || subtle.ConstantTimeCompare([]byte(secret), []byte(p.cfg.ClientSecret)) != 1 {
		if basic {
			w.Header().Set("WWW-Authenticate", `Basic realm="oidctest"`)
		}
		oauthError(w, http.StatusUnauthorized, "invalid_client")
		return false
	}
	return true
}

// sign returns a compact JWS of claims, signed the way the knobs say.
func (p *Provider) sign(claims map[string]any, knobs Knobs) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	if knobs.Unsigned {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
		return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".", nil
	}
	key := p.key
	if knobs.SigningKey != nil {
		key = knobs.SigningKey
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: p.keyID}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		return "", err
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return jws.CompactSerialize()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func oauthError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
