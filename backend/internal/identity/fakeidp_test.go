package identity

import (
	"encoding/pem"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/identity/oidctest"
)

// fakeIDP is the in-process OpenID Connect provider of these tests: an
// oidctest.Provider served over TLS by httptest, like a real provider. The
// Service trusts it through a CA file, which exercises OIDC_CA_FILE.
//
// It signs its only user in at once, without a login page, and its knobs
// (oidctest.Knobs) let a test break one thing at a time. cmd/devidp serves
// the same provider, with a login page, for the local stack.
type fakeIDP struct {
	*oidctest.Provider
	server       *httptest.Server
	caFile       string // PEM of the server's TLS certificate
	clientID     string
	clientSecret string
	redirectURL  string
}

func newFakeIDP(t *testing.T) *fakeIDP {
	t.Helper()

	idp := &fakeIDP{
		clientID:     "meurpg-test",
		clientSecret: "test-client-secret-value",
		redirectURL:  "https://meurpg.test/auth/callback",
	}
	// The listener exists before the server starts, so the issuer URL is
	// known before the provider is built.
	idp.server = httptest.NewUnstartedServer(nil)
	issuer := "https://" + idp.server.Listener.Addr().String()

	provider, err := oidctest.New(oidctest.Config{
		Issuer:       issuer,
		ClientID:     idp.clientID,
		ClientSecret: idp.clientSecret,
		RedirectURIs: []string{idp.redirectURL},
		Users: []oidctest.User{{
			Subject: "108234567890123456789",
			// A name claim, which the Service must never store or log.
			Name:          "Nome Que Nunca Guardamos",
			Email:         "mestre@example.com",
			EmailVerified: true,
		}},
		AutoSignIn: true,
	})
	if err != nil {
		t.Fatalf("oidctest.New() error = %v", err)
	}
	idp.Provider = provider
	idp.server.Config.Handler = provider
	idp.server.StartTLS()
	t.Cleanup(idp.server.Close)
	if idp.server.URL != issuer {
		t.Fatalf("test server URL = %q, want %q", idp.server.URL, issuer)
	}

	idp.caFile = filepath.Join(t.TempDir(), "idp-ca.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: idp.server.Certificate().Raw})
	if err := os.WriteFile(idp.caFile, certPEM, 0o600); err != nil {
		t.Fatalf("write CA file: %v", err)
	}
	return idp
}

func (idp *fakeIDP) issuer() string { return idp.Issuer() }

// user is the user the provider signs in.
func (idp *fakeIDP) user() oidctest.User { return idp.User(0) }

// editUser changes the user for the next sign-in.
func (idp *fakeIDP) editUser(edit func(u *oidctest.User)) { idp.EditUser(0, edit) }
