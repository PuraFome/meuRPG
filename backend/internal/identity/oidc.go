package identity

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
)

// providerTimeout bounds every call to the provider (discovery, keys, token
// exchange), so a slow provider cannot hold a request forever.
const providerTimeout = 10 * time.Second

// scopes asks for the identity (openid) and the e-mail, the only personal
// data we use (docs/privacidade.md). No profile: we never want the name or
// the photo.
var scopes = []string{oidc.ScopeOpenID, oidc.ScopeEmail}

// provider is a discovered OpenID Connect provider, ready to use.
type provider struct {
	issuer   string
	clientID string
	oauth2   oauth2.Config
	verifier *oidc.IDTokenVerifier
	client   *http.Client

	// maxAge is sent as max_age when positive (OIDC_MAX_AGE). Discovery
	// has no field that says a provider honors max_age, so configuration
	// decides.
	maxAge time.Duration
}

// providerSource discovers the provider on first use and caches it.
type providerSource struct {
	cfg    config.OIDC
	client *http.Client
	now    func() time.Time

	mu sync.Mutex
	p  *provider
}

// get returns the provider, running discovery if it has not succeeded yet.
// The lock is held during discovery on purpose: concurrent sign-ins during
// an outage wait for one attempt instead of each starting their own.
func (ps *providerSource) get(ctx context.Context) (*provider, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.p != nil {
		return ps.p, nil
	}
	p, err := discover(ctx, ps.cfg, ps.client, ps.now)
	if err != nil {
		return nil, err
	}
	ps.p = p
	return p, nil
}

// discoveryMetadata holds the fields of the discovery document that go-oidc
// does not expose. See OpenID Connect Discovery 1.0, section 3, and RFC 8414.
type discoveryMetadata struct {
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// discover reads the provider's discovery document. go-oidc checks that the
// document's issuer is exactly cfg.IssuerURL.
func discover(ctx context.Context, cfg config.OIDC, client *http.Client, now func() time.Time) (*provider, error) {
	// The same client is used later to fetch the signing keys (JWKS).
	ctx = oidc.ClientContext(ctx, client)
	op, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", cfg.IssuerURL, err)
	}

	var meta discoveryMetadata
	if err := op.Claims(&meta); err != nil {
		return nil, fmt.Errorf("read discovery metadata: %w", err)
	}
	// A provider that lists its PKCE methods but not S256 would ignore our
	// code_challenge, and the flow would silently lose PKCE. If the list is
	// missing, we still send S256 and let the provider enforce it.
	if len(meta.CodeChallengeMethodsSupported) > 0 && !slices.Contains(meta.CodeChallengeMethodsSupported, "S256") {
		return nil, errors.New("the provider does not support PKCE with S256")
	}
	authStyle, err := tokenAuthStyle(meta.TokenEndpointAuthMethodsSupported)
	if err != nil {
		return nil, err
	}

	endpoint := op.Endpoint()
	endpoint.AuthStyle = authStyle
	return &provider{
		issuer:   cfg.IssuerURL,
		clientID: cfg.ClientID,
		oauth2: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret.Reveal(),
			Endpoint:     endpoint,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       scopes,
		},
		// go-oidc checks the signature against the provider's JWKS (only
		// with the algorithms the provider advertises), iss, aud and exp.
		verifier: op.Verifier(&oidc.Config{ClientID: cfg.ClientID, Now: now}),
		client:   client,
		maxAge:   cfg.MaxAge,
	}, nil
}

// tokenAuthStyle picks how to send the client secret to the token endpoint.
// When the provider does not say, the default is client_secret_basic
// (OpenID Connect Discovery 1.0, section 3). Choosing up front avoids
// oauth2's auto-detection, which may send the secret twice.
func tokenAuthStyle(methods []string) (oauth2.AuthStyle, error) {
	switch {
	case len(methods) == 0, slices.Contains(methods, "client_secret_basic"):
		return oauth2.AuthStyleInHeader, nil
	case slices.Contains(methods, "client_secret_post"):
		return oauth2.AuthStyleInParams, nil
	default:
		return 0, fmt.Errorf("the provider supports neither client_secret_basic nor client_secret_post (it lists %v)", methods)
	}
}

// authCodeURL builds the URL that sends the browser to the provider.
func (p *provider) authCodeURL(state, nonce, verifier string) string {
	opts := []oauth2.AuthCodeOption{
		oauth2.S256ChallengeOption(verifier),
		oidc.Nonce(nonce),
	}
	if p.maxAge > 0 {
		opts = append(opts, oauth2.SetAuthURLParam("max_age", strconv.Itoa(int(p.maxAge.Seconds()))))
	}
	return p.oauth2.AuthCodeURL(state, opts...)
}

