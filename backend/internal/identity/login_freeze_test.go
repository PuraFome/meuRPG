package identity

import (
	"net/http"
	"strings"
	"testing"
)

// A state that matches the cookie but is not one this server could have
// issued is refused as malformed, before any database access.
func TestCallbackRefusesAMalformedStateBeforeTheStore(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	callbackURL, _ := h.beginLogin("/")

	const malformed = "not-a-state"
	rec := h.finishLogin(withQuery(callbackURL, "state", malformed), reqCookie(loginCookieName, malformed))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("callback status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
	if logs := h.logs.String(); !strings.Contains(logs, `"reason":"malformed_state"`) {
		t.Errorf("logs do not mention malformed_state: %s", logs)
	}
	if n := h.mem.sessionCount(); n != 0 {
		t.Errorf("sessions in the store = %d, want 0", n)
	}
}

// safeReturnTo refuses everything that is not a plain path on this site, and
// drops a fragment before judging the rest.
func TestSafeReturnToRefusalsAreFrozen(t *testing.T) {
	t.Parallel()
	refused := []string{
		"#only-a-fragment",
		"relative/path",
		"//other.example/x",
		"/\\other.example",
		"/a\\b",
		"/tab\tinside",
		"/new\nline",
		"/del\x7f",
		"/%zz",
		"/" + strings.Repeat("a", maxReturnToLength),
	}
	for _, in := range refused {
		if got, ok := safeReturnTo(in); ok || got != "" {
			t.Errorf("safeReturnTo(%q) = %q, %v; want \"\", false", in, got, ok)
		}
	}
	for in, want := range map[string]string{
		"/play#t=secret": "/play",
		"/a?b=c":         "/a?b=c",
		"/":              "/",
		"":               "/",
	} {
		if got, ok := safeReturnTo(in); !ok || got != want {
			t.Errorf("safeReturnTo(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}
}
