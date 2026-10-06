package images

import (
	"image"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"
)

// peakHeap runs f and returns the most live heap it reached above where it
// started, and the bytes it allocated in all.
func peakHeap(f func()) (peak, total uint64) {
	// A tight collector, so the peak is what is live and not what is waiting to be collected.
	defer debug.SetGCPercent(debug.SetGCPercent(5))
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

// TestShrinkMemory measures the worst reference a campaign can hold: a stored
// image of 40 megapixels, shrunk to 1024 px. Run it with MEURPG_MEASURE=1 -v
// (docs/operacao.md has the numbers).
func TestShrinkMemory(t *testing.T) {
	if os.Getenv("MEURPG_MEASURE") == "" {
		t.Skip("set MEURPG_MEASURE=1 to measure")
	}
	img := image.NewNRGBA(image.Rect(0, 0, 8000, 5000))
	for y := range 5000 {
		for x := range 8000 {
			o := y*img.Stride + x*4
			img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = uint8(x/32), uint8(y/20), uint8((x+y)/64), 255
		}
	}
	for name, data := range map[string][]byte{"JPEG 8000x5000": encodeJPEG(t, img), "PNG 8000x5000": encodePNG(t, img)} {
		img = nil
		var out []byte
		peak, total := peakHeap(func() {
			var err error
			if out, _, err = Shrink(data, 1024, 85); err != nil {
				t.Fatal(err)
			}
		})
		for _, with := range []bool{false, true} {
			makeReference = with
			var res *Result
			peakP, totalP := peakHeap(func() {
				var err error
				if res, err = Process(data); err != nil {
					t.Fatal(err)
				}
			})
			t.Logf("Process (an upload) of the %s file, reference step %v: peak live heap %.0f MiB, allocated %.0f MiB", name, with, float64(peakP)/(1<<20), float64(totalP)/(1<<20))
			_ = res
		}
		makeReference = true
		var res *Result
		peakP, totalP := peakHeap(func() {
			var err error
			if res, err = Process(data); err != nil {
				t.Fatal(err)
			}
		})
		t.Logf("Process (an upload, for comparison) of the same %s file: peak live heap %.0f MiB, allocated %.0f MiB, reference made %d KiB", name, float64(peakP)/(1<<20), float64(totalP)/(1<<20), len(res.Reference)>>10)
		t.Logf("Shrink of a %s file of %.1f MiB: peak live heap %.0f MiB, allocated %.0f MiB, result %.0f KiB",
			name, float64(len(data))/(1<<20), float64(peak)/(1<<20), float64(total)/(1<<20), float64(len(out))/(1<<10))
	}
}
