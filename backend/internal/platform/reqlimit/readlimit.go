package reqlimit

import (
	"net/http"
	"strings"

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
// An entry is added together with the field options that need it, with a
// comment that names the fields. TestRegistry requires an entry for every
// procedure that reaches a field raised over its defaults, and none above Backstop.
var raisedReadLimits = map[string]int64{}

// ReadLimit is the most bytes the body of a unary request to the procedure may have.
func ReadLimit(procedure string) int64 {
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
