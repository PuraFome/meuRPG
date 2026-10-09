package blob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// The endpoints of the two services GCS talks to. The fields that hold them
// are unexported on purpose: only a test in this package can point the store
// somewhere else, so no environment variable can send the access token to
// another host.
const (
	gcsAPIBase      = "https://storage.googleapis.com"
	gcsMetadataBase = "http://metadata.google.internal"
	gcsTokenPath    = "/computeMetadata/v1/instance/service-accounts/default/token"
)

// Time limits. Every call has one; none waits on a server that went quiet.
const (
	gcsMetadataTimeout   = 5 * time.Second
	gcsHeaderTimeout     = 15 * time.Second
	gcsDeleteTimeout     = 15 * time.Second
	gcsCheckTimeout      = 15 * time.Second
	gcsPutTimeout        = 2 * time.Minute
	gcsOpenLifetime      = 2 * time.Minute
	gcsTokenMargin       = time.Minute
	gcsMaxTokenBody      = 64 << 10
	gcsRetryPause        = 100 * time.Millisecond
	gcsDefaultObjectType = "application/octet-stream"
)

// bucketName is Cloud Storage's rule for a bucket name: 3 to 222 characters
// of lowercase letters, digits, dashes, underscores and dots, starting and
// ending with a letter or a digit.
var bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,220}[a-z0-9]$`)

// GCS is a Store on a Cloud Storage bucket, over the JSON API with net/http.
// It lives on Cloud Run, where the access token of the service's own account
// comes from the metadata server; the three calls the app needs (upload,
// download, delete) do not justify the client library and its dependencies.
//
// An object is named by its key as it is. The bucket must have uniform
// bucket-level access and no public access (docs/operations.md#images);
// the API is the only reader.
//
// Neither the token nor a response body ever goes into an error or a log:
// an error says which call failed and with what status or network error.
type GCS struct {
	bucket string

	apiBase      string
	metadataBase string
	// client talks to the API; metadata talks to the metadata server, which
	// is on the local network and never through a proxy.
	client   *http.Client
	metadata *http.Client

	openLifetime time.Duration
	putTimeout   time.Duration
	now          func() time.Time
	pause        time.Duration

	mu      sync.Mutex // guards token and expires
	token   string
	expires time.Time
}

// The compiler checks that GCS implements Store.
var _ Store = (*GCS)(nil)

// NewGCS returns a Store on bucket. It makes no call: use Check to see
// whether the bucket can be reached.
func NewGCS(bucket string) (*GCS, error) {
	if !bucketName.MatchString(bucket) {
		return nil, errors.New("blob: BLOB_BUCKET is not a valid Cloud Storage bucket name")
	}
	api := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // the standard library's transport
	api.ResponseHeaderTimeout = gcsHeaderTimeout
	// The size of an object is the length of its answer: no transparent
	// gzip, which would hide it and shift every Range.
	api.DisableCompression = true
	return &GCS{
		bucket:       bucket,
		apiBase:      gcsAPIBase,
		metadataBase: gcsMetadataBase,
		client:       &http.Client{Transport: api},
		metadata:     &http.Client{Transport: &http.Transport{DisableKeepAlives: true}},
		openLifetime: gcsOpenLifetime,
		putTimeout:   gcsPutTimeout,
		now:          time.Now,
		pause:        gcsRetryPause,
	}, nil
}

// Check proves that the store can work: it reads a token from the metadata
// server and lists one object of the bucket, which needs the permissions the
// store uses. The server calls it once at start.
func (g *GCS) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, gcsCheckTimeout)
	defer cancel()
	resp, err := g.do(ctx, "check", http.MethodGet, g.bucketURL()+"/o?maxResults=1&fields=kind", "", "", nil, true) //nolint:bodyclose // closed below by drainAndClose
	if err != nil {
		return err
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return statusError("check", resp.StatusCode)
	}
	return nil
}

// Put implements Store with a media upload, which Cloud Storage applies
// whole: a reader sees the old object or the new one. It is not retried,
// because r cannot be read twice.
func (g *GCS) Put(ctx context.Context, key, contentType string, r io.Reader) error {
	if !ValidKey(key) {
		return ErrInvalidKey
	}
	if !validContentType(contentType) {
		return fmt.Errorf("blob: invalid content type %q", contentType)
	}
	if _, ok := r.(io.Closer); ok {
		// The HTTP client closes a body that can be closed; the reader is
		// the caller's.
		r = struct{ io.Reader }{r}
	}
	ctx, cancel := context.WithTimeout(ctx, g.putTimeout)
	defer cancel()
	target := g.apiBase + "/upload/storage/v1/b/" + url.PathEscape(g.bucket) + "/o?" +
		url.Values{"uploadType": {"media"}, "name": {key}}.Encode()
	resp, err := g.do(ctx, "put", http.MethodPost, target, contentType, "", r, false) //nolint:bodyclose // closed below by drainAndClose
	if err != nil {
		return err
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return statusError("put", resp.StatusCode)
	}
	return nil
}

// Open implements Store with a streamed alt=media read. The content is read
// from the network as the caller reads it; a Seek (http.ServeContent does
// one to learn the size, and one more for a range) makes the next Read ask
// for the bytes from the new offset with a Range header. The object's size
// and content type come from the first answer's headers.
func (g *GCS) Open(ctx context.Context, key string) (*Object, error) {
	if !ValidKey(key) {
		return nil, ErrInvalidKey
	}
	// The deadline covers the whole life of the Object, body included, and
	// is released by Close.
	ctx, cancel := context.WithTimeout(ctx, g.openLifetime)
	rd := &gcsReader{g: g, ctx: ctx, target: g.objectURL(key) + "?alt=media"}
	resp, err := g.do(ctx, "open", http.MethodGet, rd.target, "", "", nil, true) //nolint:bodyclose // closed by Object.Close, or below on every refusal
	if err != nil {
		cancel()
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		drainAndClose(resp.Body)
		cancel()
		return nil, ErrNotFound
	case resp.StatusCode != http.StatusOK:
		drainAndClose(resp.Body)
		cancel()
		return nil, statusError("open", resp.StatusCode)
	case resp.ContentLength < 0:
		drainAndClose(resp.Body)
		cancel()
		return nil, errors.New("blob: cloud storage open: the answer has no length")
	}
	rd.size = resp.ContentLength
	// A later Range read names this generation, so an object replaced in
	// between fails instead of giving a mix of two versions.
	rd.generation = resp.Header.Get("X-Goog-Generation")
	rd.body = resp.Body
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = gcsDefaultObjectType
	}
	return &Object{
		ContentType: contentType,
		Size:        rd.size,
		Content:     rd,
		close: func() error {
			rd.dropBody()
			cancel()
			return nil
		},
	}, nil
}

// Delete implements Store. A 404 is the blob already gone.
func (g *GCS) Delete(ctx context.Context, key string) error {
	if !ValidKey(key) {
		return ErrInvalidKey
	}
	ctx, cancel := context.WithTimeout(ctx, gcsDeleteTimeout)
	defer cancel()
	resp, err := g.do(ctx, "delete", http.MethodDelete, g.objectURL(key), "", "", nil, false) //nolint:bodyclose // closed below by drainAndClose
	if err != nil {
		return err
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return statusError("delete", resp.StatusCode)
	}
	return nil
}

func (g *GCS) bucketURL() string {
	return g.apiBase + "/storage/v1/b/" + url.PathEscape(g.bucket)
}

// objectURL is the object's resource URL. PathEscape turns the "/" of a key
// into %2F, as the API asks for object names in a path.
func (g *GCS) objectURL(key string) string {
	return g.bucketURL() + "/o/" + url.PathEscape(key)
}

// do sends one request to the API with the bearer token. With retry, a read
// that failed on the network or with a 5xx is sent once more. The caller
// closes the body of the response it gets.
func (g *GCS) do(ctx context.Context, op, method, target, contentType, byteRange string, body io.Reader, retry bool) (*http.Response, error) {
	attempts := 1
	if retry {
		attempts = 2
	}
	var lastErr error
	for attempt := range attempts {
		if attempt > 0 {
			select {
			case <-time.After(g.pause):
			case <-ctx.Done():
				return nil, fmt.Errorf("blob: cloud storage %s: %w", op, ctx.Err())
			}
		}
		resp, err := g.send(ctx, op, method, target, contentType, byteRange, body)
		switch {
		case err != nil:
			lastErr = err
			if ctx.Err() != nil {
				return nil, err
			}
		case resp.StatusCode >= http.StatusInternalServerError && attempt+1 < attempts:
			drainAndClose(resp.Body)
			lastErr = statusError(op, resp.StatusCode)
		default:
			if resp.StatusCode == http.StatusUnauthorized {
				// The token was refused: forget it, so the next call asks
				// the metadata server for a new one.
				g.forgetToken()
			}
			return resp, nil
		}
	}
	return nil, lastErr
}

func (g *GCS) send(ctx context.Context, op, method, target, contentType, byteRange string, body io.Reader) (*http.Response, error) {
	token, err := g.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, errors.New("blob: cloud storage " + op + ": cannot build the request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, networkError(op, err)
	}
	return resp, nil
}

// accessToken returns the service account's token, from the cache while it
// has more than a margin left, else from the metadata server.
func (g *GCS) accessToken(ctx context.Context) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.token != "" && g.now().Before(g.expires) {
		return g.token, nil
	}
	token, lifetime, err := g.fetchToken(ctx)
	if err != nil {
		return "", err
	}
	margin := min(gcsTokenMargin, lifetime/2)
	g.token, g.expires = token, g.now().Add(lifetime-margin)
	return token, nil
}

func (g *GCS) forgetToken() {
	g.mu.Lock()
	g.token, g.expires = "", time.Time{}
	g.mu.Unlock()
}

// fetchToken reads the token from the metadata server, once more after a
// network error or a 5xx.
func (g *GCS) fetchToken(ctx context.Context) (string, time.Duration, error) {
	var lastErr error
	for attempt := range 2 {
		if attempt > 0 {
			select {
			case <-time.After(g.pause):
			case <-ctx.Done():
				return "", 0, fmt.Errorf("blob: access token: %w", ctx.Err())
			}
		}
		token, lifetime, retry, err := g.fetchTokenOnce(ctx)
		if err == nil {
			return token, lifetime, nil
		}
		lastErr = err
		if !retry || ctx.Err() != nil {
			break
		}
	}
	return "", 0, lastErr
}

func (g *GCS) fetchTokenOnce(ctx context.Context) (token string, lifetime time.Duration, retry bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, gcsMetadataTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.metadataBase+gcsTokenPath, nil)
	if err != nil {
		return "", 0, false, errors.New("blob: access token: cannot build the request")
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := g.metadata.Do(req) //nolint:bodyclose // closed below by drainAndClose
	if err != nil {
		return "", 0, true, networkError("access token", err)
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", 0, resp.StatusCode >= http.StatusInternalServerError, statusError("access token", resp.StatusCode)
	}
	var answer struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, gcsMaxTokenBody)).Decode(&answer); err != nil {
		return "", 0, false, errors.New("blob: access token: the metadata server's answer cannot be read")
	}
	if answer.AccessToken == "" || answer.ExpiresIn <= 0 {
		return "", 0, false, errors.New("blob: access token: the metadata server's answer has no token")
	}
	return answer.AccessToken, time.Duration(answer.ExpiresIn) * time.Second, false, nil
}

// gcsReader reads an object from the network, from wherever it is asked to.
type gcsReader struct {
	g      *GCS
	ctx    context.Context
	target string
	size   int64

	generation string // of the object the first answer described; "" if not sent

	off     int64         // where the next Read starts
	body    io.ReadCloser // the open answer, at bodyOff; nil if none
	bodyOff int64
}

var _ io.ReadSeeker = (*gcsReader)(nil)

func (rd *gcsReader) Read(p []byte) (int, error) {
	if rd.off >= rd.size {
		return 0, io.EOF
	}
	if rd.body != nil && rd.bodyOff != rd.off {
		rd.dropBody()
	}
	if rd.body == nil {
		if err := rd.reopen(); err != nil {
			return 0, err
		}
	}
	n, err := rd.body.Read(p)
	rd.off += int64(n)
	rd.bodyOff += int64(n)
	if errors.Is(err, io.EOF) && rd.off < rd.size {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

func (rd *gcsReader) Seek(offset int64, whence int) (int64, error) {
	var base int64
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		base = rd.off
	case io.SeekEnd:
		base = rd.size
	default:
		return 0, errors.New("blob: invalid seek")
	}
	if next := base + offset; next >= 0 {
		rd.off = next
		return next, nil
	}
	return 0, errors.New("blob: negative seek position")
}

// reopen asks for the object from rd.off on.
func (rd *gcsReader) reopen() error {
	byteRange := ""
	if rd.off > 0 {
		byteRange = "bytes=" + strconv.FormatInt(rd.off, 10) + "-"
	}
	target := rd.target
	if rd.generation != "" && url.QueryEscape(rd.generation) == rd.generation {
		target += "&generation=" + rd.generation
	}
	resp, err := rd.g.do(rd.ctx, "read", http.MethodGet, target, "", byteRange, nil, true) //nolint:bodyclose // kept as rd.body, or closed below on every refusal
	if err != nil {
		return err
	}
	want := http.StatusOK
	if byteRange != "" {
		want = http.StatusPartialContent
	}
	if resp.StatusCode != want || resp.ContentLength != rd.size-rd.off {
		drainAndClose(resp.Body)
		if resp.StatusCode != want {
			return statusError("read", resp.StatusCode)
		}
		return errors.New("blob: cloud storage read: the object changed while it was read")
	}
	rd.body, rd.bodyOff = resp.Body, rd.off
	return nil
}

func (rd *gcsReader) dropBody() {
	if rd.body != nil {
		_ = rd.body.Close()
		rd.body = nil
	}
}

// networkError drops the request URL that net/http puts in its errors (it
// holds the bucket and the key, and keys hold IDs, which never go to the
// logs); errors.Is still sees a deadline or a cancellation.
func networkError(op string, err error) error {
	if ue, ok := errors.AsType[*url.Error](err); ok {
		err = ue.Err
	}
	return fmt.Errorf("blob: cloud storage %s: %w", op, err)
}

func statusError(op string, status int) error {
	return fmt.Errorf("blob: cloud storage %s: status %d", op, status)
}

// drainAndClose reads a little of what is left, so the connection can be
// reused, and closes the body.
func drainAndClose(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 4<<10))
	_ = body.Close()
}
