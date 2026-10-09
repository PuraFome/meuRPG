package campaignpackage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage/packagedb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
)

const (
	maxFileNameLength    = 120
	maxFingerprintLength = 200
)

func invalid(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}

func errImportNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("upload not found"))
}

func importToProto(imp packagedb.CampaignImport, parts []packagedb.CampaignImportPart) *pkgv1.CampaignImport {
	out := &pkgv1.CampaignImport{
		Id: imp.ID, FileName: imp.FileName, TotalBytes: imp.TotalBytes, PartSize: imp.PartSize, PartCount: imp.PartCount,
		ExpiresAt: timestamppb.New(imp.ExpiresAt),
	}
	for _, p := range parts {
		out.ReceivedParts = append(out.ReceivedParts, p.PartNumber)
	}
	return out
}

// BeginCampaignImport implements campaignpackagev1connect.CampaignPackageServiceHandler.
func (s *Service) BeginCampaignImport(
	ctx context.Context,
	req *connect.Request[pkgv1.BeginCampaignImportRequest],
) (*connect.Response[pkgv1.BeginCampaignImportResponse], error) {
	userID, err := authz.RequireSignedIn(ctx)
	if err != nil {
		return nil, err
	}
	if s.blobs == nil {
		return nil, errImagesOff()
	}
	name, err := cleanFileName(req.Msg.GetFileName())
	if err != nil {
		return nil, err
	}
	fingerprint := strings.TrimSpace(req.Msg.GetFingerprint())
	if n := utf8.RuneCountInString(fingerprint); n < 1 || n > maxFingerprintLength || !utf8.ValidString(fingerprint) {
		return nil, invalid("fingerprint must be 1 to 200 characters")
	}
	total := req.Msg.GetTotalBytes()
	if total < 1 || total > MaxPackageBytes {
		return nil, invalid("total_bytes must be 1 byte to 200 MiB")
	}
	if err := s.allow(ctx, s.heavy, s.heavyN, userID); err != nil {
		return nil, err
	}
	// Refused before a single byte goes up: the cap and the creators list.
	if err := s.creation.CheckCreation(ctx, userID); err != nil {
		return nil, err
	}
	now := s.now()
	var imp packagedb.CampaignImport
	var parts []packagedb.CampaignImportPart
	var dropped []packagedb.CampaignImport
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		dropped, parts = nil, nil
		cur, err := q.GetCampaignImportOfUser(ctx, userID)
		switch {
		case err == nil && cur.Fingerprint == fingerprint && cur.TotalBytes == total && cur.ExpiresAt.After(now):
			// The same file again: its parts are still good.
			imp = cur
			if parts, err = q.ListCampaignImportParts(ctx, cur.ID); err != nil {
				return fmt.Errorf("list the parts: %w", err)
			}
			return q.TouchCampaignImport(ctx, packagedb.TouchCampaignImportParams{ID: cur.ID, UpdatedAt: now, ExpiresAt: now.Add(UploadTTL)})
		case err == nil:
			// Another file: the one-at-a-time rule drops the first.
			dropped = append(dropped, cur)
			if err := q.DeleteCampaignImport(ctx, cur.ID); err != nil {
				return fmt.Errorf("drop the previous upload: %w", err)
			}
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("find the upload: %w", err)
		}
		imp, err = q.InsertCampaignImport(ctx, packagedb.InsertCampaignImportParams{
			ID: uuid.New().String(), UserID: userID, FileName: name, Fingerprint: fingerprint, TotalBytes: total,
			PartSize: PartSize, PartCount: int32(PartCount(total)), //nolint:gosec // G115: at most 40
			CreatedAt: now, ExpiresAt: now.Add(UploadTTL),
		})
		if err != nil {
			return fmt.Errorf("insert the upload: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "begin an import", err)
	}
	for _, d := range dropped {
		s.deleteParts(context.WithoutCancel(ctx), d)
	}
	return connect.NewResponse(&pkgv1.BeginCampaignImportResponse{Upload: importToProto(imp, parts)}), nil
}

// cleanFileName checks the name the app shows for the file.
func cleanFileName(raw string) (string, error) {
	name, err := names.Clean(raw, maxFileNameLength)
	if err != nil {
		return "", invalid("file_name " + err.Error())
	}
	return name, nil
}

// GetCampaignImport implements campaignpackagev1connect.CampaignPackageServiceHandler.
func (s *Service) GetCampaignImport(
	ctx context.Context,
	_ *connect.Request[pkgv1.GetCampaignImportRequest],
) (*connect.Response[pkgv1.GetCampaignImportResponse], error) {
	userID, err := authz.RequireSignedIn(ctx)
	if err != nil {
		return nil, err
	}
	if s.blobs == nil {
		return nil, errImagesOff()
	}
	var imp packagedb.CampaignImport
	var parts []packagedb.CampaignImportPart
	found := false
	err = db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		found = false
		var err error
		if imp, err = q.GetCampaignImportOfUser(ctx, userID); errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return fmt.Errorf("find the upload: %w", err)
		}
		found = true
		if parts, err = q.ListCampaignImportParts(ctx, imp.ID); err != nil {
			return fmt.Errorf("list the parts: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "read an import", err)
	}
	res := &pkgv1.GetCampaignImportResponse{}
	if found && imp.ExpiresAt.After(s.now()) {
		res.Upload, res.Fingerprint = importToProto(imp, parts), imp.Fingerprint
	}
	return connect.NewResponse(res), nil
}

// CancelCampaignImport implements campaignpackagev1connect.CampaignPackageServiceHandler.
func (s *Service) CancelCampaignImport(
	ctx context.Context,
	req *connect.Request[pkgv1.CancelCampaignImportRequest],
) (*connect.Response[pkgv1.CancelCampaignImportResponse], error) {
	userID, err := authz.RequireSignedIn(ctx)
	if err != nil {
		return nil, err
	}
	imp, err := s.ownUpload(ctx, userID, req.Msg.GetImportId())
	if err != nil {
		return nil, err
	}
	if err := s.dropUpload(ctx, imp); err != nil {
		return nil, s.dbError(ctx, "cancel an import", err)
	}
	return connect.NewResponse(&pkgv1.CancelCampaignImportResponse{}), nil
}

// ownUpload finds an upload that is the person's own and has not expired. An
// upload of someone else is "not found", like one that does not exist.
func (s *Service) ownUpload(ctx context.Context, userID, id string) (packagedb.CampaignImport, error) {
	if !ValidID(id) {
		return packagedb.CampaignImport{}, errImportNotFound()
	}
	imp, err := s.queries.GetCampaignImport(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (imp.UserID != userID || !imp.ExpiresAt.After(s.now()))) {
		return packagedb.CampaignImport{}, errImportNotFound()
	}
	if err != nil {
		return packagedb.CampaignImport{}, s.dbError(ctx, "find an import", err)
	}
	return imp, nil
}

// dropUpload deletes an upload: the row first (nobody can reach the parts
// without it), then the parts.
func (s *Service) dropUpload(ctx context.Context, imp packagedb.CampaignImport) error {
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.queries.WithTx(tx).DeleteCampaignImport(ctx, imp.ID)
	})
	if err != nil {
		return fmt.Errorf("delete the upload: %w", err)
	}
	s.deleteParts(context.WithoutCancel(ctx), imp)
	return nil
}

