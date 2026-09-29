package identity

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// fakeIDP is an in-process OpenID Connect provider for tests. It serves
// discovery, JWKS, an authorization endpoint (which "logs the user in" at
// once and redirects back with a code) and a token endpoint that checks
// the client secret and PKCE, then signs an ID token with an RSA key.
//
// Knobs let a test break one thing at a time: the claims, the signing key,
// the discovery metadata.
type fakeIDP struct {
	server       *httptest.Server
	caFile       string // PEM of the server's TLS certificate
	clientID     string
	clientSecret string
	redirectURL  string

	key   *rsa.PrivateKey
	keyID string

	mu sync.Mutex
	// metadata is merged into the discovery document; tests change it
	// before the Service runs discovery.
	metadata map[string]any
	// codes maps each issued authorization code to its request.
	codes map[string]authRequest
	// user is who signs in at the authorization endpoint.
	user fakeUser
	// mutateClaims, when set, edits the ID token claims before signing.
	mutateClaims func(claims map[string]any)
	// signingKey, when set, signs ID tokens instead of key (a key the JWKS
	// does not publish).
	signingKey *rsa.PrivateKey
	// unsigned makes the token endpoint return an "alg":"none" ID token.
	unsigned bool
	// lastAuthorize is the query of the latest authorization request.
	lastAuthorize url.Values
	// tokenRequests counts calls to the token endpoint.
	tokenRequests int
}

type fakeUser struct {
	subject       string
	email         string
	emailVerified any // bool, string or nil (claim absent)
	authTime      time.Time
}

type authRequest struct {
	challenge   string
	nonce       string
	redirectURI string
}

func newFakeIDP(t *testing.T) *fakeIDP {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	idp := &fakeIDP{
		clientID:     "meurpg-test",
		clientSecret: "test-client-secret-value",
		redirectURL:  "https://meurpg.test/auth/callback",
		key:          key,
		keyID:        "test-key-1",
		codes:        map[string]authRequest{},
		metadata: map[string]any{
			"code_challenge_methods_supported":      []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_basic"},
		},
		user: fakeUser{
			subject:       "108234567890123456789",
			email:         "mestre@example.com",
			emailVerified: true,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", idp.handleDiscovery)
	mux.HandleFunc("GET /jwks", idp.handleJWKS)
	mux.HandleFunc("GET /authorize", idp.handleAuthorize)
	mux.HandleFunc("POST /token", idp.handleToken)

	// TLS, like a real provider; the Service trusts it through a CA file,
	// which exercises OIDC_CA_FILE.
	idp.server = httptest.NewTLSServer(mux)
	t.Cleanup(idp.server.Close)

	idp.caFile = filepath.Join(t.TempDir(), "idp-ca.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: idp.server.Certificate().Raw})
	if err := os.WriteFile(idp.caFile, certPEM, 0o600); err != nil {
		t.Fatalf("write CA file: %v", err)
	}
	return idp
}

func (idp *fakeIDP) issuer() string { return idp.server.URL }

func (idp *fakeIDP) setMetadata(key string, value any) {
	idp.mu.Lock()
	defer idp.mu.Unlock()
	if value == nil {
		delete(idp.metadata, key)
		return
	}
	idp.metadata[key] = value
}

func (idp *fakeIDP) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	idp.mu.Lock()
	defer idp.mu.Unlock()
	doc := map[string]any{
		"issuer":                                idp.issuer(),
		"authorization_endpoint":                idp.issuer() + "/authorize",
		"token_endpoint":                        idp.issuer() + "/token",
		"jwks_uri":                              idp.issuer() + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
	maps.Copy(doc, idp.metadata)
	writeJSON(w, http.StatusOK, doc)
}

func (idp *fakeIDP) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &idp.key.PublicKey,
		KeyID:     idp.keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}})
}

// handleAuthorize checks the request like a strict provider would, then
// signs the user in without a login page and redirects back with a code.
func (idp *fakeIDP) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	idp.mu.Lock()
	idp.lastAuthorize = q
	idp.mu.Unlock()

	switch {
	case q.Get("client_id") != idp.clientID:
		http.Error(w, "unknown client_id", http.StatusBadRequest)
		return
	case q.Get("redirect_uri") != idp.redirectURL:
		http.Error(w, "redirect_uri is not registered", http.StatusBadRequest)
		return
	case q.Get("response_type") != "code",
		!slices.Contains(strings.Fields(q.Get("scope")), "openid"),
		q.Get("code_challenge_method") != "S256",
		q.Get("code_challenge") == "",
		q.Get("state") == "",
		q.Get("nonce") == "":
		http.Error(w, "invalid authorization request", http.StatusBadRequest)
		return
	}

	code := rand.Text()
	idp.mu.Lock()
	idp.codes[code] = authRequest{challenge: q.Get("code_challenge"), nonce: q.Get("nonce"), redirectURI: q.Get("redirect_uri")}
	idp.mu.Unlock()

	back, _ := url.Parse(q.Get("redirect_uri"))
	back.RawQuery = url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
	http.Redirect(w, r, back.String(), http.StatusFound) //nolint:gosec // G710: redirect_uri was checked against the registered one above
}

// handleToken implements the authorization_code grant with
// client_secret_basic (or client_secret_post) and PKCE S256.
func (idp *fakeIDP) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	idp.mu.Lock()
	defer idp.mu.Unlock()
	idp.tokenRequests++

	clientID, secret, ok := r.BasicAuth()
	if !ok {
		clientID, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if clientID != idp.clientID || secret != idp.clientSecret {
		oauthError(w, http.StatusUnauthorized, "invalid_client")
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" {
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type")
		return
	}
	req, ok := idp.codes[r.PostForm.Get("code")]
	delete(idp.codes, r.PostForm.Get("code")) // codes are single use
	if !ok || r.PostForm.Get("redirect_uri") != req.redirectURI {
		oauthError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	// RFC 7636: BASE64URL(SHA256(code_verifier)) must equal the challenge.
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != req.challenge {
		oauthError(w, http.StatusBadRequest, "invalid_grant")
		return
	}

	now := time.Now()
	claims := map[string]any{
		"iss":   idp.issuer(),
		"sub":   idp.user.subject,
		"aud":   idp.clientID,
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": req.nonce,
		"name":  "Nome Que Nunca Guardamos",
	}
	if idp.user.email != "" {
		claims["email"] = idp.user.email
	}
	if idp.user.emailVerified != nil {
		claims["email_verified"] = idp.user.emailVerified
	}
	if !idp.user.authTime.IsZero() {
		claims["auth_time"] = idp.user.authTime.Unix()
	}
	if idp.mutateClaims != nil {
		idp.mutateClaims(claims)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": "fake-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     idp.sign(claims),
	})
}

// sign returns a compact JWS of claims, signed the way the knobs say.
func (idp *fakeIDP) sign(claims map[string]any) string {
	payload, err := json.Marshal(claims)
	if err != nil {
		panic(err)
	}
	if idp.unsigned {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
		return header + "." + base64.RawURLEncoding.EncodeToString(payload) + "."
	}
	key := idp.key
	if idp.signingKey != nil {
		key = idp.signingKey
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: idp.keyID}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		panic(err)
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		panic(err)
	}
	compact, err := jws.CompactSerialize()
	if err != nil {
		panic(err)
	}
	return compact
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func oauthError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
