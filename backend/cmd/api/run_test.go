package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
	"github.com/PuraFome/meuRPG/backend/internal/platform/goldentest"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

// runLog is the log of one run: it sends the process SIGTERM once the HTTP
// server has started, which is how Cloud Run stops the container, so run
// goes through its graceful shutdown and returns.
type runLog struct {
	mu      sync.Mutex
	buf     strings.Builder
	stopped bool
}

func (l *runLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Write(p)
	if !l.stopped && strings.Contains(string(p), `"message":"http server started"`) {
		l.stopped = true
		// run's signal.NotifyContext is listening by now: the signal cancels
		// its context instead of killing the test binary.
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}
	return len(p), nil
}

// lines is the log's warnings and errors, one record per line, without the
// time and the static attributes, with what depends on the machine (the port,
// the content revision, the wrapped errors) masked. The INFO lines are
// progress, not behavior.
func (l *runLog) lines(t *testing.T) []string {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	out := []string{}
	sc := bufio.NewScanner(strings.NewReader(l.buf.String()))
	for sc.Scan() {
		dec := json.NewDecoder(strings.NewReader(sc.Text()))
		if _, err := dec.Token(); err != nil {
			t.Fatalf("log line %q: %v", sc.Text(), err)
		}
		var parts []string
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				t.Fatalf("log line %q: %v", sc.Text(), err)
			}
			key, _ := keyTok.(string)
			var value any
			if err := dec.Decode(&value); err != nil {
				t.Fatalf("log line %q: %v", sc.Text(), err)
			}
			switch key {
			case "time", "service", "version", "logging.googleapis.com/sourceLocation":
				continue
			case "addr", "dir", "content_version", "error":
				// The content revision moves with every content change, and the
				// errors wrap what the OS, the driver or the database answered:
				// the message is ours, those values are the environment's.
				value = "*"
			}
			parts = append(parts, fmt.Sprintf("%s=%v", key, value))
		}
		if line := strings.Join(parts, " "); !strings.HasPrefix(line, "severity=INFO ") {
			out = append(out, line)
		}
	}
	return out
}

// tempDirName is the folder name t.TempDir gives this test, which changes
// from run to run.
var tempDirName = regexp.MustCompile(`TestRunConfigurations\d+`)

// maskRunError hides what an error owes to the machine rather than to run:
// the temporary folders, and why the metadata server could not be reached
// (no such host here, a timeout elsewhere).
func maskRunError(msg string) string {
	const metadata = "blob: cloud storage access token: "
	if strings.HasPrefix(msg, metadata) {
		return metadata + "*"
	}
	msg = strings.ReplaceAll(msg, os.TempDir(), "$TMPDIR")
	return tempDirName.ReplaceAllString(msg, "TestRunConfigurations*")
}

// runOutcome is what one run gives: its error and its log.
type runOutcome struct {
	Error string   `json:"error,omitempty"`
	Log   []string `json:"log"`
}

// runCase is one configuration of run, built when its test runs (some need
// a folder or a database).
type runCase struct {
	name string
	cfg  func(t *testing.T) config.Config
}

