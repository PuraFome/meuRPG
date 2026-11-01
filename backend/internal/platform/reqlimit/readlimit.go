package reqlimit

import (
	"net/http"
	"strings"
	"sync"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1/rulesv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/platform/slowclient"
)

const (
	// DefaultReadLimit is the most bytes the body of a unary RPC may have, for a
	// procedure that raisedReadLimits does not name.
	DefaultReadLimit = 64 << 10
	// Backstop is the most any one RPC message may have, whatever the
	// procedure: the server's connect.WithReadMaxBytes. No read limit is above it.
	Backstop = 4 << 20
)

// raisedReadLimits are the procedures whose unary body may be larger than
// DefaultReadLimit, by procedure ("/meurpg.x.v1.Service/Method"), in bytes.
// Each entry has a comment that says why. TestRegistry requires an entry for
// every procedure that reaches a field raised over its defaults, and none
// above Backstop. Tests change it only through the lock (withRaised).
var (
	raisedMu         sync.RWMutex
	raisedReadLimits = map[string]int64{
		// A table-content pack is up to 3 MiB of proto JSON (characters.MaxPackBytes).
		rulesv1connect.TableContentServiceImportTableContentProcedure: Backstop,
	}
)

// ReadLimit is the most bytes the body of a unary request to the procedure may have.
func ReadLimit(procedure string) int64 {
	raisedMu.RLock()
	defer raisedMu.RUnlock()
	if n, ok := raisedReadLimits[procedure]; ok {
		return n
	}
	return DefaultReadLimit
}

// Middleware reads the body of a unary RPC through http.MaxBytesReader at
// ReadLimit(procedure). Connect reports a body over the limit as
// `resource_exhausted`. Put it outside slowclient.Middleware, so that the
// deadline's body stays the outermost one.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/meurpg.") && slowclient.IsUnaryRPC(r) {
			r.Body = http.MaxBytesReader(w, r.Body, ReadLimit(r.URL.Path))
		}
		next.ServeHTTP(w, r)
	})
}
