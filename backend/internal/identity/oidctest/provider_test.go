package oidctest

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// The identity package's tests cover the provider as MeuRPG uses it, with
// AutoSignIn. These cover what only cmd/devidp uses: the login page, the
// provider session, max_age, prompt and auth_time.

const (
	testIssuer   = "http://idp.localhost:9090"
	testClient   = "meurpg-local"
	testSecret   = "local-secret"
	testCallback = "http://localhost:8080/auth/callback"
	testVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk" // RFC 7636, appendix B
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestProvider(t *testing.T) (*Provider, *clock) {
	t.Helper()
	c := &clock{now: time.Now().Truncate(time.Second)}
	p, err := New(Config{
		Issuer:       testIssuer,
		ClientID:     testClient,
		ClientSecret: testSecret,
		RedirectURIs: []string{testCallback},
		Users:        TestUsers(),
		Now:          c.Now,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return p, c
}

func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// authorizeQuery is a valid authorization request, with overrides applied
// (an empty value removes the parameter).
func authorizeQuery(overrides map[string]string) url.Values {
	q := url.Values{
		"client_id":             {testClient},
		"redirect_uri":          {testCallback},
		"response_type":         {"code"},
		"scope":                 {"openid email"},
		"state":                 {"the-state"},
		"nonce":                 {"the-nonce"},
		"code_challenge":        {challenge(testVerifier)},
		"code_challenge_method": {"S256"},
	}
	for k, v := range overrides {
		if v == "" {
			q.Del(k)
		} else {
			q.Set(k, v)
		}
	}
	return q
}

func do(t *testing.T, p *Provider, req *http.Request, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	for _, c := range cookies {
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	return rec
}

func getAuthorize(t *testing.T, p *Provider, q url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, testIssuer+"/authorize?"+q.Encode(), nil)
	return do(t, p, req, cookies...)
}

// clickUser submits the login page's form as the browser would.
func clickUser(t *testing.T, p *Provider, q url.Values, subject string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"user": {subject}}
	for _, name := range forwardedParams {
		if q.Has(name) {
			form.Set(name, q.Get(name))
		}
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, testIssuer+"/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	return do(t, p, req)
}

// callbackParams checks that rec redirects to the callback and returns its
// query.
func callbackParams(t *testing.T, rec *httptest.ResponseRecorder) url.Values {
	t.Helper()
	if rec.Code != http.StatusFound && rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want a redirect; body: %s", rec.Code, rec.Body)
	}
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil || !strings.HasPrefix(u.String(), testCallback+"?") {
		t.Fatalf("redirected to %q, want the callback", rec.Header().Get("Location"))
	}
	return u.Query()
}

// exchange trades a code for an ID token and returns its verified claims.
func exchange(t *testing.T, p *Provider, code, verifier string) (int, map[string]any) {
	t.Helper()
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {testCallback},
		"code_verifier": {verifier},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, testIssuer+"/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(testClient, testSecret)
	rec := do(t, p, req)
	if rec.Code != http.StatusOK {
		return rec.Code, nil
	}
	var body struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	jws, err := jose.ParseSigned(body.IDToken, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		t.Fatalf("parse ID token: %v", err)
	}
	payload, err := jws.Verify(&p.key.PublicKey)
	if err != nil {
		t.Fatalf("verify ID token: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	return rec.Code, claims
}

func TestLoginPageSignsTheChosenUserIn(t *testing.T) {
	t.Parallel()
	p, c := newTestProvider(t)
	q := authorizeQuery(nil)

	// No provider session yet: the login page lists every test user.
	rec := getAuthorize(t, p, q)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /authorize status = %d, want 200 (the login page)", rec.Code)
	}
	page := rec.Body.String()
	for _, u := range TestUsers() {
		if !strings.Contains(page, ">"+u.Name+"</button>") {
			t.Errorf("the login page has no button for %s", u.Name)
		}
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("Content-Security-Policy = %q", csp)
	}

	// A click on "Jogador Teste" goes back to the client with a code.
	loginAt := c.Now()
	c.Advance(5 * time.Second)
	rec = clickUser(t, p, q, "devidp-jogador")
	if rec.Code != http.StatusSeeOther {
		t.Errorf("login form status = %d, want 303", rec.Code)
	}
	back := callbackParams(t, rec)
	if back.Get("state") != "the-state" || back.Get("code") == "" {
		t.Fatalf("callback query = %v, want the state and a code", back)
	}

	status, claims := exchange(t, p, back.Get("code"), testVerifier)
	if status != http.StatusOK {
		t.Fatalf("token status = %d, want 200", status)
	}
	want := map[string]any{
		"iss":            testIssuer,
		"aud":            testClient,
		"sub":            "devidp-jogador",
		"nonce":          "the-nonce",
		"email":          "jogador@example.com",
		"email_verified": true,
		"name":           "Jogador Teste",
		"auth_time":      float64(loginAt.Add(5 * time.Second).Unix()),
	}
	for k, v := range want {
		if claims[k] != v {
			t.Errorf("claim %s = %v, want %v", k, claims[k], v)
		}
	}

	// The code works once.
	if status, _ := exchange(t, p, back.Get("code"), testVerifier); status != http.StatusBadRequest {
		t.Errorf("second exchange status = %d, want 400", status)
	}
}

func TestProviderSessionMaxAgeAndPrompt(t *testing.T) {
	t.Parallel()
	p, c := newTestProvider(t)

	rec := clickUser(t, p, authorizeQuery(nil), "devidp-mestre")
	session := rec.Result().Cookies()
	if len(session) != 1 || session[0].Name != sessionCookieName || !session[0].HttpOnly {
		t.Fatalf("login set cookies %v, want one HttpOnly %s", session, sessionCookieName)
	}
	firstCode := callbackParams(t, rec).Get("code")
	_, first := exchange(t, p, firstCode, testVerifier)

	c.Advance(30 * time.Minute)

	// The session signs the browser in again without the page, and
	// auth_time is still the time of the click.
	back := callbackParams(t, getAuthorize(t, p, authorizeQuery(map[string]string{"max_age": "3600"}), session...))
	_, again := exchange(t, p, back.Get("code"), testVerifier)
	if again["sub"] != "devidp-mestre" || again["auth_time"] != first["auth_time"] {
		t.Errorf("silent sign-in: sub=%v auth_time=%v, want devidp-mestre and %v", again["sub"], again["auth_time"], first["auth_time"])
	}

	// Older than max_age: the user must sign in again.
	if rec := getAuthorize(t, p, authorizeQuery(map[string]string{"max_age": "1799"}), session...); rec.Code != http.StatusOK {
		t.Errorf("max_age exceeded: status = %d, want 200 (the login page)", rec.Code)
	}
	// prompt=login always shows the page.
	if rec := getAuthorize(t, p, authorizeQuery(map[string]string{"prompt": "login"}), session...); rec.Code != http.StatusOK {
		t.Errorf("prompt=login: status = %d, want 200 (the login page)", rec.Code)
	}
	// prompt=select_account shows it too: the person chooses the account again.
	if rec := getAuthorize(t, p, authorizeQuery(map[string]string{"prompt": "select_account"}), session...); rec.Code != http.StatusOK {
		t.Errorf("prompt=select_account: status = %d, want 200 (the login page)", rec.Code)
	}
	// prompt=none never shows it.
	back = callbackParams(t, getAuthorize(t, p, authorizeQuery(map[string]string{"prompt": "none"})))
	if back.Get("error") != "login_required" || back.Get("state") != "the-state" {
		t.Errorf("prompt=none without a session: callback query = %v, want error=login_required", back)
	}
}

func TestAuthorizationRequestChecks(t *testing.T) {
	t.Parallel()

	// A wrong client or redirect URI gets an error page, never a redirect.
	for _, override := range []map[string]string{
		{"client_id": "someone-else"},
		{"redirect_uri": "http://localhost:8080/elsewhere"},
		{"redirect_uri": ""},
	} {
		p, _ := newTestProvider(t)
		if rec := getAuthorize(t, p, authorizeQuery(override)); rec.Code != http.StatusBadRequest || rec.Header().Get("Location") != "" {
			t.Errorf("%v: status = %d, Location = %q; want 400 and no redirect", override, rec.Code, rec.Header().Get("Location"))
		}
	}

	// Anything else goes back to the client as an OAuth error.
	for override, wantError := range map[string]string{
		"code_challenge_method=plain": "invalid_request",
		"code_challenge=":             "invalid_request",
		"code_challenge=short":        "invalid_request",
		"state=":                      "invalid_request",
		"nonce=":                      "invalid_request",
		"scope=email":                 "invalid_scope",
		"response_type=token":         "unsupported_response_type",
		"max_age=-1":                  "invalid_request",
		"prompt=none login":           "invalid_request",
	} {
		k, v, _ := strings.Cut(override, "=")
		p, _ := newTestProvider(t)
		back := callbackParams(t, getAuthorize(t, p, authorizeQuery(map[string]string{k: v})))
		if back.Get("error") != wantError || back.Has("code") {
			t.Errorf("%s: callback query = %v, want error=%s and no code", override, back, wantError)
		}
	}
}

func TestTokenEndpointChecks(t *testing.T) {
	t.Parallel()

	codeFor := func(t *testing.T, p *Provider) string {
		t.Helper()
		return callbackParams(t, clickUser(t, p, authorizeQuery(nil), "devidp-mestre")).Get("code")
	}

	t.Run("a PKCE verifier that does not match the challenge", func(t *testing.T) {
		t.Parallel()
		p, _ := newTestProvider(t)
		if status, _ := exchange(t, p, codeFor(t, p), strings.Repeat("a", 43)); status != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", status)
		}
	})

	t.Run("an expired code", func(t *testing.T) {
		t.Parallel()
		p, c := newTestProvider(t)
		code := codeFor(t, p)
		c.Advance(codeLifetime)
		if status, _ := exchange(t, p, code, testVerifier); status != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", status)
		}
	})

	t.Run("a wrong client secret", func(t *testing.T) {
		t.Parallel()
		p, _ := newTestProvider(t)
		form := url.Values{"grant_type": {"authorization_code"}, "code": {codeFor(t, p)}, "redirect_uri": {testCallback}, "code_verifier": {testVerifier}}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, testIssuer+"/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(testClient, "wrong")
		if rec := do(t, p, req); rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestLoginFormRejectsCrossOriginPosts(t *testing.T) {
	t.Parallel()
	p, _ := newTestProvider(t)
	form := authorizeQuery(nil)
	form.Set("user", "devidp-mestre")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, testIssuer+"/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if rec := do(t, p, req); rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestNewValidatesItsConfig(t *testing.T) {
	t.Parallel()
	valid := Config{Issuer: testIssuer, ClientID: testClient, ClientSecret: testSecret, RedirectURIs: []string{testCallback}, Users: TestUsers()}
	broken := map[string]func(c *Config){
		"relative issuer":     func(c *Config) { c.Issuer = "idp.localhost" },
		"issuer with a query": func(c *Config) { c.Issuer = testIssuer + "?x=1" },
		"no client secret":    func(c *Config) { c.ClientSecret = "" },
		"no redirect URI":     func(c *Config) { c.RedirectURIs = nil },
		"no users":            func(c *Config) { c.Users = nil },
		"duplicate subject":   func(c *Config) { c.Users = append(c.Users, c.Users[0]) },
	}
	for name, breakIt := range broken {
		cfg := valid
		cfg.Users = TestUsers()
		breakIt(&cfg)
		if _, err := New(cfg); err == nil {
			t.Errorf("%s: New() error = nil, want an error", name)
		}
	}
}
