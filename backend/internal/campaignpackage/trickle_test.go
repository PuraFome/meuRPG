package campaignpackage_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
	"github.com/PuraFome/meuRPG/backend/internal/platform/slowclient/slowtest"
)

// A part longer than the upload said is refused with the rest of the body unread. A client that goes on trickling it must
// not hold the connection: the part's deadline stays armed on every path that returns without having read the body to its end.
func TestAPartThatIsTooLongAndKeepsTricklingIsNotHeld(t *testing.T) {
	h := newHarness(t, func(o *options) {
		o.configure = func(c *campaignpackage.Config) { c.PartReadTimeout = 300 * time.Millisecond }
	})
	master := h.newUser("Mestre")
	begun := must(master.pkg.BeginCampaignImport(t.Context(), connect.NewRequest(&pkgv1.BeginCampaignImportRequest{FileName: "g.meurpg.zip", TotalBytes: 100, Fingerprint: "g:trickle"})))

	head := "PUT " + campaignpackage.PartsPath + begun.GetUpload().GetId() + "/parts/1 HTTP/1.1\r\nHost: x\r\n" + testUserHeader + ": " + master.id + "\r\n"
	line := slowtest.AnswerWhileTrickling(t, h.server.Listener.Addr().String(), head, strings.Repeat("a", 4096), 10*time.Second)
	if strings.Contains(line, fmt.Sprint(http.StatusNoContent)) {
		t.Fatalf("status line = %q, want a refusal of the long part", line)
	}
}

// A part shorter than the upload said is dropped, and the store may be slower than the part's deadline: the delete that
// drops the wrong blob must not be cut short by a deadline that outlived the body. A guard: the deadline is armed before
// the body is read and net/http clears it when its background read starts, so this passes on the code before the body disarmed
// itself too. It keeps the criterion under test.
func TestAShortPartIsDroppedEvenWhenTheStoreOutlastsTheDeadline(t *testing.T) {
	h := newHarness(t, func(o *options) {
		o.configure = func(c *campaignpackage.Config) {
			c.PartReadTimeout = 300 * time.Millisecond
			c.Blobs = slowDeleteBlobs{Store: c.Blobs, delay: 900 * time.Millisecond}
		}
	})
	master := h.newUser("Mestre")
	begun := must(master.pkg.BeginCampaignImport(t.Context(), connect.NewRequest(&pkgv1.BeginCampaignImportRequest{FileName: "g.meurpg.zip", TotalBytes: 100, Fingerprint: "g:short"})))
	id := begun.GetUpload().GetId()

	if res := master.request(http.MethodPut, campaignpackage.PartsPath+id+"/parts/1", []byte(strings.Repeat("a", 50))); res.status != http.StatusBadRequest {
		t.Fatalf("short part: status %d, body %s; want 400", res.status, res.body)
	}
	obj, err := h.blobs.Open(t.Context(), "imports/"+id+"/parts/1")
	if err == nil {
		_ = obj.Close()
		t.Fatal("the short part's blob is still stored: the delete that drops it was canceled")
	}
	if !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("Open() error = %v, want blob.ErrNotFound", err)
	}
}

// slowDeleteBlobs is a blob store whose deletes take a while, as they do under load.
type slowDeleteBlobs struct {
	blob.Store
	delay time.Duration
}

func (b slowDeleteBlobs) Delete(ctx context.Context, key string) error {
	select {
	case <-time.After(b.delay):
	case <-ctx.Done():
		return fmt.Errorf("the request was canceled while the blob was deleted: %w", context.Cause(ctx))
	}
	return b.Store.Delete(ctx, key)
}
