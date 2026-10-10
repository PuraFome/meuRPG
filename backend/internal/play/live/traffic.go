package live

import (
	"sync/atomic"
	"time"
)

// lastUnary is when this process last started a request that is not a stream
// (unix nanoseconds); 0 until the first one.
var lastUnary atomic.Int64

// NoteUnaryRequest records that this process just received a request that is
// not a stream. cmd/api calls it for every unary RPC.
func NoteUnaryRequest() { lastUnary.Store(time.Now().UnixNano()) }

// Orphaned says whether this process has had no unary request for longer than
// quiet. A Cloud Run revision that was replaced keeps serving the streams it
// already had, but the load balancer sends every new request, a master's
// change included, to the new revision: the hub of the old one never hears of
// a change, so its streams are silent for good. A visible page always makes
// unary requests (the session notice, every 30 s), so a process that has had
// none for a few minutes, and still has streams, is not where the table is.
// With no request at all since the process started it is not orphaned (it has
// not been told anything yet). A wrong answer costs one reconnection.
func Orphaned(quiet time.Duration) bool {
	last := lastUnary.Load()
	return last != 0 && time.Since(time.Unix(0, last)) > quiet
}
