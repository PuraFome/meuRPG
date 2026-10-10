package live

import (
	"testing"
	"time"
)

func TestOrphaned(t *testing.T) {
	// Not parallel: it uses the process-wide clock of the last request.
	lastUnary.Store(0)
	if Orphaned(time.Minute) {
		t.Fatal("a process with no request yet must not be orphaned")
	}
	NoteUnaryRequest()
	if Orphaned(time.Minute) {
		t.Fatal("a process that just had a request must not be orphaned")
	}
	lastUnary.Store(time.Now().Add(-4 * time.Minute).UnixNano())
	if !Orphaned(3 * time.Minute) {
		t.Fatal("a process silent for 4 minutes must be orphaned after 3")
	}
	lastUnary.Store(0)
}
