package identity

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	identityv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/identity/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/identity/v1/identityv1connect"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func isUnauthenticated(err error) bool {
	return connect.CodeOf(err) == connect.CodeUnauthenticated
}

func TestGetMeUnauthenticated(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	unknown, _ := newSecret()

	tests := []struct {
		name   string
		cookie *http.Cookie
	}{
		{"no cookie", nil},
		{"empty cookie", reqCookie(SessionCookieName, "")},
		{"malformed token", reqCookie(SessionCookieName, "not-a-token")},
		{"well-formed token that was never issued", reqCookie(SessionCookieName, unknown)},
		{"the right token under another cookie name", reqCookie("meurpg_session", h.signIn().Value)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := h.getMe(tt.cookie)
			if !isUnauthenticated(err) {
				t.Fatalf("GetMe() error = %v, want unauthenticated", err)
			}
			if ce, ok := errors.AsType[*connect.Error](err); !ok || ce.Meta().Get("Cache-Control") != "no-store" {
				t.Errorf("unauthenticated response lacks Cache-Control: no-store")
			}
		})
	}
}

func TestGetMeWithHTTPGet(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	session := h.signIn()

	// GetMe is NO_SIDE_EFFECTS, so Connect clients may use GET. The request
	// is empty, so the URL carries no personal data.
	me, err := h.getMe(session, connect.WithHTTPGet())
	if err != nil {
		t.Fatalf("GetMe() over GET error = %v", err)
	}
	if got := me.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("GetMe Cache-Control = %q, want no-store", got)
	}
}

func TestSessionHasAnAbsoluteThirtyDayLifetime(t *testing.T) {
	t.Parallel()
	for _, store := range testStores(t) {
		t.Run(store.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, withStore(store.new(t)))
			start := h.clock.Now()
			session := h.signIn()

			// Using the session does not extend it (no sliding expiry).
			for _, day := range []int{1, 10, 29} {
				h.clock.Set(start.Add(time.Duration(day) * 24 * time.Hour))
				me, err := h.getMe(session)
				if err != nil {
					t.Fatalf("day %d: GetMe() error = %v", day, err)
				}
				if got := me.Msg.GetSessionExpiresAt().AsTime(); !got.Equal(start.Add(SessionLifetime)) {
					t.Errorf("day %d: session ends %v, want %v (never extended)", day, got, start.Add(SessionLifetime))
				}
			}

			h.clock.Set(start.Add(SessionLifetime - time.Second))
			if _, err := h.getMe(session); err != nil {
				t.Errorf("one second before 30 days: GetMe() error = %v, want the session to work", err)
			}
			h.clock.Advance(time.Second)
			if _, err := h.getMe(session); !isUnauthenticated(err) {
				t.Errorf("at 30 days: GetMe() error = %v, want unauthenticated", err)
			}
		})
	}
}

func TestSignOut(t *testing.T) {
	t.Parallel()
	for _, store := range testStores(t) {
		t.Run(store.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, withStore(store.new(t)))
			session := h.signIn()
			otherDevice := h.signIn()

			res, err := h.client(session).SignOut(t.Context(), connect.NewRequest(&identityv1.SignOutRequest{}))
			if err != nil {
				t.Fatalf("SignOut() error = %v", err)
			}
			cleared := cookieIn((&http.Response{Header: res.Header()}).Cookies(), SessionCookieName)
			if cleared == nil || cleared.Value != "" || cleared.MaxAge >= 0 {
				t.Errorf("SignOut Set-Cookie = %q, want the session cookie deleted", res.Header().Values("Set-Cookie"))
			} else {
				assertHostCookie(t, cleared)
			}
			if got := res.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("SignOut Cache-Control = %q, want no-store", got)
			}

			// Revoked on the server: the same token no longer works...
			if _, err := h.getMe(session); !isUnauthenticated(err) {
				t.Errorf("GetMe() after SignOut error = %v, want unauthenticated", err)
			}
			// ...but the user's other device is still signed in.
			if _, err := h.getMe(otherDevice); err != nil {
				t.Errorf("GetMe() on the other device error = %v, want it still signed in", err)
			}

			// Signing out again: unauthenticated, and the stale cookie is
			// cleared all the same.
			_, err = h.client(session).SignOut(t.Context(), connect.NewRequest(&identityv1.SignOutRequest{}))
			if !isUnauthenticated(err) {
				t.Fatalf("second SignOut() error = %v, want unauthenticated", err)
			}
			if ce, ok := errors.AsType[*connect.Error](err); !ok || !strings.Contains(strings.Join(ce.Meta().Values("Set-Cookie"), ";"), SessionCookieName+"=;") {
				t.Errorf("second SignOut did not clear the cookie: %v", err)
			}
		})
	}
}