// deleteParts deletes the blobs of every part an upload could have. Deleting a
// key that is not there is not an error, so the whole range is safe.
func (s *Service) deleteParts(ctx context.Context, imp packagedb.CampaignImport) {
	for n := 1; n <= int(imp.PartCount); n++ {
		if err := s.blobs.Delete(ctx, partKey(imp.ID, n)); err != nil {
			s.logger.WarnContext(ctx, "campaignpackage: cannot delete a part of an upload", "import_id", imp.ID, "part", n, "error", err)
		}
	}
}

// PreviewCampaignImport implements campaignpackagev1connect.CampaignPackageServiceHandler.
func (s *Service) PreviewCampaignImport(
	ctx context.Context,
	req *connect.Request[pkgv1.PreviewCampaignImportRequest],
) (*connect.Response[pkgv1.PreviewCampaignImportResponse], error) {
	userID, err := authz.RequireSignedIn(ctx)
	if err != nil {
		return nil, err
	}
	if s.blobs == nil {
		return nil, errImagesOff()
	}
	imp, err := s.ownUpload(ctx, userID, req.Msg.GetImportId())
	if err != nil {
		return nil, err
	}
	if err := s.allow(ctx, s.heavy, s.heavyN, userID); err != nil {
		return nil, err
	}
	run, err := s.read(ctx, imp, userID, nil, nil, false)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(run.preview()), nil
}

