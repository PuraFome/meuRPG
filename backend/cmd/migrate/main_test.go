package main

import (
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
)

// A DATABASE_URL pgx cannot parse must not come back in the error: the URL holds
// the password, and the error goes to the log.
func TestRunDoesNotEchoTheDatabaseURL(t *testing.T) {
	t.Parallel()
	cfg := config.Config{DatabaseURL: "postgresql://app:topsecret@host:99999999/db"} //nolint:gosec // G101: a made-up password
	err := run(logging.New(&strings.Builder{}, config.DefaultLogLevel), cfg, []string{"status"})
	if err == nil {
		t.Fatal("run() accepted a broken DATABASE_URL")
	}
	if strings.Contains(err.Error(), "topsecret") || strings.Contains(err.Error(), "host") {
		t.Errorf("the error echoes the URL: %v", err)
	}
}