func TestSignOutUnauthenticated(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_, err := h.client(nil).SignOut(t.Context(), connect.NewRequest(&identityv1.SignOutRequest{}))
	if !isUnauthenticated(err) {
		t.Errorf("SignOut() without a session error = %v, want unauthenticated", err)
	}
}

func TestConnectRequiresTheProtocolHeader(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	// A plain JSON POST, like an HTML form or fetch() from another site
	// could send without a CORS preflight, is refused before the handler.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		identityv1connect.IdentityServiceSignOutProcedure, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Connect-Protocol-Version") {
		t.Errorf("POST without Connect-Protocol-Version: status %d, body %s; want 400 asking for the header", rec.Code, rec.Body)
	}
}

func TestSessionLookupWhenTheStoreIsDown(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withStore(failingStore{}))
	token, _ := newSecret()

	// Answering "unauthenticated" would make the app think the user signed
	// out; "unavailable" says to try again.
	_, err := h.getMe(reqCookie(SessionCookieName, token))
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Errorf("GetMe() error = %v, want unavailable", err)
	}
}

func TestRequireSessionFailsClosed(t *testing.T) {
	t.Parallel()
	// Without the interceptor there is never a session in the context.
	if _, err := RequireSession(context.Background()); !isUnauthenticated(err) {
		t.Errorf("RequireSession() error = %v, want unauthenticated", err)
	}
}

func TestMountDisabled(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	MountDisabled(mux.Handle, connect.WithRequireConnectProtocolHeader())

	for _, path := range []string{"/auth/login?return_to=/", "/auth/callback?code=x&state=y"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "not configured") {
			t.Errorf("GET %s: status %d, body %q; want 503 saying sign-in is not configured", path, rec.Code, rec.Body)
		}
	}

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := identityv1connect.NewIdentityServiceClient(server.Client(), server.URL)
	if _, err := client.GetMe(t.Context(), connect.NewRequest(&identityv1.GetMeRequest{})); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Errorf("GetMe() error = %v, want unavailable", err)
	}
	if _, err := client.SignOut(t.Context(), connect.NewRequest(&identityv1.SignOutRequest{})); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Errorf("SignOut() error = %v, want unavailable", err)
	}
}