// exchange trades the authorization code for tokens and returns the raw ID
// token. The PKCE verifier proves we are the client that started the flow.
// The access token is not used: everything we need is in the ID token.
func (p *provider) exchange(ctx context.Context, code, verifier string) (string, error) {
	ctx = oidc.ClientContext(ctx, p.client)
	token, err := p.oauth2.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", err
	}
	rawIDToken, _ := token.Extra("id_token").(string)
	if rawIDToken == "" {
		return "", errors.New("the token response has no id_token")
	}
	return rawIDToken, nil
}

// verifiedIdentity is what a valid ID token tells us about the user.
type verifiedIdentity struct {
	Issuer  string
	Subject string
	// Email is set only when the provider marks it as verified.
	Email string
	// AuthTime is the auth_time claim, zero when absent. It is recorded
	// for auditing only, never trusted for a decision: providers differ
	// (Google sends it only on request; a local test provider kept the
	// first login's time even after forcing a new login with max_age).
	AuthTime time.Time
}

// extraClaims are the ID token claims go-oidc does not check or expose.
type extraClaims struct {
	AuthorizedParty string          `json:"azp"`
	AuthTime        json.RawMessage `json:"auth_time"`
	Email           string          `json:"email"`
	EmailVerified   flexibleBool    `json:"email_verified"`
}

// verify checks a raw ID token, following OpenID Connect Core 1.0, section
// 3.1.3.7. go-oidc does steps 1 to 3 and 9 (signature, iss, aud, exp); the
// rest is here.
func (p *provider) verify(ctx context.Context, rawIDToken, wantNonce string) (verifiedIdentity, error) {
	ctx = oidc.ClientContext(ctx, p.client)
	token, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return verifiedIdentity{}, err
	}

	// The nonce ties the token to the login state of this browser, so a
	// token stolen from another sign-in cannot be replayed here.
	if subtle.ConstantTimeCompare([]byte(token.Nonce), []byte(wantNonce)) != 1 {
		return verifiedIdentity{}, errors.New("the ID token nonce does not match")
	}

	// go-oidc accepts a token whose aud merely contains our client ID. Core
	// says to reject tokens with extra audiences the client does not trust,
	// and we trust none.
	for _, aud := range token.Audience {
		if aud != p.clientID {
			return verifiedIdentity{}, errors.New("the ID token has an audience other than this client")
		}
	}

	var extra extraClaims
	if err := token.Claims(&extra); err != nil {
		return verifiedIdentity{}, fmt.Errorf("read ID token claims: %w", err)
	}
	// azp is optional, but when present it must be us.
	if extra.AuthorizedParty != "" && extra.AuthorizedParty != p.clientID {
		return verifiedIdentity{}, errors.New("the ID token azp is not this client")
	}
	// go-oidc does not require sub, which is our key for the user.
	if token.Subject == "" || len(token.Subject) > 255 {
		return verifiedIdentity{}, errors.New("the ID token sub is missing or longer than 255 characters")
	}

	id := verifiedIdentity{
		// The configured issuer, not token.Issuer: go-oidc has checked that
		// they match, but it also accepts "accounts.google.com" for
		// "https://accounts.google.com". Storing the token's spelling
		// would split one person into two accounts.
		Issuer:   p.issuer,
		Subject:  token.Subject,
		AuthTime: numericDate(extra.AuthTime),
	}
	if extra.EmailVerified {
		id.Email = extra.Email
	}
	return id, nil
}

// maxNumericDate is 9999-12-31T23:59:59Z, the last second the database can
// store.
const maxNumericDate = 253402300799

// numericDate decodes an RFC 7519 NumericDate (seconds since 1970, maybe
// with a fraction). auth_time is informational, so a missing or malformed
// value is ignored instead of failing the sign-in.
func numericDate(raw json.RawMessage) time.Time {
	var seconds float64
	if len(raw) == 0 || json.Unmarshal(raw, &seconds) != nil || !(seconds > 0 && seconds <= maxNumericDate) {
		return time.Time{}
	}
	return time.Unix(int64(seconds), 0)
}

// flexibleBool decodes a JSON boolean, or the strings "true" and "false",
// which some providers send for email_verified. Anything else is false: an
// unexpected value must not make an e-mail count as verified.
type flexibleBool bool

// UnmarshalJSON implements json.Unmarshaler.
func (b *flexibleBool) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*b = v == true || v == "true"
	return nil
}

// newHTTPClient returns the client used to talk to the provider. It trusts
// the system's CA certificates plus, when caFile is set, the ones in that
// PEM file (for a local provider with a self-signed certificate).
func newHTTPClient(caFile string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}

	if caFile != "" {
		// The path comes from the operator's configuration, not a request.
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read OIDC_CA_FILE: %w", err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("OIDC_CA_FILE %s has no PEM certificate", caFile)
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	return &http.Client{Transport: transport, Timeout: providerTimeout}, nil
}
