package campaignpackage_test

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
)

// A part longer than the upload said is refused with the rest of the body unread. A client that goes on trickling it must
// not hold the connection: the part's deadline stays armed on every path that returns without having read the body to its end.
func TestAPartThatIsTooLongAndKeepsTricklingIsNotHeld(t *testing.T) {
	h := newHarness(t, func(o *options) {
		o.configure = func(c *campaignpackage.Config) { c.PartReadTimeout = 300 * time.Millisecond }
	})
	master := h.newUser("Mestre")
	begun := must(master.pkg.BeginCampaignImport(t.Context(), connect.NewRequest(&pkgv1.BeginCampaignImportRequest{FileName: "g.meurpg.zip", TotalBytes: 100, Fingerprint: "g:trickle"})))

	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", h.server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	chunk := func(s string) string { return fmt.Sprintf("%x\r\n%s\r\n", len(s), s) }
	head := "PUT " + campaignpackage.PartsPath + begun.GetUpload().GetId() + "/parts/1 HTTP/1.1\r\nHost: x\r\n" + testUserHeader + ": " + master.id + "\r\nTransfer-Encoding: chunked\r\n\r\n"
	if _, err := conn.Write([]byte(head + chunk(strings.Repeat("a", 4096)))); err != nil {
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
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("no answer within 10s while the client kept trickling (the connection has no read deadline left): %v", err)
	}
	if strings.Contains(line, fmt.Sprint(http.StatusNoContent)) {
		t.Fatalf("status line = %q, want a refusal of the long part", line)
	}
}
