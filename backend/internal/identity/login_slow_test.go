package identity

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/slowclient"
	"github.com/PuraFome/meuRPG/backend/internal/platform/slowclient/slowtest"
)

// A client that promises a form and trickles it must not hold the connection:
// the body read has a deadline, and the answer is a plain "malformed form".
func TestLoginFormReadHasADeadline(t *testing.T) {
	t.Parallel()
	type result struct {
		le      *loginError
		message string
	}
	done := make(chan result, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slowclient.ReadBody(w, r, 300*time.Millisecond)
		_, le, message := parseLoginForm(w, r)
		done <- result{le, message}
	}))
	t.Cleanup(srv.Close)

	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	// Promises 100 bytes of form, sends 10, then nothing.
	_, _ = conn.Write([]byte("POST /auth/login HTTP/1.1\r\nHost: x\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 100\r\n\r\nreturn_to="))

	select {
	case got := <-done:
		if got.le == nil || got.le.reason != "malformed_form" {
			t.Errorf("parseLoginForm() = %+v, want a malformed_form error", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the handler is still waiting for the form")
	}
}

// A form the route will not read is refused with the whole body unread. A client that goes on trickling it must not hold
// the connection: the deadline is armed before the refusal, and nothing on the way out clears it. Without the deadline
// (before ReadBody moved to the handler's first line) net/http waits for 256 KB of a trickle that never comes.
func TestARefusedLoginFormThatKeepsTricklingIsNotHeld(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, head string
		first      string
		want       string
	}{
		{"wrong media type", "POST /auth/login HTTP/1.1\r\nHost: x\r\nContent-Type: text/plain\r\n", "return_to=%2Fcampaigns", " 415 "},
		{"query on a post", "POST /auth/login?return_to=%2F HTTP/1.1\r\nHost: x\r\nContent-Type: application/x-www-form-urlencoded\r\n", "return_to=%2Fcampaigns", " 400 "},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.svc.loginFormReadTimeout = 300 * time.Millisecond
			srv := httptest.NewServer(h.mux)
			t.Cleanup(srv.Close)
			line := slowtest.AnswerWhileTrickling(t, srv.Listener.Addr().String(), tt.head, tt.first, 10*time.Second)
			if !strings.Contains(line, tt.want) {
				t.Fatalf("status line = %q, want %q", line, tt.want)
			}
		})
	}
}

// A form over the limit is refused with the rest of the body unread. A client that goes on trickling it must not hold the
// connection. (net/http closes the connection after a MaxBytesReader overflow and the read deadline's error is sticky, so
// this one is a guard: it is not red with the deadline cleared on the way out.)
func TestAnOversizedLoginFormThatKeepsTricklingIsNotHeld(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.svc.loginFormReadTimeout = 300 * time.Millisecond
	srv := httptest.NewServer(h.mux)
	t.Cleanup(srv.Close)

	head := "POST /auth/login HTTP/1.1\r\nHost: x\r\nContent-Type: application/x-www-form-urlencoded\r\n"
	line := slowtest.AnswerWhileTrickling(t, srv.Listener.Addr().String(), head, "return_to="+strings.Repeat("a", maxLoginFormBytes+100), 10*time.Second)
	if !strings.Contains(line, " 413 ") {
		t.Fatalf("status line = %q, want 413 (form_too_large)", line)
	}
}
