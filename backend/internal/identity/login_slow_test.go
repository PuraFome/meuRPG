package identity

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
		_, le, message := parseLoginForm(w, r, 300*time.Millisecond)
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
