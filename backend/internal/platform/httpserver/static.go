package httpserver

import (
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/slowclient"
)

// init teaches the standard library the types of two files the app copies as they are. Go's
// built-in table has neither, and without a system mime.types (the production image has none)
// the file server would sniff the manifest as text/plain, which browsers refuse as a manifest.
func init() {
	for ext, typ := range map[string]string{
		".webmanifest": "application/manifest+json",
		".ico":         "image/x-icon",
	} {
		// An error only means the extension or type is malformed: these are constants.
		_ = mime.AddExtensionType(ext, typ)
	}
}

// staticWriteTimeout is how long a client has to take one of the app's files,
// which are a few megabytes at most; a client that stops reading is dropped.
const staticWriteTimeout = 2 * time.Minute

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
//   - form-action is 'self' plus the sign-in provider's origin, when one is
//     configured (WithFormActionOrigin). Browsers apply form-action to the
//     whole redirect chain of a form submission, so the invite page's POST
//     to /auth/login, which the server answers with a 303 to the provider,
//     is blocked unless the provider's origin is listed.
func cspHeader(formActionOrigins []string) string {
	var formAction strings.Builder
	formAction.WriteString("'self'")
	for _, origin := range formActionOrigins {
		formAction.WriteString(" " + origin)
	}
	return "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"connect-src 'self'; " +
		"base-uri 'self'; " +
		"form-action " + formAction.String() + "; " +
		"frame-ancestors 'none'; " +
		"object-src 'none'"
}

// StaticOption configures NewStatic.
type StaticOption func(*staticHandler) error

// WithFormActionOrigin allows forms on the app's pages to lead to origin, an
// absolute http(s) URL reduced to its scheme and host (for example the OIDC
// issuer, whose authorization endpoint the sign-in redirect goes to). A URL
// with a path, query or fragment is reduced to its origin.
func WithFormActionOrigin(rawURL string) StaticOption {
	return func(h *staticHandler) error {
		u, err := url.Parse(rawURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("form-action origin must be an absolute http(s) URL, got %q", rawURL)
		}
		h.formActionOrigins = append(h.formActionOrigins, u.Scheme+"://"+u.Host)
		return nil
	}
}

// NewStatic returns a handler that serves the Angular production build
// (index.html plus its hashed assets) from dir, falling back to index.html
// for any GET that doesn't match a file, so the Angular router can handle
// client-side routes.
//
// ok is false when dir is empty or has no index.html: the caller should
// skip registering the handler and log why instead of failing to start.
// That is the normal case outside the Docker image (e.g. `make run` on the
// host, where WEB_DIR's default has nothing under it). err reports an
// invalid option.
func NewStatic(dir string, opts ...StaticOption) (handler http.Handler, ok bool, err error) {
	if dir == "" {
		return nil, false, nil
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, false, nil
	}
	if !isFile(root, "index.html") {
		_ = root.Close()
		return nil, false, nil
	}
	h := &staticHandler{root: root, indexPath: filepath.Join(dir, "index.html")}
	for _, opt := range opts {
		if err := opt(h); err != nil {
			_ = root.Close()
			return nil, false, err
		}
	}
	h.csp = cspHeader(h.formActionOrigins)
	return h, true, nil
}

// isFile reports whether rel exists under root and is not a directory. A missing build is
// the normal case outside the Docker image, so it is not an error.
func isFile(root *os.Root, rel string) bool {
	info, err := root.Stat(rel)
	return err == nil && !info.IsDir()
}

// hashedName matches the names Angular's build gives its output: the name,
// a dash and an 8-character content hash (upper-case letters and digits),
// then the extension, as in "main-3QX7LZ2K.js" or "media/font-AB12CD34.woff2".
var hashedName = regexp.MustCompile(`-[A-Z0-9]{8}\.[A-Za-z0-9]+(\.[A-Za-z0-9]+)*$`)

// staticHandler reads only through root, which stays open for the life of the handler.
// root refuses any path, symlinks included, that leads out of the build directory.
type staticHandler struct {
	root              *os.Root
	indexPath         string
	formActionOrigins []string
	csp               string
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// path.Clean on an absolute path can never produce a ".." that escapes above "/".
	// h.root enforces the same for every file that is opened.
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

	if rel, ok := h.existingAsset(cleanPath); ok {
		// Angular's outputHashing:"all" puts a content hash in the name of
		// every file it builds, so those are safe to cache forever: a
		// change always ships under a new name. The files copied as they are
		// (favicon.ico, material-symbols/*) keep their names, so the
		// browser must ask again whether they changed.
		if hashedName.MatchString(path.Base(cleanPath)) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		defer slowclient.WriteBody(w, staticWriteTimeout)()
		h.serveAsset(w, r, rel)
		return
	}

	// Either "/" itself, or a client-side route with no matching file
	// (e.g. a future "/campaigns/42"): serve the app shell and let the
	// Angular router take over.
	w.Header().Set("Cache-Control", "no-cache")
	defer slowclient.WriteBody(w, staticWriteTimeout)()
	http.ServeFile(w, r, h.indexPath)
}

