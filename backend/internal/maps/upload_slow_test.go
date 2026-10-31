package maps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
	"github.com/PuraFome/meuRPG/backend/internal/platform/slowclient/slowtest"
)

// The read deadline of the upload surfaces as os.ErrDeadlineExceeded from the
// body; the answer is a 400 about a slow upload, not a 500.
func TestFormErrorForASlowUpload(t *testing.T) {
	t.Parallel()
	err := formError(fmt.Errorf("read: %w", os.ErrDeadlineExceeded), "cannot read the file")
	he, ok := errors.AsType[*httpError](err)
	if !ok || he.reason != ReasonMalformedRequest || he.status != 400 {
		t.Fatalf("formError() = %#v, want a 400 MALFORMED_REQUEST", err)
	}
}

type countingReader struct {
	r    io.Reader
	read *atomic.Int64
}

func (c countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read.Add(int64(n))
	return n, err
}

// An upload waits for the processing slot before it reads its file, so the
// uploads queued behind a busy slot do not each hold a body of up to 10 MiB.
func TestQueuedUploadsDoNotReadTheirBodies(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Master")
	campaign := h.newCampaign(master)

	const k = 6
	const fileSize = 9 << 20
	content := append([]byte("\xff\xd8\xff"), make([]byte, fileSize)...)

	// Occupy the one-at-a-time slot, as a running image would.
	h.svc.processing <- struct{}{}
	released := false
	release := func() {
		if !released {
			released = true
			<-h.svc.processing
		}
	}
	defer release()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var consumed atomic.Int64
	var total int64
	var wg sync.WaitGroup
	for range k {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("campaign_id", campaign)
		part, _ := w.CreateFormFile("file", "a.jpg")
		_, _ = part.Write(content)
		_ = w.Close()
		total += int64(buf.Len())
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.server.URL+UploadPath, countingReader{&buf, &consumed})
		if err != nil {
			t.Fatal(err)
		}
		req.ContentLength = int64(buf.Len())
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set(testUserHeader, master.id)
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := h.server.Client().Do(req)
			if err == nil {
				_, _ = io.Copy(io.Discard, res.Body)
				_ = res.Body.Close()
			}
		}()
	}

	// Correct behavior: while the slot is busy, the waiting requests do not
	// all hold a full body. Wait until every upload is parked on the slot: from
	// then on none of them can read more, so the count is final (a sleep would
	// either end too early and pass for the wrong reason, or cost its whole length).
	waitForUploadsOnTheSlot(t, k)
	got := consumed.Load()
	cancel()
	release()
	wg.Wait()
	if got >= total-total/10 {
		t.Errorf("with the processing slot busy, the server read %d of %d bytes of %d queued uploads (about %d bodies of %d MiB held in memory); want at most about one body", got, total, k, got/fileSize, fileSize>>20)
	}
}

// waitForUploadsOnTheSlot returns once n goroutines are blocked in acquireProcessing.
func waitForUploadsOnTheSlot(t *testing.T, n int) {
	t.Helper()
	buf := make([]byte, 1<<20)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		parked := 0
		for _, g := range strings.Split(string(buf[:runtime.Stack(buf, true)]), "\n\n") {
			if strings.Contains(g, "(*Service).acquireProcessing(") {
				parked++
			}
		}
		if parked >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("fewer than %d uploads reached the processing slot", n)
}

