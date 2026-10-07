package gen

import (
	"encoding/base64"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func peakHeap(f func()) (peak, total uint64) {
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	done := make(chan struct{})
	var wg sync.WaitGroup
	var top uint64
	wg.Add(1)
	go func() {
		defer wg.Done()
		var m runtime.MemStats
		for {
			runtime.ReadMemStats(&m)
			top = max(top, m.HeapAlloc)
			select {
			case <-done:
				return
			case <-time.After(2 * time.Millisecond):
			}
		}
	}()
	f()
	close(done)
	wg.Wait()
	var end runtime.MemStats
	runtime.ReadMemStats(&end)
	return top - min(top, base.HeapAlloc), end.TotalAlloc - base.TotalAlloc
}

// TestMeasureMemory measures the two sides of a call at their worst: the body
// of a request with 14 references that fill the 8 MiB cap, written as a stream,
// and an answer of the largest size parseAnswer reads (8 MiB). Run it with
// MEURPG_MEASURE=1 -v; docs/operations.md has the numbers.
func TestMeasureMemory(t *testing.T) {
	if os.Getenv("MEURPG_MEASURE") == "" {
		t.Skip("set MEURPG_MEASURE=1 to measure")
	}
	req := Request{Prompt: strings.Repeat("a", 500), AspectRatio: "16:9"}
	for i := range 14 {
		// 420 KB each: 14 of them make 5.9 MB, 7.8 MB in base64, just under the cap.
		req.References = append(req.References, Reference{Image: Image{MimeType: "image/jpeg", Data: make([]byte, 420<<10)}, Character: i >= 10})
	}
	held := req.References // the images are held by the caller anyway
	peak, total := peakHeap(func() {
		if err := writeBody(io.Discard, "m", req); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("request body (%d refs, ~%.1f MiB estimated): peak extra heap %.2f MiB, allocated %.2f MiB", len(held), float64(req.BodySize())/(1<<20), float64(peak)/(1<<20), float64(total)/(1<<20))

	// The largest answer: a picture of 4.5 MiB (6 MiB in base64) after a draft.
	raw := make([]byte, 6<<20)
	b64 := base64.StdEncoding.EncodeToString(raw)
	answerJSON := answer("completed", step("thought", imageBlock(b64[:len(b64)/3]))+","+step("model_output", imageBlock(b64[:len(b64)/2])))
	t.Logf("answer of %.1f MiB", float64(len(answerJSON))/(1<<20))
	peak, total = peakHeap(func() {
		img, err := parseAnswer(200, strings.NewReader(answerJSON))
		if err != nil || len(img.Data) == 0 {
			t.Fatalf("parseAnswer() = %v", err)
		}
	})
	t.Logf("answer parse: peak extra heap %.1f MiB, allocated %.1f MiB (the answer text itself is %.1f MiB, outside this)", float64(peak)/(1<<20), float64(total)/(1<<20), float64(len(answerJSON))/(1<<20))
}