// CreateCampaignFromImport implements campaignpackagev1connect.CampaignPackageServiceHandler.
func (s *Service) CreateCampaignFromImport(
	ctx context.Context,
	req *connect.Request[pkgv1.CreateCampaignFromImportRequest],
) (*connect.Response[pkgv1.CreateCampaignFromImportResponse], error) {
	userID, err := authz.RequireSignedIn(ctx)
	if err != nil {
		return nil, err
	}
	if s.blobs == nil {
		return nil, errImagesOff()
	}
	key, err := idem.Clean(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, invalid("idempotency_key is required")
	}
	hash := idem.Hash(req.Msg)
	// A retry of a create that worked finds its campaign, even with the upload
	// gone and the account at its cap.
	if c, ok, err := s.creation.FindCreated(ctx, userID, key, hash); err != nil {
		return nil, err
	} else if ok {
		return connect.NewResponse(&pkgv1.CreateCampaignFromImportResponse{Campaign: c, Counts: &pkgv1.PackageCounts{}}), nil
	}
	imp, err := s.ownUpload(ctx, userID, req.Msg.GetImportId())
	if err != nil {
		return nil, err
	}
	if err := s.allow(ctx, s.heavy, s.heavyN, userID); err != nil {
		return nil, err
	}
	if err := s.creation.CheckCreation(ctx, userID); err != nil {
		return nil, err
	}
	run, err := s.read(ctx, imp, userID, idem.Scope(userID, key), hash, true)
	if err != nil {
		return nil, err
	}
	if run.problems.Any() {
		return nil, blocked(pkgv1.CampaignPackageBlockedReason_CAMPAIGN_PACKAGE_BLOCKED_REASON_HAS_PROBLEMS, "the package has problems", run.preview())
	}
	created, ok, err := s.creation.FindCreated(ctx, userID, key, hash)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, s.dbError(ctx, "read the new campaign", errors.New("the campaign just created is missing"))
	}
	// The package is in the campaign now: the upload is done with.
	if err := s.dropUpload(context.WithoutCancel(ctx), imp); err != nil {
		s.logger.WarnContext(ctx, "campaignpackage: cannot delete a finished upload", "import_id", imp.ID, "error", err)
	}
	return connect.NewResponse(&pkgv1.CreateCampaignFromImportResponse{Campaign: created, Counts: run.counts}), nil
}

// uploadComplete reports whether every part of the upload arrived, whole.
func (s *Service) uploadComplete(ctx context.Context, imp packagedb.CampaignImport) (bool, error) {
	parts, err := s.queries.ListCampaignImportParts(ctx, imp.ID)
	if err != nil {
		return false, s.dbError(ctx, "list the parts", err)
	}
	if len(parts) != int(imp.PartCount) {
		return false, nil
	}
	for i, p := range parts {
		if int(p.PartNumber) != i+1 || int64(p.ByteSize) != partLength(imp.TotalBytes, i+1) {
			return false, nil
		}
	}
	return true, nil
}

func nowOr(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
