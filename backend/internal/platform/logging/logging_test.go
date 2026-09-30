package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNewUsesCloudLoggingKeys(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	logger.Debug("hidden") // below the level, must not be written
	logger.Warn("disk almost full", "free_mb", 12)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("expected exactly one JSON line, got %q: %v", buf.String(), err)
	}

	want := map[string]any{
		"severity": "WARNING",
		"message":  "disk almost full",
		"free_mb":  float64(12), // JSON numbers decode as float64
	}
	for key, value := range want {
		if entry[key] != value {
			t.Errorf("entry[%q] = %v, want %v", key, entry[key], value)
		}
	}
	for _, key := range []string{"level", "msg"} {
		if _, found := entry[key]; found {
			t.Errorf("entry has key %q, want it renamed", key)
		}
	}
}
