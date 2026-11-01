package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestBuild writes a minimal fake Angular build to a temp dir:
// index.html plus "hashed" assets and files copied as they are, the way `ng build` would.
func newTestBuild(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "index.html"), "<html>app shell</html>")
	writeFile(t, filepath.Join(dir, "main-ABCD1234.js"), "console.log('app')")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "sub", "nested-EFGH5678.js"), "console.log('nested')")
	// Copied by the build as they are, so without a hash in the name.
	writeFile(t, filepath.Join(dir, "favicon.ico"), "icon")
	if err := os.Mkdir(filepath.Join(dir, "material-symbols"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "material-symbols", "outlined.css"), "css")
	writeFile(t, filepath.Join(dir, "material-symbols", "material-symbols-outlined.woff2"), "font")

	return dir
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestNewStatic_MissingBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dir  string
	}{
		{name: "empty dir means disabled", dir: ""},
		{name: "dir does not exist", dir: "/no/such/directory"},
		{name: "dir exists but has no index.html", dir: t.TempDir()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, ok, _ := NewStatic(tt.dir)
			if ok {
				t.Fatalf("NewStatic(%q) ok = true, want false", tt.dir)
			}
			if handler != nil {
				t.Fatalf("NewStatic(%q) handler = %v, want nil", tt.dir, handler)
			}
		})
	}
}

func TestStaticHandler_Routing(t *testing.T) {
	t.Parallel()

	dir := newTestBuild(t)
	handler, ok, _ := NewStatic(dir)
	if !ok {
		t.Fatalf("NewStatic(%q) ok = false, want true", dir)
	}

	const indexBody = "<html>app shell</html>"

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string // checked only when non-empty
		wantCache  string // checked only when wantStatus is set and not a redirect/error
	}{
		{name: "root serves index", path: "/", wantStatus: http.StatusOK, wantBody: indexBody, wantCache: "no-cache"},
		{
			name: "hashed asset is served with immutable caching", path: "/main-ABCD1234.js",
			wantStatus: http.StatusOK, wantBody: "console.log('app')", wantCache: "public, max-age=31536000, immutable",
		},
		{
			name: "nested asset is served too", path: "/sub/nested-EFGH5678.js",
			wantStatus: http.StatusOK, wantBody: "console.log('nested')", wantCache: "public, max-age=31536000, immutable",
		},
		{
			name: "an unhashed file is revalidated, not immutable", path: "/favicon.ico",
			wantStatus: http.StatusOK, wantBody: "icon", wantCache: "no-cache",
		},
		{
			name: "an unhashed stylesheet is revalidated", path: "/material-symbols/outlined.css",
			wantStatus: http.StatusOK, wantBody: "css", wantCache: "no-cache",
		},
		{
			// "-outlined." looks like a hash by length, but it is not upper-case.
			name: "an unhashed font is revalidated", path: "/material-symbols/material-symbols-outlined.woff2",
			wantStatus: http.StatusOK, wantBody: "font", wantCache: "no-cache",
		},
		{
			// http.ServeFile's own well-known behavior: a request whose
			// path ends in "/index.html" is redirected to "./" rather than
			// served directly, so it can never end up as a second,
			// separately-cacheable URL for the same page.
			name: "index.html requested directly redirects to the canonical /",
			path: "/index.html", wantStatus: http.StatusMovedPermanently,
		},
		{
			name: "unknown client route falls back to index (SPA)", path: "/campaigns/42",
			wantStatus: http.StatusOK, wantBody: indexBody, wantCache: "no-cache",
		},
		{
			name: "unknown Connect RPC path is a 404, not the app shell",
			path: "/meurpg.system.v1.SystemService/Nope", wantStatus: http.StatusNotFound,
		},
		{name: "auth path is a 404, not the app shell", path: "/auth/callback", wantStatus: http.StatusNotFound},
		{name: "image path is a 404, not the app shell", path: "/images/6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001", wantStatus: http.StatusNotFound},
		{name: "upload path is a 404, not the app shell", path: "/uploads/images", wantStatus: http.StatusNotFound},
		{name: "campaign package part path is a 404, not the app shell", path: "/uploads/campaign-imports/6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001/parts/1", wantStatus: http.StatusNotFound},
		{name: "export download path is a 404, not the app shell", path: "/downloads/campaign-exports/0123456789abcdef0123456789abcdef", wantStatus: http.StatusNotFound},
		{name: "healthz is a 404 here (it is registered elsewhere on the mux)", path: "/healthz", wantStatus: http.StatusNotFound},
		{name: "readyz is a 404 here (it is registered elsewhere on the mux)", path: "/readyz", wantStatus: http.StatusNotFound},
		{
			// http.ServeFile itself rejects any request path containing
			// "..", regardless of the file we asked it to serve: a real
			// second layer of defense on top of path.Clean.
			name: "path traversal is rejected outright", path: "/../../../etc/passwd", wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
			if tt.wantCache != "" {
				if got := rec.Header().Get("Cache-Control"); got != tt.wantCache {
					t.Errorf("Cache-Control = %q, want %q", got, tt.wantCache)
				}
			}
		})
	}
}

