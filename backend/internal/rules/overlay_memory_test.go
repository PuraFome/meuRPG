package rules

import (
	"os"
	"runtime"
	"testing"
)

// TestWithMemory measures what a With keeps alive: a 300-entry overlay's content
// minus the shared SRD, with MEURPG_MEASURE=1 and -v (ADR-0018: eight cached
// contents in about 24 MB, so about 3 MB each, and a hard stop at 4 MB).
func TestWithMemory(t *testing.T) {
	if os.Getenv("MEURPG_MEASURE") == "" {
		t.Skip("set MEURPG_MEASURE=1 to measure")
	}
	srd := loadForTest(t)
	o := realisticOverlay(t, srd)
	heap := func() float64 { // bytes as a float: the difference of two readings is signed
		runtime.GC()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return float64(m.HeapAlloc)
	}
	const n = 8
	keep := make([]*Content, 0, n)
	before := heap()
	for range n {
		c, err := srd.With(o)
		if err != nil {
			t.Fatal(err)
		}
		keep = append(keep, c)
	}
	after := heap()
	runtime.KeepAlive(o) // the overlay lives past the second reading: it cancels out, not "content minus overlay"
	t.Logf("%d contents of 300 entries keep %.2f MB each (%.1f MB together)", n, (after-before)/n/1e6, (after-before)/1e6)
	runtime.KeepAlive(keep)
}
