// Package config reads the process configuration from environment variables.
//
// Following the twelve-factor app style, everything that changes between
// environments (local, CI, Cloud Run) comes from the environment, and the
// defaults are chosen so that `go run ./cmd/api` works with no setup at all.
// The package only uses the standard library on purpose: configuration is
// small enough that a framework would add more concepts than it removes.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// Defaults used when a variable is unset or empty.
const (
	DefaultPort     = 8080
	DefaultLogLevel = slog.LevelInfo
	// DefaultWebDir is where backend/Dockerfile's Node build stage copies
	// the Angular production build.
	DefaultWebDir = "/app/web"
)

// Config is the validated configuration shared by the binaries in cmd/.
type Config struct {
	// Port is the TCP port the HTTP server listens on. Cloud Run injects it
	// through $PORT (usually 8080).
	Port int

	// DatabaseURL is a PostgreSQL connection string for CockroachDB, e.g.
	// postgresql://user:pass@host:26257/meurpg?sslmode=verify-full.
	// Empty means "run without a database": the API still starts, and
	// /readyz reports the database as disabled.
	DatabaseURL string

	// LogLevel is the minimum level written to the logs.
	LogLevel slog.Level

	// WebDir is the directory holding the Angular production build (see
	// internal/platform/httpserver.NewStatic). When nothing exists there,
	// the server logs it and runs API-only instead of failing to start:
	// that is the normal case outside the Docker image, e.g. `make run`.
	WebDir string
}

// Load builds a Config from getenv, which is usually os.Getenv. Taking the
// lookup function as a parameter keeps Load free of global state, so tests
// can pass a fake environment instead of mutating the real one.
//
// All problems are reported together, so a misconfigured deploy shows every
// mistake in a single log line instead of one per restart.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:        DefaultPort,
		DatabaseURL: strings.TrimSpace(getenv("DATABASE_URL")),
		LogLevel:    DefaultLogLevel,
		WebDir:      DefaultWebDir,
	}

	var errs []error

	if raw := strings.TrimSpace(getenv("PORT")); raw != "" {
		port, err := strconv.Atoi(raw)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("PORT must be a number, got %q", raw))
		case port < 1 || port > 65535:
			errs = append(errs, fmt.Errorf("PORT must be between 1 and 65535, got %d", port))
		default:
			cfg.Port = port
		}
	}

	if raw := strings.TrimSpace(getenv("WEB_DIR")); raw != "" {
		cfg.WebDir = raw
	}

	if raw := strings.TrimSpace(getenv("LOG_LEVEL")); raw != "" {
		level, err := parseLogLevel(raw)
		if err != nil {
			errs = append(errs, err)
		} else {
			cfg.LogLevel = level
		}
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

// parseLogLevel accepts the four slog levels, case-insensitively.
func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error; got %q", raw)
	}
}
