package safego_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/platform/safego"
)

func TestRecoverStopsAPanicAndLogsIt(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	failed := false

	func() {
		defer safego.Recover(logger, "test job", func() { failed = true })
		panic("bad data")
	}()

	if !failed {
		t.Error("onPanic did not run")
	}
	for _, want := range []string{"level=ERROR", "test job", "bad data", "stack="} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log %q misses %q", logs.String(), want)
		}
	}
}

func TestRecoverDoesNothingWithoutAPanic(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	func() { defer safego.Recover(slog.New(slog.NewTextHandler(&logs, nil)), "calm job") }()
	if logs.Len() != 0 {
		t.Errorf("logged %q without a panic", logs.String())
	}
}