func TestNewValidatesItsConfig(t *testing.T) {
	t.Parallel()
	idp := newFakeIDP(t)
	valid := Config{Store: newMemStore(), Logger: discardLogger()}
	valid.OIDC.IssuerURL = idp.issuer()
	valid.OIDC.ClientID = idp.clientID
	valid.OIDC.RedirectURL = idp.redirectURL

	noOIDC := valid
	noOIDC.OIDC.IssuerURL = ""
	noStore := valid
	noStore.Store = nil
	missingCA := valid
	missingCA.OIDC.CAFile = "/does/not/exist.pem"
	notPEM := valid
	notPEM.OIDC.CAFile = writeTemp(t, "not a certificate")

	for name, cfg := range map[string]Config{
		"no OIDC":         noOIDC,
		"no store":        noStore,
		"missing CA file": missingCA,
		"CA file, no PEM": notPEM,
	} {
		if _, err := New(t.Context(), cfg); err == nil {
			t.Errorf("%s: New() error = nil, want an error", name)
		}
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestUpdateProfile(t *testing.T) {
	t.Parallel()
	for _, store := range testStores(t) {
		t.Run(store.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, withStore(store.new(t)))
			session := h.signIn()
			client := h.client(session)
			update := func(name string) (*connect.Response[identityv1.UpdateProfileResponse], error) {
				return client.UpdateProfile(t.Context(), connect.NewRequest(&identityv1.UpdateProfileRequest{DisplayName: name}))
			}
			displayName := func() string {
				t.Helper()
				me, err := h.getMe(session)
				if err != nil {
					t.Fatalf("GetMe() error = %v", err)
				}
				return me.Msg.GetUser().GetDisplayName()
			}

			// A new account has no display name: nothing comes from the
			// provider, even though the fake one sends a name.
			if got := displayName(); got != "" {
				t.Errorf("display name of a new account = %q, want none", got)
			}

			res, err := update("  Pensantus  ")
			if err != nil {
				t.Fatalf("UpdateProfile() error = %v", err)
			}
			if got := res.Msg.GetUser().GetDisplayName(); got != "Pensantus" {
				t.Errorf("UpdateProfile() display name = %q, want it trimmed", got)
			}
			if got := res.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("UpdateProfile Cache-Control = %q, want no-store", got)
			}
			if got := displayName(); got != "Pensantus" {
				t.Errorf("GetMe() display name = %q, want Pensantus", got)
			}

			// Invalid names are refused, and the saved one stays.
			for _, bad := range []string{strings.Repeat("x", MaxDisplayNameLength+1), "Pensan\ntus", "Pensan\u202etus"} {
				if _, err := update(bad); connect.CodeOf(err) != connect.CodeInvalidArgument {
					t.Errorf("UpdateProfile(%q) error = %v, want invalid_argument", bad, err)
				}
			}
			if _, err := update(strings.Repeat("é", MaxDisplayNameLength)); err != nil {
				t.Errorf("UpdateProfile(40 characters) error = %v", err)
			}

			// An empty name removes it.
			if _, err := update("   "); err != nil {
				t.Fatalf("UpdateProfile(empty) error = %v", err)
			}
			if got := displayName(); got != "" {
				t.Errorf("display name after removing it = %q, want none", got)
			}

			if _, err := h.client(nil).UpdateProfile(t.Context(), connect.NewRequest(&identityv1.UpdateProfileRequest{DisplayName: "X"})); !isUnauthenticated(err) {
				t.Errorf("UpdateProfile() signed out error = %v, want unauthenticated", err)
			}
		})
	}
}

func TestDisplayNames(t *testing.T) {
	t.Parallel()
	for _, store := range testStores(t) {
		t.Run(store.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, withStore(store.new(t)))
			ctx := t.Context()
			named, err := h.store.UpsertUser(ctx, ExternalIdentity{Issuer: "https://idp.test", Subject: "named"})
			if err != nil {
				t.Fatalf("UpsertUser() error = %v", err)
			}
			nameless, err := h.store.UpsertUser(ctx, ExternalIdentity{Issuer: "https://idp.test", Subject: "nameless"})
			if err != nil {
				t.Fatalf("UpsertUser() error = %v", err)
			}
			if err := h.store.SetDisplayName(ctx, named, "Ana"); err != nil {
				t.Fatalf("SetDisplayName() error = %v", err)
			}

			got, err := h.svc.DisplayNames(ctx, []string{named, nameless})
			if err != nil {
				t.Fatalf("DisplayNames() error = %v", err)
			}
			if len(got) != 1 || got[named] != "Ana" {
				t.Errorf("DisplayNames() = %v, want only Ana", got)
			}
		})
	}
}