// TestStaticHandler_BrandFileTypes: the icons, the manifest and robots.txt go out with the type a
// browser expects, and revalidated (they keep their names, so they carry no hash).
func TestStaticHandler_BrandFileTypes(t *testing.T) {
	t.Parallel()

	dir := newTestBuild(t)
	writeFile(t, filepath.Join(dir, "favicon.svg"), `<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	writeFile(t, filepath.Join(dir, "site.webmanifest"), `{"name":"MeuRPG"}`)
	writeFile(t, filepath.Join(dir, "robots.txt"), "User-agent: *\nDisallow: /\n")
	handler, ok, _ := NewStatic(dir)
	if !ok {
		t.Fatal("NewStatic ok = false, want true")
	}

	tests := []struct{ path, wantType string }{
		{"/favicon.ico", "image/x-icon"},
		{"/favicon.svg", "image/svg+xml"},
		{"/site.webmanifest", "application/manifest+json"},
		{"/robots.txt", "text/plain; charset=utf-8"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != tt.wantType {
				t.Errorf("Content-Type = %q, want %q", got, tt.wantType)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
				t.Errorf("Cache-Control = %q, want no-cache", got)
			}
		})
	}
}

func TestStaticHandler_SecurityHeaders(t *testing.T) {
	t.Parallel()

	handler, ok, _ := NewStatic(newTestBuild(t))
	if !ok {
		t.Fatal("NewStatic ok = false, want true")
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if got := rec.Header().Get("Content-Security-Policy"); got != cspHeader(nil) {
		t.Errorf("Content-Security-Policy = %q, want %q", got, cspHeader(nil))
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Errorf("Referrer-Policy = %q, want same-origin", got)
	}
}

func TestStaticHandler_RejectsOtherMethods(t *testing.T) {
	t.Parallel()

	handler, ok, _ := NewStatic(newTestBuild(t))
	if !ok {
		t.Fatal("NewStatic ok = false, want true")
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

// TestStaticHandler_Integration wires NewStatic into a real Server, the way
// cmd/api/main.go does, to prove the mux's longest-pattern-wins routing
// really does give /healthz and /readyz priority over the "/" catch-all.
func TestStaticHandler_Integration(t *testing.T) {
	t.Parallel()

	static, ok, _ := NewStatic(newTestBuild(t))
	if !ok {
		t.Fatal("NewStatic ok = false, want true")
	}

	srv := New(Config{Addr: ":0", Logger: discardLogger()})
	srv.Handle("/", static)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("/healthz Cache-Control = %q, want no-store (the probe's, not the static handler's)", got)
	}
}

func TestStaticHandler_FormActionOrigin(t *testing.T) {
	t.Parallel()

	// The invite page posts to /auth/login, which redirects to the provider:
	// browsers check form-action on every hop, so the provider's origin must
	// be listed, reduced to scheme and host.
	handler, ok, err := NewStatic(newTestBuild(t), WithFormActionOrigin("https://accounts.google.com/o/oauth2/v2/auth?x=1"))
	if err != nil || !ok {
		t.Fatalf("NewStatic() = ok %v, err %v; want ok, nil", ok, err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "form-action 'self' https://accounts.google.com;") {
		t.Errorf("Content-Security-Policy = %q, want form-action 'self' https://accounts.google.com", csp)
	}
	if !strings.Contains(csp, "script-src 'self';") {
		t.Errorf("Content-Security-Policy = %q, want the rest of the policy unchanged", csp)
	}
}

func TestStaticHandler_FormActionOriginMustBeAbsolute(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{"", "accounts.google.com", "/relative", "javascript:alert(1)", "ftp://example.com"} {
		if _, ok, err := NewStatic(newTestBuild(t), WithFormActionOrigin(bad)); err == nil || ok {
			t.Errorf("NewStatic(WithFormActionOrigin(%q)) = ok %v, err %v; want an error", bad, ok, err)
		}
	}
}

// TestStaticHandler_PrecompressedAssets: a script or stylesheet goes out as its .br or .gz copy when the
// request accepts that coding, with the type and the caching of the original file.
func TestStaticHandler_PrecompressedAssets(t *testing.T) {
	t.Parallel()

	const (
		plain = "console.log('app')"
		br    = "fake-brotli-bytes"
		gz    = "fake-gzip-bytes"
	)

	tests := []struct {
		name         string
		method       string
		path         string
		acceptEnc    string
		withBr       bool
		withGz       bool
		wantEncoding string
		wantBody     string
		wantType     string
		wantCache    string
	}{
		{
			name: "br is preferred over gzip", path: "/main-ABCD1234.js", acceptEnc: "gzip, deflate, br", withBr: true, withGz: true,
			wantEncoding: "br", wantBody: br,
		},
		{
			name: "gzip when there is no br copy", path: "/main-ABCD1234.js", acceptEnc: "gzip, br", withGz: true,
			wantEncoding: "gzip", wantBody: gz,
		},
		{
			name: "gzip when the client does not accept br", path: "/main-ABCD1234.js", acceptEnc: "gzip", withBr: true, withGz: true,
			wantEncoding: "gzip", wantBody: gz,
		},
		{
			name: "q=0 refuses a coding", path: "/main-ABCD1234.js", acceptEnc: "br;q=0, gzip", withBr: true, withGz: true,
			wantEncoding: "gzip", wantBody: gz,
		},
		{
			name: "a negative or NaN q-value refuses a coding", path: "/main-ABCD1234.js", acceptEnc: "br;q=-1, gzip;q=NaN", withBr: true, withGz: true,
			wantEncoding: "", wantBody: plain,
		},
		{
			name: "q=0 on every coding gives the plain file", path: "/main-ABCD1234.js", acceptEnc: "br;q=0, gzip;q=0", withBr: true, withGz: true,
			wantBody: plain,
		},
		{
			name: "star accepts br", path: "/main-ABCD1234.js", acceptEnc: "*", withBr: true, withGz: true,
			wantEncoding: "br", wantBody: br,
		},
		{
			name: "star does not override a refused coding", path: "/main-ABCD1234.js", acceptEnc: "br;q=0, *", withBr: true, withGz: true,
			wantEncoding: "gzip", wantBody: gz,
		},
		{
			name: "star with q=0 accepts nothing", path: "/main-ABCD1234.js", acceptEnc: "*;q=0", withBr: true, withGz: true,
			wantBody: plain,
		},
		{
			name: "tokens are case-insensitive", path: "/main-ABCD1234.js", acceptEnc: "BR;Q=0.8", withBr: true,
			wantEncoding: "br", wantBody: br,
		},
		{
			name: "no Accept-Encoding gives the plain file", path: "/main-ABCD1234.js", withBr: true, withGz: true,
			wantBody: plain,
		},
		{
			name: "a missing copy falls back to the plain file", path: "/main-ABCD1234.js", acceptEnc: "br, gzip",
			wantBody: plain,
		},
		{
			name: "an unhashed stylesheet is encoded and revalidated", path: "/material-symbols/outlined.css", acceptEnc: "br", withBr: true,
			wantEncoding: "br", wantBody: br, wantType: "text/css; charset=utf-8", wantCache: "no-cache",
		},
		{
			name: "HEAD carries the headers and no body", method: http.MethodHead, path: "/main-ABCD1234.js", acceptEnc: "br", withBr: true,
			wantEncoding: "br", wantBody: "",
		},
		{
			name: "a .br file requested directly is a plain file", path: "/main-ABCD1234.js.br", acceptEnc: "br, gzip", withBr: true,
			wantBody: br, wantType: "text/plain; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := newTestBuild(t)
			for _, base := range []string{"main-ABCD1234.js", "material-symbols/outlined.css"} {
				if tt.withBr {
					writeFile(t, filepath.Join(dir, filepath.FromSlash(base)+".br"), br)
				}
				if tt.withGz {
					writeFile(t, filepath.Join(dir, filepath.FromSlash(base)+".gz"), gz)
				}
			}
			handler, ok, _ := NewStatic(dir)
			if !ok {
				t.Fatalf("NewStatic(%q) ok = false, want true", dir)
			}
			method := tt.method
			if method == "" {
				method = http.MethodGet
			}
			req := httptest.NewRequestWithContext(t.Context(), method, tt.path, nil)
			if tt.acceptEnc != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEnc)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Header().Get("Content-Encoding"); got != tt.wantEncoding {
				t.Errorf("Content-Encoding = %q, want %q", got, tt.wantEncoding)
			}
			if got := rec.Body.String(); got != tt.wantBody && method == http.MethodGet {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if method == http.MethodHead && rec.Body.Len() != 0 {
				t.Errorf("HEAD body = %q, want empty", rec.Body.String())
			}
			checkPrecompressedHeaders(t, rec, tt.path, tt.wantType, tt.wantCache)
		})
	}
}

// checkPrecompressedHeaders checks the headers that do not depend on the coding that was chosen.
func checkPrecompressedHeaders(t *testing.T, rec *httptest.ResponseRecorder, path, wantType, wantCache string) {
	t.Helper()

	if wantType == "" {
		wantType = "text/javascript; charset=utf-8"
	}
	if wantCache == "" {
		wantCache = "public, max-age=31536000, immutable"
	}
	if got := rec.Header().Get("Content-Type"); got != wantType {
		t.Errorf("Content-Type = %q, want %q", got, wantType)
	}
	if got := rec.Header().Get("Cache-Control"); got != wantCache {
		t.Errorf("Cache-Control = %q, want %q", got, wantCache)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	wantVary := "Accept-Encoding"
	if strings.HasSuffix(path, ".br") {
		wantVary = ""
	}
	if got := rec.Header().Get("Vary"); got != wantVary {
		t.Errorf("Vary = %q, want %q", got, wantVary)
	}
}

// TestStaticHandler_IndexIsNeverEncoded: the app shell goes out as it is even when a compressed copy of it exists.
func TestStaticHandler_IndexIsNeverEncoded(t *testing.T) {
	t.Parallel()

	dir := newTestBuild(t)
	writeFile(t, filepath.Join(dir, "index.html.br"), "fake-brotli-bytes")
	writeFile(t, filepath.Join(dir, "index.html.gz"), "fake-gzip-bytes")
	handler, ok, _ := NewStatic(dir)
	if !ok {
		t.Fatal("NewStatic ok = false, want true")
	}

	for _, path := range []string{"/", "/campaigns/42"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			req.Header.Set("Accept-Encoding", "br, gzip")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := rec.Header().Get("Content-Encoding"); got != "" {
				t.Errorf("Content-Encoding = %q, want none", got)
			}
			if got := rec.Body.String(); got != "<html>app shell</html>" {
				t.Errorf("body = %q, want the plain app shell", got)
			}
		})
	}
}

// TestStaticHandler_CannotLeaveTheDirectory: a request that climbs out of the build directory never gets
// a byte of a file next to it, plain or compressed.
func TestStaticHandler_CannotLeaveTheDirectory(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	dir := filepath.Join(parent, "web")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "index.html"), "<html>app shell</html>")
	writeFile(t, filepath.Join(parent, "secret.js"), "SECRET")
	writeFile(t, filepath.Join(parent, "secret.js.br"), "SECRET-BR")
	handler, ok, _ := NewStatic(dir)
	if !ok {
		t.Fatal("NewStatic ok = false, want true")
	}

	for _, target := range []string{"/../secret.js", "/%2e%2e/secret.js", "/..%2fsecret.js", "/sub/../../secret.js"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
			req.Header.Set("Accept-Encoding", "br")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if strings.Contains(rec.Body.String(), "SECRET") {
				t.Errorf("body = %q, leaked a file outside the directory", rec.Body.String())
			}
			if got := rec.Header().Get("Content-Encoding"); got != "" {
				t.Errorf("Content-Encoding = %q, want none", got)
			}
		})
	}
}

// TestStaticHandler_SymlinkOutOfTheDirectory: a symlink in the build directory that points outside it is
// served neither as a compressed copy nor as a file.
func TestStaticHandler_SymlinkOutOfTheDirectory(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	dir := filepath.Join(parent, "web")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "index.html"), "<html>app shell</html>")
	writeFile(t, filepath.Join(dir, "link-ABCD1234.js"), "console.log('plain')")
	writeFile(t, filepath.Join(parent, "outside.br"), "OUTSIDE")
	writeFile(t, filepath.Join(parent, "outside.txt"), "OUTSIDE")
	if err := os.Symlink(filepath.Join("..", "outside.br"), filepath.Join(dir, "link-ABCD1234.js.br")); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
	if err := os.Symlink(filepath.Join("..", "outside.txt"), filepath.Join(dir, "out.txt")); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
	handler, ok, _ := NewStatic(dir)
	if !ok {
		t.Fatal("NewStatic ok = false, want true")
	}

	for _, target := range []string{"/link-ABCD1234.js", "/out.txt"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
		req.Header.Set("Accept-Encoding", "br")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if strings.Contains(rec.Body.String(), "OUTSIDE") {
			t.Errorf("%s: body = %q, served a file outside the directory", target, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("%s: Content-Encoding = %q, want none", target, got)
		}
	}
}
