package identity

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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

// A form over the limit is refused with the rest of the body unread. A client that goes on trickling it must not hold the
// connection: the deadline stays armed on every path that returns without having read the form to its end.
func TestAnOversizedLoginFormThatKeepsTricklingIsNotHeld(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, le, _ := parseLoginForm(w, r, 300*time.Millisecond)
		status := http.StatusNoContent
		if le != nil {
			status = le.status
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	head := "POST /auth/login HTTP/1.1\r\nHost: x\r\nContent-Type: application/x-www-form-urlencoded\r\n"
	line := answerWhileTrickling(t, srv.Listener.Addr().String(), head, "return_to="+strings.Repeat("a", maxLoginFormBytes+100), 10*time.Second)
	if !strings.Contains(line, " 413 ") {
		t.Fatalf("status line = %q, want 413 (form_too_large)", line)
	}
}

// answerWhileTrickling sends head and first as the start of a chunked request, goes on sending a byte every 20 ms and
// returns the status line of the answer. It fails when no answer comes within wait: net/http discards what is left of a
// body the handler did not read before it answers, so a connection with no deadline left holds the answer back for as
// long as the client trickles.
func answerWhileTrickling(t *testing.T, addr, head, first string, wait time.Duration) string {
	t.Helper()
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	chunk := func(s string) string { return fmt.Sprintf("%x\r\n%s\r\n", len(s), s) }
	if _, err := conn.Write([]byte(head + "Transfer-Encoding: chunked\r\n\r\n" + chunk(first))); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				if _, err := conn.Write([]byte(chunk("x"))); err != nil {
					return
				}
			}
		}
	}()
	_ = conn.SetReadDeadline(time.Now().Add(wait))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("no answer within %v while the client kept trickling (the connection has no read deadline left): %v", wait, err)
	}
	return line
}
