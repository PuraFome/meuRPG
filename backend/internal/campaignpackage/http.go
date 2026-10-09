package campaignpackage

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage/packagedb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/slowclient"
)

// partReadTimeout is how long a client has to send one part: 5 MiB in two
// minutes is under 350 kbit/s, which a phone on a bad connection reaches.
const partReadTimeout = 2 * time.Minute

// downloadMinRate is the slowest connection a download waits for, in bytes per
// second (1 Mbit/s); downloadFloor and downloadCeiling bound the wait.
const (
	downloadMinRate = 125_000
	downloadFloor   = 2 * time.Minute
	downloadCeiling = 30 * time.Minute
)

// handlePart serves PUT /uploads/campaign-imports/{id}/parts/{n}: the raw
// bytes of part n of the caller's upload. The same part sent again replaces
// itself, so a retry after a lost answer is safe. It answers 204.
//
// CSRF: the route changes state and is not a Connect RPC, so the Connect
// protocol header does not protect it; http.CrossOriginProtection, around the
// whole server, refuses a cross-origin write before it gets here, and the
// session cookie is SameSite=Lax.
func (s *Service) handlePart(w http.ResponseWriter, r *http.Request) {
	if s.blobs == nil {
		s.writeError(w, r, errImagesOffHTTP())
		return
	}
	if err := s.part(w, r); err != nil {
		s.writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func errImagesOffHTTP() *httpError {
	return &httpError{status: http.StatusServiceUnavailable, code: connect.CodeUnavailable, reason: "IMAGES_OFF", message: "packages are off: the server has no store for files"}
}

func (s *Service) part(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	userID, err := authz.RequireSignedIn(ctx)
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 1 {
		return notFound()
	}
	imp, err := s.ownUpload(ctx, userID, r.PathValue("id"))
	if err != nil {
		return notFound() // someone else's upload reads as one that does not exist
	}
	if n > int(imp.PartCount) {
		return notFound()
	}
	if err := s.allow(ctx, s.partsLim, s.partsN, userID); err != nil {
		return rateLimited(err)
	}
	want := partLength(imp.TotalBytes, n)
	slowclient.ReadBody(w, partReadTimeout)
	// One byte past the length is enough to know the body is too long.
	body := http.MaxBytesReader(w, r.Body, want+1)
	counter := &countingReader{r: body}
	if err := s.blobs.Put(ctx, partKey(imp.ID, n), "application/octet-stream", io.LimitReader(counter, want+1)); err != nil {
		if _, tooBig := errors.AsType[*http.MaxBytesError](err); tooBig {
			return wrongLength()
		}
		return fmt.Errorf("store a part: %w", err)
	}
	if counter.n != want {
		// Short or long: the part is not what the upload said it would be. Drop it.
		_ = s.blobs.Delete(ctx, partKey(imp.ID, n))
		return wrongLength()
	}
	now := s.now()
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		// The upload may have been canceled or replaced while the part came.
		cur, err := q.GetCampaignImport(ctx, imp.ID)
		if err != nil {
			return fmt.Errorf("find the upload: %w", err)
		}
		if err := q.UpsertCampaignImportPart(ctx, packagedb.UpsertCampaignImportPartParams{
			ImportID: cur.ID, PartNumber: int32(n), ByteSize: int32(want), CreatedAt: now, //nolint:gosec // G115: at most PartSize
		}); err != nil {
			return fmt.Errorf("record the part: %w", err)
		}
		return q.TouchCampaignImport(ctx, packagedb.TouchCampaignImportParams{ID: cur.ID, UpdatedAt: now, ExpiresAt: now.Add(UploadTTL)})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_ = s.blobs.Delete(ctx, partKey(imp.ID, n))
		return notFound()
	}
	return err
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func wrongLength() *httpError {
	return &httpError{status: http.StatusBadRequest, code: connect.CodeInvalidArgument, reason: "WRONG_LENGTH", message: "the part is not the length the upload says"}
}

// rateLimited turns the limiter's Connect error into the route's 429.
func rateLimited(error) error {
	return &httpError{status: http.StatusTooManyRequests, code: connect.CodeResourceExhausted, reason: "RATE_LIMITED", message: "too many requests, please wait a moment and try again", retryAfter: 1}
}

// handleDownload serves GET /downloads/campaign-exports/{id}. The id is 128
// random bits, and it is not what protects the file: the route checks that the
// caller is the master of the export's campaign, again, on every request. Any
// refusal is the same 404, so the route does not tell a stranger which ids
// exist.
func (s *Service) handleDownload(w http.ResponseWriter, r *http.Request) {
	if err := s.download(w, r); err != nil {
		s.writeError(w, r, err)
	}
}

func (s *Service) download(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	if s.blobs == nil {
		return errImagesOffHTTP()
	}
	userID, err := authz.RequireSignedIn(ctx)
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	if !validExportID(id) {
		return notFound()
	}
	row, err := s.queries.GetCampaignExport(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notFound()
		}
		return fmt.Errorf("find the export: %w", err)
	}
	// The master of the campaign now, not the person who asked for the export.
	if _, err := authz.RequireCampaignRole(ctx, row.CampaignID, authz.RoleMaster); err != nil {
		if ce, ok := errors.AsType[*connect.Error](err); ok && ce.Code() == connect.CodeUnauthenticated {
			return err
		}
		return notFound()
	}
	if row.State != "done" || !row.ExpiresAt.After(s.now()) {
		return notFound()
	}
	if err := s.allow(ctx, s.heavy, s.heavyN, userID); err != nil {
		return rateLimited(err)
	}
	obj, err := s.blobs.Open(ctx, row.BlobKey)
	if errors.Is(err, blob.ErrNotFound) {
		return notFound()
	}
	if err != nil {
		return fmt.Errorf("open the export: %w", err)
	}
	defer func() { _ = obj.Close() }()

	h := w.Header()
	h.Set("Content-Type", "application/zip")
	h.Set("Content-Disposition", `attachment; filename="`+row.FileName+`"`)
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	wait := min(max(time.Duration(obj.Size/downloadMinRate)*time.Second, downloadFloor), downloadCeiling)
	defer slowclient.WriteBody(w, wait)()
	http.ServeContent(w, r, "", time.Time{}, obj.Content)
	return nil
}
