package tasks

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/PuraFome/meuRPG/backend/internal/platform/safego"
)

// Where Google signs the OIDC tokens that Cloud Tasks attaches: the issuer, and the keys of the
// service accounts (https://www.googleapis.com/oauth2/v3/certs, as the documentation of Cloud
// Tasks' OIDC tokens and Google's own libraries use). The fields of tokenChecker hold them so
// that only a test in this package can point the check at another issuer.
const (
	googleIssuer = "https://accounts.google.com"
	googleJWKS   = "https://www.googleapis.com/oauth2/v3/certs"
)

// tokenChecker accepts only a token that Google signed for audience and whose e-mail is the
// service account's, verified.
type tokenChecker struct {
	verifier *oidc.IDTokenVerifier
	email    string
}

func newTokenChecker(issuer, jwksURL, audience, email string, client *http.Client, now func() time.Time) *tokenChecker {
	// The key set fetches (and caches) the keys for the life of the process, so its context is
	// not a request's.
	ctx := context.Background()
	if client != nil {
		ctx = oidc.ClientContext(ctx, client)
	}
	keys := oidc.NewRemoteKeySet(ctx, jwksURL)
	return &tokenChecker{
		verifier: oidc.NewVerifier(issuer, keys, &oidc.Config{ClientID: audience, SupportedSigningAlgs: []string{oidc.RS256}, Now: now}),
		email:    email,
	}
}

// valid says whether the Authorization header carries a good token: signature, issuer, audience
// and expiry (go-oidc), then the e-mail of the account and that Google verified it.
func (t *tokenChecker) valid(ctx context.Context, header string) bool {
	raw, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || raw == "" {
		return false
	}
	token, err := t.verifier.Verify(ctx, raw)
	if err != nil {
		return false
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := token.Claims(&claims); err != nil {
		return false
	}
	return claims.EmailVerified && claims.Email == t.email
}

// Handler serves the tasks: POST EndpointPrefix + kind. The check of the token is the only gate
// (the load balancer forwards every path to the service, so this is reachable from the internet):
// anything that fails it, a wrong method included, answers 401 with an empty body and no detail,
// and the token is never logged. Then it runs the handler of the kind, with the request's context
// limited to the kind's Deadline:
//   - nil: 200 (the task is deleted);
//   - Permanent(err): 200, logged (a retry cannot help);
//   - any other error, or a panic: 500 (Cloud Tasks retries under the queue's settings).
//
// A delivery of a scheduled kind first enqueues the next slot of its chain.
func (c *CloudTasks) Handler() http.Handler {
	return http.HandlerFunc(c.serve)
}

// Mount registers the endpoint with a server's Handle.
func (c *CloudTasks) Mount(handle func(pattern string, h http.Handler)) {
	handle(EndpointPrefix, c.Handler())
}

func (c *CloudTasks) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost || !c.check.valid(r.Context(), r.Header.Get("Authorization")) {
		c.logger.InfoContext(r.Context(), "tasks: a call to the task endpoint was refused")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	kind := Kind(strings.TrimPrefix(r.URL.Path, EndpointPrefix))
	e, ok := c.lookup(kind)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPayloadBytes))
	if err != nil || (len(payload) > 0 && !json.Valid(payload)) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(c.run(r, kind, e, payload))
}

// run does one delivery and answers the status for it.
func (c *CloudTasks) run(r *http.Request, kind Kind, e entry, payload json.RawMessage) int {
	c.running.Add(1)
	defer c.running.Add(-1)
	ctx, cancel := context.WithTimeout(r.Context(), e.opts.Deadline)
	defer cancel()
	// The shutdown (Cancel) ends the handler too.
	defer context.AfterFunc(c.base, cancel)()
	retries := r.Header.Get("X-CloudTasks-TaskRetryCount")

	if e.interval > 0 {
		// The next slot first: a handler that fails or is cut off must not break the chain.
		if err := c.enqueueNext(ctx, kind); err != nil {
			c.logger.WarnContext(ctx, "tasks: the next sweep could not be scheduled", "kind", string(kind), "error", err)
			return http.StatusInternalServerError
		}
	}
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}
	err := c.call(ctx, kind, e, payload)
	switch {
	case err == nil:
		return http.StatusOK
	case IsPermanent(err):
		c.logger.InfoContext(ctx, "tasks: a task ended with a failure a retry cannot help", "kind", string(kind), "error", err)
		return http.StatusOK
	default:
		c.logger.WarnContext(ctx, "tasks: a task failed", "kind", string(kind), "retry_count", retries, "error", err)
		return http.StatusInternalServerError
	}
}

// call runs the handler; a panic becomes an error (a retry may help).
func (c *CloudTasks) call(ctx context.Context, kind Kind, e entry, payload json.RawMessage) (err error) {
	defer safego.Recover(c.logger, "task "+string(kind), func() { err = errPanicked })
	return e.h(ctx, payload)
}
