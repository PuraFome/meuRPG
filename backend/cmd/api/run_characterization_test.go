package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

var updateRunGolden = flag.Bool("update-run-golden", false, "write testdata/golden/run.json again from the current run")

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

// lines is the log, one record per line, without the time and the static
// attributes, with what changes between runs (the port, durations) masked.
func (l *runLog) lines(t *testing.T) []string {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
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
			case "addr", "dir":
				value = "*"
			case "error":
				value = maskRunError(fmt.Sprint(value))
			}
			parts = append(parts, fmt.Sprintf("%s=%v", key, value))
		}
		out = append(out, strings.Join(parts, " "))
	}
	return out
}

// tempDirName is the folder name t.TempDir gives this test, which changes
// from run to run.
var tempDirName = regexp.MustCompile(`TestRunCharacterization\d+`)

// maskRunError hides what an error owes to the machine rather than to run:
// the temporary folders, and why the metadata server could not be reached
// (no such host here, a timeout elsewhere).
func maskRunError(msg string) string {
	const metadata = "blob: cloud storage access token: "
	if strings.HasPrefix(msg, metadata) {
		return metadata + "*"
	}
	msg = strings.ReplaceAll(msg, os.TempDir(), "$TMPDIR")
	return tempDirName.ReplaceAllString(msg, "TestRunCharacterization*")
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
// each refusal before the server starts, and the servers it builds without a
// database, with one that cannot be reached, and with a real one.
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
			return listen(config.Config{BlobBucket: "No Such Bucket", WebDir: "/nonexistent"})
		}},
		{"blob dir under a file", func(t *testing.T) config.Config {
			t.Helper()
			file := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(file, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			return listen(config.Config{BlobDir: filepath.Join(file, "blobs"), WebDir: "/nonexistent"})
		}},
		{"bad form-action origin", func(t *testing.T) config.Config {
			t.Helper()
			return listen(config.Config{OIDC: config.OIDC{IssuerURL: "ftp://idp"}, WebDir: web(t)})
		}},
		{"bad CA file", func(*testing.T) config.Config {
			bad := oidc
			bad.CAFile = "/nonexistent/ca.pem"
			return listen(config.Config{DatabaseURL: unreachable, OIDC: bad, WebDir: "/nonexistent"})
		}},
		{"nothing configured", func(*testing.T) config.Config {
			return listen(config.Config{WebDir: "/nonexistent"})
		}},
		{"sign-in without a database", func(t *testing.T) config.Config {
			t.Helper()
			return listen(config.Config{
				OIDC: oidc, BlobDir: t.TempDir(), WebDir: web(t), TrustedProxyHops: 1, TraceProject: "p",
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

// TestRunCharacterization freezes, for every configuration of runCases, the
// error run returns and every line it logs, in order, in
// testdata/golden/run.json. Not parallel: the server is stopped with a
// signal to the whole process.
func TestRunCharacterization(t *testing.T) {
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
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join("testdata", "golden", "run.json")
	if *updateRunGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the golden (run with -update-run-golden): %v", err)
	}
	if !bytes.Equal(want, data) {
		t.Errorf("run drifted from %s; diff it after running with -update-run-golden", path)
	}
}