// The processing slot is shared with the fog tiles and every image job, so an
// upload that holds it has its own, shorter, deadline for the file: a client
// that trickles the body is answered like any slow upload and the slot is
// freed, long before the upload's whole two minutes.
func TestATrickledUploadDoesNotHoldTheProcessingSlot(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.svc.slotReadTimeout = 300 * time.Millisecond
	master := h.newUser("Master")
	campaign := h.newCampaign(master)

	// A prompt upload under the same short deadline passes: the deadline is
	// for a trickle, not for a client that sends its file.
	if res := master.upload(campaign, "a.png", pngImage(t, 8, 8)); res.status != http.StatusCreated {
		t.Fatalf("prompt upload: status %d, body %s; want 201", res.status, res.body)
	}

	pr, pw := io.Pipe()
	var head bytes.Buffer
	form := multipart.NewWriter(&head)
	_ = form.WriteField("campaign_id", campaign)
	if _, err := form.CreateFormFile("file", "slow.png"); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, h.server.URL+UploadPath, pr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set(testUserHeader, master.id)

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		defer pw.Close()
		if _, err := pw.Write(head.Bytes()); err != nil {
			return
		}
		for { // one byte at a time: never enough for the file, never idle for long
			select {
			case <-stop:
				return
			case <-time.After(50 * time.Millisecond):
				if _, err := pw.Write([]byte{0xff}); err != nil {
					return
				}
			}
		}
	}()

	type answer struct {
		res httpResult
		err error
	}
	answered := make(chan answer, 1)
	started := time.Now()
	go func() {
		res, err := h.server.Client().Do(req)
		if err != nil {
			answered <- answer{err: err}
			return
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		answered <- answer{res: httpResult{status: res.StatusCode, header: res.Header, body: body}}
	}()

	// While the client trickles, the slot is held (so the test sees what it guards).
	for held := false; !held; {
		select {
		case h.svc.processing <- struct{}{}:
			<-h.svc.processing
			if time.Since(started) > 10*time.Second {
				t.Fatal("the trickled upload never took the processing slot")
			}
			time.Sleep(10 * time.Millisecond)
		default:
			held = true
		}
	}

	select {
	case a := <-answered:
		if a.err != nil || a.res.status != http.StatusBadRequest || !bytes.Contains(a.res.body, []byte(ReasonMalformedRequest)) {
			t.Fatalf("trickled upload = %d %s, %v; want 400 %s", a.res.status, a.res.body, a.err, ReasonMalformedRequest)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("a trickled upload kept the processing slot for more than the slot's deadline")
	}

	// The slot is free again.
	select {
	case h.svc.processing <- struct{}{}:
		<-h.svc.processing
	case <-time.After(5 * time.Second):
		t.Fatal("the processing slot was not freed after the trickled upload")
	}
}

// slowBlobs is a blob store whose writes take a while, as they do under load.
type slowBlobs struct {
	blob.Store
	delay time.Duration
}

func (b slowBlobs) Put(ctx context.Context, key, contentType string, r io.Reader) error {
	select {
	case <-time.After(b.delay):
	case <-ctx.Done():
		return fmt.Errorf("the request was canceled while the image was stored: %w", context.Cause(ctx))
	}
	return b.Store.Put(ctx, key, contentType, r)
}

// The slot's deadline is for the file's read. A small upload arrives whole before the slot is taken, so net/http's
// background read (the one that notices a client that left) is already waiting on the connection; a deadline that
// outlived the read would fire on it while the image is stored and cancel the request (499). The store here takes
// four times the deadline, and the upload must still be created.
func TestASmallUploadWhoseStoreOutlastsTheSlotDeadlineIsCreated(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.svc.slotReadTimeout = 50 * time.Millisecond
	h.svc.blobs = slowBlobs{Store: h.svc.blobs, delay: 200 * time.Millisecond}
	master := h.newUser("Master")
	campaign := h.newCampaign(master)

	if res := master.upload(campaign, "a.png", pngImage(t, 8, 8)); res.status != http.StatusCreated {
		t.Fatalf("upload with a slow store: status %d, body %s; want 201", res.status, res.body)
	}
}

func multipartStart(boundary, campaign string) string {
	return "--" + boundary + "\r\nContent-Disposition: form-data; name=\"campaign_id\"\r\n\r\n" + campaign + "\r\n"
}

// otherPart is a part that is not the file: the upload is refused at once, with the rest of the body unread.
func otherPart(boundary string) string {
	return "--" + boundary + "\r\nContent-Disposition: form-data; name=\"note\"\r\n\r\nabc"
}

// A form whose second part is not the file is refused at once, under the processing slot, with the rest of the body unread.
// A client that goes on trickling does not hold the connection: the slot's deadline stays armed on a path that returns
// without having read the body to its end.
func TestARefusedFormThatKeepsTricklingUnderTheSlotIsNotHeld(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.svc.slotReadTimeout = 300 * time.Millisecond
	master := h.newUser("Master")
	campaign := h.newCampaign(master)
	head := "POST " + UploadPath + " HTTP/1.1\r\nHost: x\r\n" + testUserHeader + ": " + master.id + "\r\nContent-Type: multipart/form-data; boundary=B\r\n"
	first := multipartStart("B", campaign) + otherPart("B")
	line := slowtest.AnswerWhileTrickling(t, h.server.Listener.Addr().String(), head, first, 10*time.Second)
	if !strings.Contains(line, " 400 ") {
		t.Fatalf("status line = %q, want 400 (the form must have a file field)", line)
	}
}

// A caller who may not upload is refused right after the campaign field, with the rest of the body unread: a client that
// goes on trickling does not hold the connection past the upload's deadline.
func TestARefusedUploadThatKeepsTricklingIsNotHeld(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.svc.formReadTimeout = 300 * time.Millisecond
	master := h.newUser("Master")
	campaign := h.newCampaign(master)
	stranger := h.newUser("Stranger")
	head := "POST " + UploadPath + " HTTP/1.1\r\nHost: x\r\n" + testUserHeader + ": " + stranger.id + "\r\nContent-Type: multipart/form-data; boundary=B\r\n"
	line := slowtest.AnswerWhileTrickling(t, h.server.Listener.Addr().String(), head, multipartStart("B", campaign)+otherPart("B"), 10*time.Second)
	if strings.Contains(line, " 201 ") {
		t.Fatalf("status line = %q, want a refusal", line)
	}
}