// runCases are the configurations that take run down each of its branches:
// each refusal before the server starts (a missing DATABASE_URL among them),
// and the servers it builds with a database that cannot be reached and with a
// real one.
func runCases() []runCase {
	listen := func(cfg config.Config) config.Config {
		cfg.ListenHost, cfg.Port, cfg.LogLevel = "127.0.0.1", 0, config.DefaultLogLevel
		return cfg
	}
	web := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html>"), 0o600); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	oidc := config.OIDC{
		IssuerURL: "http://localhost:1", ClientID: "meurpg", ClientSecret: "s",
		RedirectURL: "http://localhost:8080/auth/callback", MaxAge: time.Hour,
	}
	unreachable := config.Secret("postgresql://root@127.0.0.1:1/none?sslmode=disable&connect_timeout=5")
	return []runCase{
		{"broken database URL", func(*testing.T) config.Config {
			return listen(config.Config{DatabaseURL: "postgresql://h:99999999/x"})
		}},
		{"invalid bucket", func(*testing.T) config.Config {
			return listen(config.Config{DatabaseURL: unreachable, BlobBucket: "No Such Bucket", WebDir: "/nonexistent"})
		}},
		{"blob dir under a file", func(t *testing.T) config.Config {
			t.Helper()
			file := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(file, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			return listen(config.Config{DatabaseURL: unreachable, BlobDir: filepath.Join(file, "blobs"), WebDir: "/nonexistent"})
		}},
		{"bad form-action origin", func(t *testing.T) config.Config {
			t.Helper()
			return listen(config.Config{DatabaseURL: unreachable, OIDC: config.OIDC{IssuerURL: "ftp://idp"}, WebDir: web(t)})
		}},
		{"bad CA file", func(*testing.T) config.Config {
			bad := oidc
			bad.CAFile = "/nonexistent/ca.pem"
			return listen(config.Config{DatabaseURL: unreachable, OIDC: bad, WebDir: "/nonexistent"})
		}},
		{"no database URL", func(*testing.T) config.Config {
			return listen(config.Config{OIDC: oidc, BlobDir: "/nonexistent", WebDir: "/nonexistent"})
		}},
		{"unreachable database, no sign-in, a blob dir", func(t *testing.T) config.Config {
			t.Helper()
			return listen(config.Config{
				DatabaseURL: unreachable, BlobDir: t.TempDir(), WebDir: web(t), TrustedProxyHops: 1, TraceProject: "p",
				Images: config.Images{Fake: true},
			})
		}},
		{"unreachable database", func(*testing.T) config.Config {
			return listen(config.Config{
				DatabaseURL: unreachable, BlobBucket: "meurpg-images", WebDir: "/nonexistent",
				Images: config.Images{GeminiAPIKey: "k3y", Model: "some-model"},
			})
		}},
		{"unreachable database with sign-in", func(t *testing.T) config.Config {
			t.Helper()
			return listen(config.Config{
				DatabaseURL: unreachable, OIDC: oidc, WebDir: web(t), Images: config.Images{Fake: true, MonthlyLimit: 3},
			})
		}},
		{"database with sign-in", func(t *testing.T) config.Config {
			t.Helper()
			pool := dbtest.NewPool(t, "apirun")
			return listen(config.Config{
				DatabaseURL: config.Secret(pool.Config().ConnString()), OIDC: oidc, WebDir: "/nonexistent",
			})
		}},
	}
}

// TestRunConfigurations: for every configuration of runCases, the error run
// returns and every warning or error it logs, in order, in
// testdata/golden/run.json. Not parallel: the server is stopped with a signal
// to the whole process.
func TestRunConfigurations(t *testing.T) {
	got := map[string]runOutcome{}
	for _, c := range runCases() {
		cfg := c.cfg(t)
		log := &runLog{}
		done := make(chan error, 1)
		go func() { done <- run(logging.New(log, cfg.LogLevel, logging.WithVersion(version)), cfg) }()
		var err error
		select {
		case err = <-done:
		case <-time.After(2 * time.Minute):
			t.Fatalf("%s: run did not return", c.name)
		}
		out := runOutcome{Log: log.lines(t)}
		if err != nil {
			out.Error = maskRunError(err.Error())
		}
		got[c.name] = out
	}
	goldentest.JSON(t, "testdata/golden/run.json", got)
}

// Without DATABASE_URL the process refuses to start, and the error names the variable.
// With one, openDatabase does not refuse: an unreachable database is only a warning.
func TestOpenDatabaseNeedsTheURL(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	if _, err := openDatabase(t.Context(), logger, config.Config{}); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("openDatabase without a URL = %v, want an error naming DATABASE_URL", err)
	}
	pool, err := openDatabase(t.Context(), logger, config.Config{DatabaseURL: "postgresql://root@127.0.0.1:1/none?sslmode=disable&connect_timeout=5"})
	if err != nil {
		t.Fatalf("openDatabase with a URL: %v", err)
	}
	pool.Close()
}
