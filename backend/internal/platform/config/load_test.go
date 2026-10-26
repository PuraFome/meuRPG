package config

import (
	"maps"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/platform/goldentest"
)

// loadOutcome is what Load gives for one environment: the exact error, or the
// configuration with its secrets revealed (Config's JSON redacts them).
type loadOutcome struct {
	Env     map[string]string `json:"env"`
	Error   string            `json:"error,omitempty"`
	Config  *Config           `json:"config,omitempty"`
	Secrets map[string]string `json:"secrets,omitempty"`
}

// loadCorpus is one environment per branch of Load, loadOIDC and loadLimits,
// plus environments with several mistakes, which pin the order of the errors.
func loadCorpus() map[string]map[string]string {
	cloud := func(vars map[string]string) map[string]string {
		out := map[string]string{"K_SERVICE": "api"}
		maps.Copy(out, vars)
		return out
	}
	return map[string]map[string]string{
		"empty": {},

		"port text":         {"PORT": "abc"},
		"port zero":         {"PORT": "0"},
		"port too big":      {"PORT": "65536"},
		"port lowest":       {"PORT": "1"},
		"port highest":      {"PORT": " 65535 "},
		"listen host":       {"LISTEN_HOST": " 127.0.0.1 "},
		"web dir":           {"WEB_DIR": " /srv/web "},
		"web dir blank":     {"WEB_DIR": "   "},
		"blob dir":          {"BLOB_DIR": " /data/blobs "},
		"blob bucket":       {"BLOB_BUCKET": "meurpg-images"},
		"blob both":         {"BLOB_DIR": "/data", "BLOB_BUCKET": "b"},
		"blob both cloud":   cloud(map[string]string{"BLOB_DIR": "/data", "BLOB_BUCKET": "b"}),
		"blob dir cloud":    cloud(map[string]string{"BLOB_DIR": "/data"}),
		"blob bucket cloud": cloud(map[string]string{"BLOB_BUCKET": "b"}),

		"tasks none": {},
		"tasks all": {
			"TASKS_QUEUE": " projects/demo-project/locations/southamerica-east1/queues/demo-tasks ", "TASKS_TARGET_URL": "https://demo-api-abc123-rj.a.run.app",
			"TASKS_SERVICE_ACCOUNT": "demo-api@demo-project.iam.gserviceaccount.com",
		},
		"tasks all cloud": cloud(map[string]string{
			"TASKS_QUEUE": "projects/demo-project/locations/southamerica-east1/queues/demo-tasks", "TASKS_TARGET_URL": "https://demo-api-abc123-rj.a.run.app/",
			"TASKS_SERVICE_ACCOUNT": "demo-api@demo-project.iam.gserviceaccount.com",
		}),
		"tasks local target": {
			"TASKS_QUEUE": "projects/demo-project/locations/southamerica-east1/queues/demo-tasks", "TASKS_TARGET_URL": "http://localhost:8080",
			"TASKS_SERVICE_ACCOUNT": "demo-api@demo-project.iam.gserviceaccount.com",
		},
		"tasks queue only":   {"TASKS_QUEUE": "projects/demo-project/locations/southamerica-east1/queues/demo-tasks"},
		"tasks target only":  {"TASKS_TARGET_URL": "https://demo-api-abc123-rj.a.run.app"},
		"tasks account only": {"TASKS_SERVICE_ACCOUNT": "demo-api@demo-project.iam.gserviceaccount.com"},
		"tasks no account": {
			"TASKS_QUEUE": "projects/demo-project/locations/southamerica-east1/queues/demo-tasks", "TASKS_TARGET_URL": "https://demo-api-abc123-rj.a.run.app",
		},
		"tasks bad queue": {"TASKS_QUEUE": "demo-tasks", "TASKS_TARGET_URL": "https://demo-api-abc123-rj.a.run.app", "TASKS_SERVICE_ACCOUNT": "demo-api@demo-project.iam.gserviceaccount.com"},
		"tasks http target": {
			"TASKS_QUEUE": "projects/demo-project/locations/southamerica-east1/queues/demo-tasks", "TASKS_TARGET_URL": "http://demo-api.run.app",
			"TASKS_SERVICE_ACCOUNT": "demo-api@demo-project.iam.gserviceaccount.com",
		},
		"tasks target path": {
			"TASKS_QUEUE": "projects/demo-project/locations/southamerica-east1/queues/demo-tasks", "TASKS_TARGET_URL": "https://demo-api.run.app/api?x=1",
			"TASKS_SERVICE_ACCOUNT": "demo-api@demo-project.iam.gserviceaccount.com",
		},
		"tasks target not a url": {
			"TASKS_QUEUE": "projects/demo-project/locations/southamerica-east1/queues/demo-tasks", "TASKS_TARGET_URL": "demo-api",
			"TASKS_SERVICE_ACCOUNT": "demo-api@demo-project.iam.gserviceaccount.com",
		},
		"tasks account not an email": {
			"TASKS_QUEUE": "projects/demo-project/locations/southamerica-east1/queues/demo-tasks", "TASKS_TARGET_URL": "https://demo-api.run.app",
			"TASKS_SERVICE_ACCOUNT": "demo-api",
		},

		"images fake":          {"IMAGE_GENERATOR": "fake"},
		"images fake cloud":    cloud(map[string]string{"IMAGE_GENERATOR": "fake"}),
		"images unknown":       {"IMAGE_GENERATOR": "dalle"},
		"images key and model": {"GEMINI_API_KEY": "k3y", "GEMINI_IMAGE_MODEL": "m", "IMAGE_GENERATOR": "gemini"},
		"images limits":        {"IMAGE_MONTHLY_LIMIT": "500", "IMAGE_DAILY_LIMIT": "10000"},
		"images limits bad":    {"IMAGE_MONTHLY_LIMIT": "x", "IMAGE_DAILY_LIMIT": "y"},
		"images limits range":  {"IMAGE_MONTHLY_LIMIT": "501", "IMAGE_DAILY_LIMIT": "0"},

		"log debug":   {"LOG_LEVEL": "debug"},
		"log warn up": {"LOG_LEVEL": "WARN"},
		"log error":   {"LOG_LEVEL": "error"},
		"log info":    {"LOG_LEVEL": "Info"},
		"log bad":     {"LOG_LEVEL": "trace"},

		"database local plain":     {"DATABASE_URL": " postgresql://root@db:26257/meurpg?sslmode=disable "},
		"database cloud plain":     cloud(map[string]string{"DATABASE_URL": "postgresql://u@db:26257/meurpg?sslmode=disable"}),
		"database cloud require":   cloud(map[string]string{"DATABASE_URL": "postgresql://u@db:26257/meurpg?sslmode=require"}),
		"database cloud verify":    cloud(map[string]string{"DATABASE_URL": "postgresql://u@db:26257/meurpg?sslmode=verify-full"}),
		"database cloud verify-ca": cloud(map[string]string{"DATABASE_URL": "postgresql://u@db:26257/meurpg?sslmode=verify-ca"}),
		"database cloud broken":    cloud(map[string]string{"DATABASE_URL": "postgresql://u@db:99999999/meurpg"}),

		"cloud trace":          cloud(map[string]string{"GOOGLE_CLOUD_PROJECT": " proj "}),
		"local trace ignored":  {"GOOGLE_CLOUD_PROJECT": "proj"},
		"hops local":           {"TRUSTED_PROXY_HOPS": "1"},
		"hops cloud two":       cloud(map[string]string{"TRUSTED_PROXY_HOPS": "2"}),
		"hops cloud bad":       cloud(map[string]string{"TRUSTED_PROXY_HOPS": "3"}),
		"hops error first":     {"TRUSTED_PROXY_HOPS": "1", "PORT": "x"},
		"limits all":           {"RATE_LIMIT_MULTIPLIER": "2.5", "MAX_CAMPAIGNS_PER_USER": "1000", "CAMPAIGN_CREATORS": " A@x.com,b@x.com ,a@X.com,"},
		"limits nan":           {"RATE_LIMIT_MULTIPLIER": "NaN"},
		"limits inf":           {"RATE_LIMIT_MULTIPLIER": "inf"},
		"limits negative":      {"RATE_LIMIT_MULTIPLIER": "-1"},
		"limits zero":          {"RATE_LIMIT_MULTIPLIER": "0"},
		"limits max":           {"RATE_LIMIT_MULTIPLIER": "1000"},
		"limits over max":      {"RATE_LIMIT_MULTIPLIER": "1000.5"},
		"cap off":              {"MAX_CAMPAIGNS_PER_USER": "OFF"},
		"cap off cloud":        cloud(map[string]string{"MAX_CAMPAIGNS_PER_USER": "off"}),
		"cap text":             {"MAX_CAMPAIGNS_PER_USER": "many"},
		"cap zero":             {"MAX_CAMPAIGNS_PER_USER": "0"},
		"cap too big":          {"MAX_CAMPAIGNS_PER_USER": "1001"},
		"creators bad entries": {"CAMPAIGN_CREATORS": "a@x.com, mestre ,, other"},
		"creators separators":  {"CAMPAIGN_CREATORS": " , ,"},
		// The "no e-mail address" check looks at every error
		// loadLimits has collected so far, not only the list's, so a bad
		// multiplier hides it (the bad multiplier still fails the load).
		"creators separators after a bad multiplier": {"RATE_LIMIT_MULTIPLIER": "x", "CAMPAIGN_CREATORS": ","},
		"limits every error":                         {"RATE_LIMIT_MULTIPLIER": "x", "MAX_CAMPAIGNS_PER_USER": "y", "CAMPAIGN_CREATORS": "z"},

		"idle text":     {"SESSION_IDLE_TIMEOUT": "two weeks"},
		"idle short":    {"SESSION_IDLE_TIMEOUT": "59m"},
		"idle shortest": {"SESSION_IDLE_TIMEOUT": "1h"},
		"idle longest":  {"SESSION_IDLE_TIMEOUT": "720h"},
		"idle long":     {"SESSION_IDLE_TIMEOUT": "721h"},

		"oidc full":            withOIDC(nil),
		"oidc full extras":     withOIDC(map[string]string{"OIDC_CA_FILE": " /ca.pem ", "OIDC_MAX_AGE": "1.5s"}),
		"oidc max age alone":   {"OIDC_MAX_AGE": "1h"},
		"oidc ca alone":        {"OIDC_CA_FILE": "/ca.pem"},
		"oidc client id alone": {"OIDC_CLIENT_ID": "x"},
		"oidc secret alone":    {"OIDC_CLIENT_SECRET": "x"},
		"oidc max age text":    withOIDC(map[string]string{"OIDC_MAX_AGE": "soon"}),
		"oidc max age short":   withOIDC(map[string]string{"OIDC_MAX_AGE": "999ms"}),
		"oidc max age 1s":      withOIDC(map[string]string{"OIDC_MAX_AGE": "1s"}),
		"oidc max age 720h":    withOIDC(map[string]string{"OIDC_MAX_AGE": "720h"}),
		"oidc max age long":    withOIDC(map[string]string{"OIDC_MAX_AGE": "721h"}),
		"oidc issuer not url":  withOIDC(map[string]string{"OIDC_ISSUER": "://x"}),
		"oidc issuer relative": withOIDC(map[string]string{"OIDC_ISSUER": "/idp"}),
		"oidc issuer user":     withOIDC(map[string]string{"OIDC_ISSUER": "https://me@idp.example.com"}),
		"oidc issuer http":     withOIDC(map[string]string{"OIDC_ISSUER": "http://idp.example.com/x?q=1"}),
		"oidc issuer ftp":      withOIDC(map[string]string{"OIDC_ISSUER": "ftp://idp.example.com"}),
		"oidc issuer loopback": withOIDC(map[string]string{"OIDC_ISSUER": "http://127.0.0.1:9090"}),
		"oidc issuer query":    withOIDC(map[string]string{"OIDC_ISSUER": "https://idp.example.com/p?token=t"}),
		"oidc issuer fragment": withOIDC(map[string]string{"OIDC_ISSUER": "https://idp.example.com/p#frag"}),
		"oidc redirect bad":    withOIDC(map[string]string{"OIDC_REDIRECT_URL": "meurpg"}),
		"oidc redirect http":   withOIDC(map[string]string{"OIDC_REDIRECT_URL": "http://meurpg.example.com/auth/callback"}),
		"oidc redirect path":   withOIDC(map[string]string{"OIDC_REDIRECT_URL": "https://meurpg.example.com/cb"}),
		"oidc redirect query":  withOIDC(map[string]string{"OIDC_REDIRECT_URL": "https://meurpg.example.com/auth/callback?x=1"}),
		"oidc redirect frag":   withOIDC(map[string]string{"OIDC_REDIRECT_URL": "https://meurpg.example.com/auth/callback#x"}),
		"oidc every error": {
			"OIDC_ISSUER": "http://idp.example.com", "OIDC_REDIRECT_URL": "https://x/cb", "OIDC_MAX_AGE": "0s",
		},

		"everything wrong locally": {
			"TRUSTED_PROXY_HOPS": "2", "PORT": "-1", "BLOB_DIR": "/d", "BLOB_BUCKET": "b",
			"IMAGE_GENERATOR": "x", "IMAGE_MONTHLY_LIMIT": "0", "IMAGE_DAILY_LIMIT": "z", "LOG_LEVEL": "loud",
			"RATE_LIMIT_MULTIPLIER": "0", "MAX_CAMPAIGNS_PER_USER": "0", "CAMPAIGN_CREATORS": "nobody",
			"SESSION_IDLE_TIMEOUT": "1s", "OIDC_ISSUER": "https://idp.example.com?x", "OIDC_MAX_AGE": "x",
		},
		"everything wrong on cloud run": cloud(map[string]string{
			"TRUSTED_PROXY_HOPS": "9", "PORT": "x", "BLOB_DIR": "/d", "IMAGE_GENERATOR": "fake", "LOG_LEVEL": "x",
			"DATABASE_URL": "postgresql://db/x?sslmode=prefer", "MAX_CAMPAIGNS_PER_USER": "off",
			"SESSION_IDLE_TIMEOUT": "x", "OIDC_REDIRECT_URL": "x",
		}),
		"everything set on cloud run": cloud(map[string]string{
			"TRUSTED_PROXY_HOPS": "2", "PORT": "9090", "LISTEN_HOST": "0.0.0.0", "WEB_DIR": "/w", "BLOB_BUCKET": "b",
			"GEMINI_API_KEY": "k", "IMAGE_MONTHLY_LIMIT": "3", "IMAGE_DAILY_LIMIT": "4", "LOG_LEVEL": "debug",
			"DATABASE_URL":          "postgresql://u@db/x?sslmode=verify-full",
			"RATE_LIMIT_MULTIPLIER": "3", "MAX_CAMPAIGNS_PER_USER": "5", "CAMPAIGN_CREATORS": "m@x.com",
			"SESSION_IDLE_TIMEOUT": "48h", "GOOGLE_CLOUD_PROJECT": "p",
			"OIDC_ISSUER": "https://accounts.google.com", "OIDC_CLIENT_ID": "id", "OIDC_CLIENT_SECRET": "s",
			"OIDC_REDIRECT_URL": "https://meurpg.example.com/auth/callback",
		}),
	}
}

// TestLoadEnvironments: for every environment of loadCorpus, the exact error
// (every message, in order) or the exact configuration Load gives, in
// testdata/golden/load.json.
func TestLoadEnvironments(t *testing.T) {
	t.Parallel()
	got := map[string]loadOutcome{}
	for name, vars := range loadCorpus() {
		cfg, err := Load(env(vars))
		out := loadOutcome{Env: vars}
		if err != nil {
			out.Error = err.Error()
			if cfg.Port != 0 || cfg.DatabaseURL != "" || cfg.OIDC != (OIDC{}) {
				t.Errorf("%s: Load() returned a configuration with its error: %+v", name, cfg)
			}
		} else {
			out.Config = &cfg
			out.Secrets = map[string]string{
				"DatabaseURL":       cfg.DatabaseURL.Reveal(),
				"OIDC.ClientSecret": cfg.OIDC.ClientSecret.Reveal(),
				"Images.GeminiKey":  cfg.Images.GeminiAPIKey.Reveal(),
			}
		}
		got[name] = out
	}
	goldentest.JSON(t, "testdata/golden/load.json", got)
}
