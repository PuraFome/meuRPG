package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// TestRefusesToBeDeployed is the guard that keeps devidp off real servers.
func TestRefusesToBeDeployed(t *testing.T) {
	t.Parallel()

	refused := map[string]map[string]string{
		"a public https issuer":           {"DEVIDP_ISSUER": "https://idp.example.com"},
		"a public http issuer":            {"DEVIDP_ISSUER": "http://idp.example.com:9090"},
		"a private network address":       {"DEVIDP_ISSUER": "http://10.0.0.5:9090"},
		"every interface":                 {"DEVIDP_ISSUER": "http://0.0.0.0:9090"},
		"a name that only starts with it": {"DEVIDP_ISSUER": "http://localhost.example.com"},
		"a name that only ends with it":   {"DEVIDP_ISSUER": "http://evillocalhost:9090"},
		"a Cloud Run service":             {"DEVIDP_ISSUER": "http://localhost:9090", "K_SERVICE": "meurpg-api"},
		"a Cloud Run revision":            {"K_REVISION": "meurpg-api-00001-abc"},
		"not a URL":                       {"DEVIDP_ISSUER": "idp.localhost:9090"},
		"another scheme":                  {"DEVIDP_ISSUER": "ftp://localhost"},
		"a public issuer given as a flag": {"DEVIDP_ISSUER": "http://localhost:9090", "flag": "https://idp.example.com"},
	}
	for name, vars := range refused {
		var args []string
		if f, ok := vars["flag"]; ok {
			args = []string{"-issuer", f}
		}
		_, err := parseSettings(args, env(vars))
		switch {
		case err == nil:
			t.Errorf("%s: parseSettings() error = nil, want a refusal", name)
		case !strings.Contains(err.Error(), "never be deployed") && !strings.Contains(err.Error(), "absolute http(s) URL"):
			t.Errorf("%s: error = %q, want it to say why", name, err)
		}
	}

	allowed := []string{
		"http://localhost:9090",
		"http://idp.localhost:9090",
		"http://127.0.0.1:9090",
		"http://[::1]:9090",
		"https://idp.localhost",
	}
	for _, issuer := range allowed {
		if _, err := parseSettings(nil, env(map[string]string{"DEVIDP_ISSUER": issuer})); err != nil {
			t.Errorf("issuer %q: parseSettings() error = %v, want it accepted", issuer, err)
		}
	}
}

func TestSettings(t *testing.T) {
	t.Parallel()

	defaults, err := parseSettings(nil, env(nil))
	if err != nil {
		t.Fatalf("parseSettings() with defaults: %v", err)
	}
	if defaults.issuer != "http://localhost:9090" || defaults.listen != "127.0.0.1:9090" ||
		defaults.clientID != "meurpg-local" || len(defaults.redirectURIs) != 1 {
		t.Errorf("defaults = %+v", defaults)
	}

	s, err := parseSettings(
		[]string{"-client-id", "from-flag"},
		env(map[string]string{
			"DEVIDP_ISSUER":        "http://idp.localhost:9090",
			"DEVIDP_LISTEN":        ":9090",
			"DEVIDP_CLIENT_ID":     "from-env",
			"DEVIDP_REDIRECT_URIS": "http://localhost:8080/auth/callback, http://localhost:4200/auth/callback",
		}),
	)
	if err != nil {
		t.Fatalf("parseSettings() error = %v", err)
	}
	if s.clientID != "from-flag" {
		t.Errorf("client ID = %q, want the flag to win over the environment", s.clientID)
	}
	if s.issuer != "http://idp.localhost:9090" || s.listen != ":9090" {
		t.Errorf("issuer, listen = %q, %q", s.issuer, s.listen)
	}
	if len(s.redirectURIs) != 2 || s.redirectURIs[1] != "http://localhost:4200/auth/callback" {
		t.Errorf("redirect URIs = %q", s.redirectURIs)
	}
}

func TestServesDiscovery(t *testing.T) {
	t.Parallel()
	s, err := parseSettings(nil, env(map[string]string{"DEVIDP_ISSUER": "http://idp.localhost:9090"}))
	if err != nil {
		t.Fatal(err)
	}
	provider, err := newProvider(s, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newProvider() error = %v", err)
	}

	rec := httptest.NewRecorder()
	provider.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/openid-configuration", nil))
	var doc map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&doc); err != nil || doc["issuer"] != "http://idp.localhost:9090" {
		t.Fatalf("discovery = %v (%v), want issuer http://idp.localhost:9090", doc, err)
	}
	if doc["authorization_endpoint"] != "http://idp.localhost:9090/authorize" {
		t.Errorf("authorization_endpoint = %v", doc["authorization_endpoint"])
	}
}
