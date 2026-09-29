package httpserver

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// cspHeader is as strict as the app allows:
//
//   - script-src has no exception at all. The production build (see
//     web/angular.json, "inlineCritical: false") ships no inline <script>
//     and no inline event-handler attribute, verified both by inspecting
//     the built index.html and by loading the app with the console open
//     (no CSP violations).
//   - style-src needs 'unsafe-inline'. Angular's default view encapsulation
//     injects each component's styles as a <style> tag at *runtime*
//     (nothing to do with the build step above); confirmed empirically
//     here too, since without it every component's styles silently failed
//     to apply. The alternative is a per-request nonce threaded through
//     both this header and a <meta name="csp-nonce"> in index.html
//     (https://angular.dev/best-practices/security#content-security-policy),
//     which trades this one relaxation for turning index.html from a
//     cacheable static file into something rendered per request. Not worth
//     it yet for a single low-risk directive; revisit if that changes.
const cspHeader = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'; " +
	"object-src 'none'"

// NewStatic returns a handler that serves the Angular production build
// (index.html plus its hashed assets) from dir, falling back to index.html
// for any GET that doesn't match a file, so the Angular router can handle
// client-side routes.
//
// ok is false when dir is empty or has no index.html: the caller should
// skip registering the handler and log why instead of failing to start.
// That is the normal case outside the Docker image (e.g. `make run` on the
// host, where WEB_DIR's default has nothing under it).
func NewStatic(dir string) (handler http.Handler, ok bool) {
	if dir == "" {
		return nil, false
	}
	indexPath := filepath.Join(dir, "index.html")
	if info, err := os.Stat(indexPath); err != nil || info.IsDir() {
		return nil, false
	}
	return &staticHandler{dir: dir, indexPath: indexPath}, true
}

type staticHandler struct {
	dir       string
	indexPath string
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// path.Clean on an absolute path can never produce a ".." that escapes
	// above "/": Clean has nothing to pop it against. That, plus
	// filepath.Join below, keeps fsPath inside h.dir.
	cleanPath := path.Clean(r.URL.Path)

	// Registering this handler at "/" only ever receives requests that no
	// more specific pattern on the mux claimed. isAPIPath is a second,
	// explicit guard against serving index.html for a request that *looks*
	// like it was meant for a Connect RPC, /auth or a probe but doesn't
	// exist (a typo'd method, or /auth/* before the identity module
	// registers it) - callers of those should see 404, not an HTML page.
	if isAPIPath(cleanPath) {
		http.NotFound(w, r)
		return
	}

	h.setSecurityHeaders(w)

	if fsPath, ok := h.existingAsset(cleanPath); ok {
		// Angular's outputHashing:"all" puts a content hash in every
		// filename except index.html, so any file found here is safe to
		// cache forever: a change always ships under a new name.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeFile(w, r, fsPath)
		return
	}

	// Either "/" itself, or a client-side route with no matching file
	// (e.g. a future "/campaigns/42"): serve the app shell and let the
	// Angular router take over.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, h.indexPath)
}

// existingAsset reports whether cleanPath maps to a real, non-directory
// file under h.dir other than index.html, which always goes through the
// no-cache fallback path above instead.
func (h *staticHandler) existingAsset(cleanPath string) (string, bool) {
	if cleanPath == "/" {
		return "", false
	}
	fsPath := filepath.Join(h.dir, filepath.FromSlash(cleanPath))
	// #nosec G703 -- cleanPath came from path.Clean on an absolute URL
	// path, which can never contain a ".." that escapes above "/"; and
	// http.ServeFile independently rejects any raw request path containing
	// ".." before it ever looks at fsPath (see ServeHTTP above).
	info, err := os.Stat(fsPath)
	if err != nil || info.IsDir() || fsPath == h.indexPath {
		return "", false
	}
	return fsPath, true
}

func (h *staticHandler) setSecurityHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("Content-Security-Policy", cspHeader)
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin")
}

// isAPIPath reports whether p belongs to a namespace this server reserves
// for something other than the static app: Connect RPCs (every package in
// proto/ starts with "meurpg."), the identity module's routes, and the
// probes already registered on the mux in New().
func isAPIPath(p string) bool {
	switch {
	case strings.HasPrefix(p, "/meurpg."):
		return true
	case p == "/auth" || strings.HasPrefix(p, "/auth/"):
		return true
	case p == "/healthz" || p == "/readyz":
		return true
	default:
		return false
	}
}