// existingAsset reports whether cleanPath maps to a real, non-directory
// file under the build directory other than index.html, which always goes through the
// no-cache fallback path above instead.
// rel is its path relative to the root.
func (h *staticHandler) existingAsset(cleanPath string) (rel string, ok bool) {
	rel = filepath.FromSlash(strings.TrimPrefix(cleanPath, "/"))
	if !filepath.IsLocal(rel) || rel == "index.html" || !isFile(h.root, rel) {
		return "", false
	}
	return rel, true
}

// serveAsset serves the file at rel, as its .br or .gz copy when it is a script or a stylesheet,
// the client accepts that encoding and the copy exists.
// The content type is always the one of the original file.
func (h *staticHandler) serveAsset(w http.ResponseWriter, r *http.Request, rel string) {
	if isCompressible(rel) {
		w.Header().Add("Vary", "Accept-Encoding")
		if coding, f, info := h.openVariant(r, rel); f != nil {
			defer func() { _ = f.Close() }()
			w.Header().Set("Content-Encoding", coding)
			http.ServeContent(w, r, filepath.Base(rel), info.ModTime(), f)
			return
		}
	}
	f, err := h.root.Open(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, filepath.Base(rel), info.ModTime(), f)
}

// precompressed lists the encodings the build ships next to its scripts and stylesheets,
// in the order the server prefers them.
var precompressed = []struct{ coding, suffix string }{
	{"br", ".br"},
	{"gzip", ".gz"},
}

// isCompressible reports whether the build ships compressed copies of the file at p.
func isCompressible(p string) bool {
	ext := filepath.Ext(p)
	return ext == ".js" || ext == ".css"
}

// openVariant opens the best compressed copy of rel that the request accepts.
// f is nil when there is none.
func (h *staticHandler) openVariant(r *http.Request, rel string) (coding string, f *os.File, info os.FileInfo) {
	accept := r.Header.Values("Accept-Encoding")
	for _, v := range precompressed {
		if !acceptsCoding(accept, v.coding) {
			continue
		}
		file, err := h.root.Open(rel + v.suffix)
		if err != nil {
			continue
		}
		st, err := file.Stat()
		if err != nil || st.IsDir() {
			_ = file.Close()
			continue
		}
		return v.coding, file, st
	}
	return "", nil, nil
}

// acceptsCoding reports whether the Accept-Encoding header values allow coding.
// Tokens are case-insensitive, "*" stands for every coding not named, and a q-value that is not above 0 (q=0, a negative or NaN) refuses one.
func acceptsCoding(values []string, coding string) bool {
	wildcard, named := false, false
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			token, q := parseCoding(part)
			switch token {
			case coding:
				if !(q > 0) {
					return false
				}
				named = true
			case "*":
				wildcard = wildcard || q > 0
			}
		}
	}
	return named || wildcard
}

// parseCoding splits one element of an Accept-Encoding header into its lower-case token and its q-value.
// A missing or malformed q-value counts as 1.
func parseCoding(part string) (token string, q float64) {
	fields := strings.Split(part, ";")
	token = strings.ToLower(strings.TrimSpace(fields[0]))
	q = 1
	for _, param := range fields[1:] {
		key, val, ok := strings.Cut(strings.TrimSpace(param), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "q") {
			continue
		}
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			q = parsed
		}
	}
	return token, q
}

func (h *staticHandler) setSecurityHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("Content-Security-Policy", h.csp)
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin")
}

// isAPIPath reports whether p belongs to a namespace this server reserves
// for something other than the static app: Connect RPCs (every package in
// proto/ starts with "meurpg."), the identity module's routes, the maps
// module's image routes (/images/{id}, /uploads/images), the campaign package's
// part uploads and export downloads (/uploads/campaign-imports, /downloads), the conversation's audio (/session-messages/{id}/audio), and the probes
// already registered on the mux in New().
func isAPIPath(p string) bool {
	switch {
	case strings.HasPrefix(p, "/meurpg."):
		return true
	case p == "/auth" || strings.HasPrefix(p, "/auth/"):
		return true
	case p == "/images" || strings.HasPrefix(p, "/images/"):
		return true
	case p == "/uploads" || strings.HasPrefix(p, "/uploads/"):
		return true
	case p == "/session-messages" || strings.HasPrefix(p, "/session-messages/"):
		return true
	case p == "/downloads" || strings.HasPrefix(p, "/downloads/"):
		return true
	case p == "/healthz" || p == "/readyz":
		return true
	default:
		return false
	}
}
