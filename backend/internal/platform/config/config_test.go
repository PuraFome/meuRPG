package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// env turns a map into a getenv function, so each test case declares its
// environment in one place.
func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr []string // substrings expected in the error; nil means success
	}{
		{
			name: "defaults when nothing is set",
			env:  map[string]string{},
			want: Config{Port: 8080, LogLevel: slog.LevelInfo, WebDir: DefaultWebDir},
		},
		{
			name: "all variables set",
			env: map[string]string{
				"PORT":         "9090",
				"DATABASE_URL": "postgresql://root@localhost:26257/meurpg?sslmode=disable",
				"LOG_LEVEL":    "DEBUG",
				"WEB_DIR":      "/srv/web",
				"BLOB_DIR":     "/var/lib/meurpg/images",
			},
			want: Config{
				Port:        9090,
				DatabaseURL: "postgresql://root@localhost:26257/meurpg?sslmode=disable",
				LogLevel:    slog.LevelDebug,
				WebDir:      "/srv/web",
				BlobDir:     "/var/lib/meurpg/images",
			},
		},
		{
			name: "surrounding whitespace is ignored",
			env: map[string]string{
				"PORT": " 3000 ", "LOG_LEVEL": " warn ", "DATABASE_URL": "  ", "WEB_DIR": "  ", "BLOB_DIR": " ",
			},
			want: Config{Port: 3000, LogLevel: slog.LevelWarn, WebDir: DefaultWebDir},
		},
		{
			name:    "port is not a number",
			env:     map[string]string{"PORT": "http"},
			wantErr: []string{"PORT must be a number"},
		},
		{
			name:    "port out of range",
			env:     map[string]string{"PORT": "70000"},
			wantErr: []string{"PORT must be between 1 and 65535"},
		},
		{
			name:    "unknown log level",
			env:     map[string]string{"LOG_LEVEL": "verbose"},
			wantErr: []string{"LOG_LEVEL must be one of"},
		},
		{
			name:    "every problem is reported at once",
			env:     map[string]string{"PORT": "0", "LOG_LEVEL": "loud"},
			wantErr: []string{"PORT", "LOG_LEVEL"},
		},
		{
			name: "oidc fully configured",
			env:  withOIDC(nil),
			want: Config{Port: 8080, LogLevel: slog.LevelInfo, WebDir: DefaultWebDir, OIDC: OIDC{
				IssuerURL:    "https://idp.example.com",
				ClientID:     "meurpg",
				ClientSecret: "s3cret",
				RedirectURL:  "https://meurpg.example.com/auth/callback",
			}},
		},
		{
			name: "oidc with a CA file, max_age and a local http provider",
			env: withOIDC(map[string]string{
				"OIDC_ISSUER":       "http://localhost:9443/oauth2/token",
				"OIDC_REDIRECT_URL": "http://127.0.0.1:8080/auth/callback",
				"OIDC_CA_FILE":      "/certs/local-idp.pem",
				"OIDC_MAX_AGE":      "1h30m",
			}),
			want: Config{Port: 8080, LogLevel: slog.LevelInfo, WebDir: DefaultWebDir, OIDC: OIDC{
				IssuerURL:    "http://localhost:9443/oauth2/token",
				ClientID:     "meurpg",
				ClientSecret: "s3cret",
				RedirectURL:  "http://127.0.0.1:8080/auth/callback",
				CAFile:       "/certs/local-idp.pem",
				MaxAge:       90 * time.Minute,
			}},
		},
		{
			// The local stack's issuer: the browser and the API container
			// both reach the development provider by this name.
			name: "oidc over plain http on a *.localhost name",
			env: withOIDC(map[string]string{
				"OIDC_ISSUER":       "http://idp.localhost:9090",
				"OIDC_REDIRECT_URL": "http://localhost:8080/auth/callback",
			}),
			want: Config{Port: 8080, LogLevel: slog.LevelInfo, WebDir: DefaultWebDir, OIDC: OIDC{
				IssuerURL:    "http://idp.localhost:9090",
				ClientID:     "meurpg",
				ClientSecret: "s3cret",
				RedirectURL:  "http://localhost:8080/auth/callback",
			}},
		},
		{
			name:    "oidc issuer over plain http on a name that only contains localhost",
			env:     withOIDC(map[string]string{"OIDC_ISSUER": "http://localhost.example.com"}),
			wantErr: []string{"OIDC_ISSUER must use https"},
		},
		{
			name:    "oidc issuer over plain http on a name that only ends in localhost",
			env:     withOIDC(map[string]string{"OIDC_ISSUER": "http://evillocalhost"}),
			wantErr: []string{"OIDC_ISSUER must use https"},
		},
		{
			name: "on Cloud Run",
			env:  map[string]string{"K_SERVICE": "meurpg-api"},
			want: Config{Port: 8080, LogLevel: slog.LevelInfo, WebDir: DefaultWebDir, CloudRun: true},
		},
		{
			name:    "oidc max_age is not a duration",
			env:     withOIDC(map[string]string{"OIDC_MAX_AGE": "3600"}),
			wantErr: []string{"OIDC_MAX_AGE must be a duration"},
		},
		{
			name:    "oidc max_age longer than a session",
			env:     withOIDC(map[string]string{"OIDC_MAX_AGE": "721h"}),
			wantErr: []string{"OIDC_MAX_AGE must be between 1s and 720h"},
		},
		{
			name:    "oidc max_age alone",
			env:     map[string]string{"OIDC_MAX_AGE": "1h"},
			wantErr: []string{"missing OIDC_ISSUER"},
		},
		{
			name:    "oidc partly configured",
			env:     map[string]string{"OIDC_ISSUER": "https://idp.example.com", "OIDC_CLIENT_ID": "meurpg"},
			wantErr: []string{"missing OIDC_CLIENT_SECRET, OIDC_REDIRECT_URL"},
		},
		{
			name:    "oidc CA file alone",
			env:     map[string]string{"OIDC_CA_FILE": "/certs/local-idp.pem"},
			wantErr: []string{"missing OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, OIDC_REDIRECT_URL"},
		},
		{
			name:    "oidc issuer over plain http on a real host",
			env:     withOIDC(map[string]string{"OIDC_ISSUER": "http://idp.example.com"}),
			wantErr: []string{"OIDC_ISSUER must use https"},
		},
		{
			name:    "oidc issuer with a query",
			env:     withOIDC(map[string]string{"OIDC_ISSUER": "https://idp.example.com?tenant=x"}),
			wantErr: []string{"OIDC_ISSUER must not have a query"},
		},
		{
			name:    "oidc issuer is not a URL",
			env:     withOIDC(map[string]string{"OIDC_ISSUER": "idp.example.com"}),
			wantErr: []string{"OIDC_ISSUER must be an absolute URL"},
		},
		{
			name:    "oidc redirect URL on another path",
			env:     withOIDC(map[string]string{"OIDC_REDIRECT_URL": "https://meurpg.example.com/callback"}),
			wantErr: []string{"OIDC_REDIRECT_URL must end in /auth/callback"},
		},
		{
			name:    "oidc redirect URL over plain http on a real host",
			env:     withOIDC(map[string]string{"OIDC_REDIRECT_URL": "http://meurpg.example.com/auth/callback"}),
			wantErr: []string{"OIDC_REDIRECT_URL must use https"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Load(env(tt.env))

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Load() error = nil, want an error containing %q", tt.wantErr)
				}
				for _, sub := range tt.wantErr {
					if !strings.Contains(err.Error(), sub) {
						t.Errorf("Load() error = %q, want it to contain %q", err, sub)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// withOIDC returns a complete, valid OIDC environment with overrides applied.
func withOIDC(overrides map[string]string) map[string]string {
	vars := map[string]string{
		"OIDC_ISSUER":        "https://idp.example.com",
		"OIDC_CLIENT_ID":     "meurpg",
		"OIDC_CLIENT_SECRET": "s3cret",
		"OIDC_REDIRECT_URL":  "https://meurpg.example.com/auth/callback",
	}
	for k, v := range overrides {
		vars[k] = v
	}
	return vars
}

func TestSecretNeverPrintsItself(t *testing.T) {
	t.Parallel()

	cfg := OIDC{ClientID: "meurpg", ClientSecret: "s3cret"}

	var logs bytes.Buffer
	slog.New(slog.NewJSONHandler(&logs, nil)).Info("config", "oidc", cfg, "secret", cfg.ClientSecret)

	printed := []string{
		fmt.Sprint(cfg.ClientSecret),
		fmt.Sprintf("%v %+v %#v %s %q", cfg, cfg, cfg, cfg.ClientSecret, cfg.ClientSecret),
		logs.String(),
	}
	for _, out := range printed {
		if strings.Contains(out, "s3cret") {
			t.Errorf("secret leaked: %s", out)
		}
	}
	if got := cfg.ClientSecret.Reveal(); got != "s3cret" {
		t.Errorf("Reveal() = %q, want the secret", got)
	}
	if got := Secret("").String(); got != "" {
		t.Errorf("empty Secret prints %q, want an empty string", got)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"localhost":             true,
		"LOCALHOST":             true,
		"localhost.":            true,
		"idp.localhost":         true,
		"a.b.localhost":         true,
		"IdP.LocalHost":         true,
		"127.0.0.1":             true,
		"127.1.2.3":             true,
		"::1":                   true,
		"":                      false,
		"localhost.example.com": false,
		"evillocalhost":         false,
		"example.com":           false,
		"10.0.0.1":              false,
		"0.0.0.0":               false,
		"::":                    false,
		"::ffff:10.0.0.1":       false,
	}
	for host, want := range tests {
		if got := IsLoopbackHost(host); got != want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestLoadImages(t *testing.T) {
	t.Parallel()
	cfg, err := Load(env(map[string]string{"GEMINI_API_KEY": " abc-not-a-real-key ", "GEMINI_IMAGE_MODEL": "m", "IMAGE_MONTHLY_LIMIT": "7"}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Images.GeminiAPIKey.Reveal() != "abc-not-a-real-key" || cfg.Images.Model != "m" || cfg.Images.MonthlyLimit != 7 || cfg.Images.Fake {
		t.Errorf("Images = %#v", cfg.Images)
	}
	// The key never prints: not with %v, %+v, %#v, nor as JSON or in a log value.
	for _, shown := range []string{fmt.Sprintf("%v", cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg)} {
		if strings.Contains(shown, "abc-not-a-real-key") {
			t.Errorf("the key leaked into %q", shown)
		}
	}
	if b, _ := json.Marshal(cfg.Images); strings.Contains(string(b), "abc-not-a-real-key") {
		t.Errorf("the key leaked into JSON %s", b)
	}
	var out bytes.Buffer
	slog.New(slog.NewJSONHandler(&out, nil)).Info("config", "images", cfg.Images, "key", cfg.Images.GeminiAPIKey)
	if strings.Contains(out.String(), "abc-not-a-real-key") {
		t.Errorf("the key leaked into the log: %s", out.String())
	}

	def, err := Load(env(nil))
	if err != nil || def.Images.MonthlyLimit != 0 || def.Images.GeminiAPIKey != "" {
		t.Errorf("defaults: %#v, %v", def.Images, err)
	}
	for name, vars := range map[string]map[string]string{
		"a bad limit":         {"IMAGE_MONTHLY_LIMIT": "0"},
		"a huge limit":        {"IMAGE_MONTHLY_LIMIT": "100000"},
		"a bad generator":     {"IMAGE_GENERATOR": "dall-e"},
		"the fake on a cloud": {"IMAGE_GENERATOR": "fake", "K_SERVICE": "api"},
	} {
		if _, err := Load(env(vars)); err == nil {
			t.Errorf("%s: Load() error = nil", name)
		}
	}
	if fake, err := Load(env(map[string]string{"IMAGE_GENERATOR": "fake"})); err != nil || !fake.Images.Fake {
		t.Errorf("the fake: %v, %v", fake.Images, err)
	}
}

func TestTraceProjectOnlyOnCloudRun(t *testing.T) {
	t.Parallel()

	local, err := Load(env(map[string]string{"GOOGLE_CLOUD_PROJECT": "my-proj"}))
	if err != nil || local.TraceProject != "" {
		t.Errorf("off Cloud Run: project %q, err %v; want empty", local.TraceProject, err)
	}
	run, err := Load(env(map[string]string{"K_SERVICE": "meurpg", "GOOGLE_CLOUD_PROJECT": "my-proj"}))
	if err != nil || run.TraceProject != "my-proj" {
		t.Errorf("on Cloud Run: project %q, err %v; want my-proj", run.TraceProject, err)
	}
}
