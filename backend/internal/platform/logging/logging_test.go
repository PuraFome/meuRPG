package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
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

type otherKey struct{}

func decodeLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("not a JSON line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func TestEveryLineHasServiceAndVersion(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo, WithVersion("v1.2.3"))
	logger.Info("no context at all")

	line := decodeLines(t, &buf)[0]
	if line["service"] != "meurpg-api" || line["version"] != "v1.2.3" {
		t.Errorf("service/version = %v/%v", line["service"], line["version"])
	}
	for _, key := range []string{"request_id", "user_id", "campaign_id"} {
		if _, ok := line[key]; ok {
			t.Errorf("%s present without a request", key)
		}
	}
}

func TestFieldsComeFromTheRequestContext(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelDebug)

	ctx := WithRequest(context.Background(), "0123456789abcdef0123456789abcdef", "projects/p/traces/abc")
	// Set on a derived context, as an inner interceptor does: the outer
	// context must see it.
	inner := context.WithValue(ctx, otherKey{}, 1)
	SetUserID(inner, "11111111-1111-1111-1111-111111111111")
	SetCampaignID(inner, "22222222-2222-2222-2222-222222222222")

	logger.InfoContext(ctx, "rpc")

	line := decodeLines(t, &buf)[0]
	want := map[string]any{
		"request_id":  "0123456789abcdef0123456789abcdef",
		"user_id":     "11111111-1111-1111-1111-111111111111",
		"campaign_id": "22222222-2222-2222-2222-222222222222",
		TraceKey:      "projects/p/traces/abc",
	}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("%s = %v, want %v", k, line[k], v)
		}
	}
	if RequestID(ctx) != "0123456789abcdef0123456789abcdef" || UserID(ctx) == "" || CampaignID(ctx) == "" {
		t.Error("getters do not read back what was set")
	}
}

func TestExplicitKeyWinsOverTheContext(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelDebug)
	ctx := WithRequest(context.Background(), "r", "")
	SetCampaignID(ctx, "from-context")

	logger.InfoContext(ctx, "event", "campaign_id", "explicit")

	raw := buf.String()
	if strings.Count(raw, `"campaign_id"`) != 1 || !strings.Contains(raw, `"explicit"`) {
		t.Errorf("want exactly one explicit campaign_id, got %s", raw)
	}
}

func TestSettersWithoutARequestDoNothing(t *testing.T) {
	t.Parallel()
	SetUserID(context.Background(), "x") // must not panic
	if UserID(context.Background()) != "" {
		t.Error("a context without a request has no user")
	}
}

func TestDenyListDropsKeysAtAnyDepth(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelDebug)

	logger.Info("x",
		"email", "a@b.c", "Token", "t", "password", "p", "secret", "s", "cookie", "c",
		"authorization", "a", "prompt", "pr", "body", "b", "text", "tx", "name", "n", "display_name", "d",
		slog.Group("g", slog.String("name", "inner"), slog.Group("h", slog.String("email", "deep"), slog.Int("ok", 1))),
		slog.Any("m", map[string]any{"token": "in-a-map", "list": []any{map[string]any{"text": "in-a-list"}}, "id": "kept"}),
		"character_id", "kept-id",
	)
	logger.With("name", "bound").WithGroup("grp").Info("y", "email", "e")

	raw := buf.String()
	for _, secret := range []string{"a@b.c", `"t"`, "inner", "deep", "in-a-map", "in-a-list", "bound", `"e"`, `"n"`, `"d"`} {
		if strings.Contains(raw, secret) {
			t.Errorf("the log leaks %s: %s", secret, raw)
		}
	}
	for _, kept := range []string{"kept-id", `"ok":1`, `"id":"kept"`} {
		if !strings.Contains(raw, kept) {
			t.Errorf("the log lost %s: %s", kept, raw)
		}
	}
}

func TestEventIsDebugAndOneLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	ctx := WithRequest(context.Background(), "r", "")

	Event(ctx, New(&buf, slog.LevelInfo), "combat.started", slog.String("combat_id", "c"))
	if buf.Len() != 0 {
		t.Fatalf("an event must not be written at INFO: %s", buf.String())
	}
	Event(ctx, New(&buf, slog.LevelDebug), "combat.started", slog.String("combat_id", "c"))
	line := decodeLines(t, &buf)[0]
	if line["message"] != "event" || line["event"] != "combat.started" || line["severity"] != "DEBUG" || line["combat_id"] != "c" {
		t.Errorf("event line = %v", line)
	}
	Event(ctx, nil, "x") // a nil logger must not panic
}

func TestRequestIDAndTrace(t *testing.T) {
	t.Parallel()
	id := NewRequestID()
	if len(id) != 32 || strings.Trim(id, "0123456789abcdef") != "" {
		t.Errorf("request id %q is not 32 lowercase hex chars", id)
	}
	if NewRequestID() == id {
		t.Error("two request ids are equal")
	}
	trace := "105445aa7843bc8bf206b12000100000"
	if got := TraceFromHeader("proj", trace+"/1;o=1"); got != "projects/proj/traces/"+trace {
		t.Errorf("trace = %q", got)
	}
	for _, bad := range [][2]string{{"", trace + "/1"}, {"proj", ""}, {"proj", "short/1"}, {"proj", strings.Repeat("z", 32) + "/1"}} {
		if got := TraceFromHeader(bad[0], bad[1]); got != "" {
			t.Errorf("TraceFromHeader(%q, %q) = %q, want empty", bad[0], bad[1], got)
		}
	}
}
