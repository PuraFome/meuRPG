package campaignpackage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage/packagedb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
)

// exportMargin is what an estimate adds to the gallery's bytes for the data
// (the JSON entries and the zip's own records).
const exportMargin = 1 << 20

// exportIDLength is the length of an export id: 128 bits as hex digits.
const exportIDLength = 32

// newExportID is 128 random bits as 32 hex digits.
func newExportID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("draw an export id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func exportToProto(e packagedb.CampaignExport) *pkgv1.CampaignExport {
	out := &pkgv1.CampaignExport{
		Id: e.ID, Percent: e.Percent, CreatedAt: timestamppb.New(e.CreatedAt),
	}
	switch e.State {
	case "running":
		out.State = pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_RUNNING
	case "done":
		out.State = pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_DONE
		out.FileName, out.ByteSize, out.EntryCount = e.FileName, e.ByteSize, e.EntryCount
		out.ExpiresAt = timestamppb.New(e.ExpiresAt)
		out.DownloadPath = DownloadsPath + e.ID
	case "failed":
		out.State = pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_FAILED
		out.Failure = pkgv1.CampaignExportFailure_CAMPAIGN_EXPORT_FAILURE_INTERRUPTED
		if e.Failure == "too_big" {
			out.Failure = pkgv1.CampaignExportFailure_CAMPAIGN_EXPORT_FAILURE_TOO_BIG
		}
	case "canceled":
		out.State = pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_CANCELED
	}
	if e.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*e.FinishedAt)
	}
	return out
}

// StartCampaignExport implements campaignpackagev1connect.CampaignPackageServiceHandler.
func (s *Service) StartCampaignExport(
	ctx context.Context,
	req *connect.Request[pkgv1.StartCampaignExportRequest],
) (*connect.Response[pkgv1.StartCampaignExportResponse], error) {
	in, err := s.checkExportStart(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	var row packagedb.CampaignExport
	started := false
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		row, started, err = s.startExport(ctx, s.queries.WithTx(tx), in)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "start an export", err)
	}
	if started {
		s.launchExport(row.ID, in.campaignID)
	}
	return connect.NewResponse(&pkgv1.StartCampaignExportResponse{Export: exportToProto(row)}), nil
}

// exportStart is a StartCampaignExport request that passed every check, with
// the ID a new export would get.
type exportStart struct {
	campaignID string
	userID     string
	// scoped and hash are the idempotency key, unique for the master in the
	// campaign, and the hash of the request.
	scoped, hash *string
	id           string
}

// checkExportStart makes StartCampaignExport's checks, in order: the master,
// the store, the idempotency key and the rate limit.
func (s *Service) checkExportStart(ctx context.Context, msg *pkgv1.StartCampaignExportRequest) (exportStart, error) {
	m, err := authz.RequireCampaignRole(ctx, msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return exportStart{}, err
	}
	if s.blobs == nil {
		return exportStart{}, errImagesOff()
	}
	key, err := idem.Clean(msg.GetIdempotencyKey())
	if err != nil {
		return exportStart{}, err
	}
	if err := s.allow(ctx, s.heavy, s.heavyN, m.UserID); err != nil {
		return exportStart{}, err
	}
	scoped, hash := idem.Scope(m.CampaignID+":"+m.UserID, key), idem.Hash(msg)
	id, err := newExportID()
	if err != nil {
		return exportStart{}, s.dbError(ctx, "start an export", err)
	}
	return exportStart{campaignID: m.CampaignID, userID: m.UserID, scoped: scoped, hash: hash, id: id}, nil
}

// startExport is StartCampaignExport's transaction: the export of the key,
// or the one running for the campaign, or a new one. started is true for a
// new one, which the caller launches.
func (s *Service) startExport(ctx context.Context, q *packagedb.Queries, in exportStart) (row packagedb.CampaignExport, started bool, err error) {
	row, _, err = idem.Create(ctx, in.scoped, in.hash, q.GetCampaignExportByCreateKey,
		func(e packagedb.CampaignExport) *string { return e.CreateHash },
		func() (packagedb.CampaignExport, error) {
			e, inserted, err := s.insertExportUnlessRunning(ctx, q, in)
			started = inserted
			return e, err
		})
	return row, started, err
}

// insertExportUnlessRunning inserts a new export, unless one is already
// running for the campaign: that one is the answer, since two would only do
// the same work twice. inserted is true for a new row.
func (s *Service) insertExportUnlessRunning(ctx context.Context, q *packagedb.Queries, in exportStart) (e packagedb.CampaignExport, inserted bool, err error) {
	if running, err := q.GetRunningCampaignExport(ctx, in.campaignID); err == nil {
		return running, false, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return packagedb.CampaignExport{}, false, fmt.Errorf("find a running export: %w", err)
	}
	now := s.now()
	e, err = q.InsertCampaignExport(ctx, packagedb.InsertCampaignExportParams{
		ID: in.id, CampaignID: in.campaignID, RequestedBy: in.userID, CreateKey: in.scoped, CreateHash: in.hash,
		Now: now, ExpiresAt: now.Add(ExportTTL),
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return e, false, fmt.Errorf("insert an export: %w", err)
	}
	return e, err == nil && e.ID == in.id, err
}

// GetCampaignExport implements campaignpackagev1connect.CampaignPackageServiceHandler.
func (s *Service) GetCampaignExport(
	ctx context.Context,
	req *connect.Request[pkgv1.GetCampaignExportRequest],
) (*connect.Response[pkgv1.GetCampaignExportResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	if s.blobs == nil {
		return nil, errImagesOff()
	}
	now := s.now()
	if err := s.failStale(ctx, now); err != nil {
		return nil, s.dbError(ctx, "read the export", err)
	}
	res := &pkgv1.GetCampaignExportResponse{LimitBytes: MaxPackageBytes}
	row, err := s.queries.GetLatestCampaignExport(ctx, packagedb.GetLatestCampaignExportParams{CampaignID: m.CampaignID, Now: now})
	switch {
	case err == nil:
		res.Export = exportToProto(row)
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, s.dbError(ctx, "read the export", err)
	}
	res.EstimatedBytes = exportMargin
	for _, p := range s.parts {
		est, ok := p.(Estimator)
		if !ok {
			continue
		}
		n, err := est.EstimateBytes(ctx, m.CampaignID)
		if err != nil {
			return nil, s.dbError(ctx, "estimate the export", err)
		}
		res.EstimatedBytes += n
	}
	return connect.NewResponse(res), nil
}

// failStale gives up on the exports that stopped moving: a restart killed them.
func (s *Service) failStale(ctx context.Context, now time.Time) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := s.queries.WithTx(tx).FailStaleCampaignExports(ctx, packagedb.FailStaleCampaignExportsParams{
			Now: now, ExpiresAt: now.Add(failedExportTTL), StaleBefore: now.Add(-staleExportAfter),
		})
		if err != nil {
			return fmt.Errorf("fail the stale exports: %w", err)
		}
		return nil
	})
}

// CancelCampaignExport implements campaignpackagev1connect.CampaignPackageServiceHandler.
func (s *Service) CancelCampaignExport(
	ctx context.Context,
	req *connect.Request[pkgv1.CancelCampaignExportRequest],
) (*connect.Response[pkgv1.CancelCampaignExportResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	id := req.Msg.GetExportId()
	if !validExportID(id) {
		return nil, errExportNotFound()
	}
	var row packagedb.CampaignExport
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		cur, err := q.GetCampaignExportForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && cur.CampaignID != m.CampaignID) {
			return errExportNotFound()
		}
		if err != nil {
			return fmt.Errorf("find the export: %w", err)
		}
		row = cur
		if cur.State != "running" {
			return nil // finished already: nothing to stop
		}
		now := s.now()
		row, err = q.CancelCampaignExport(ctx, packagedb.CancelCampaignExportParams{ID: id, UpdatedAt: now, ExpiresAt: now.Add(failedExportTTL)})
		if err != nil {
			return fmt.Errorf("cancel the export: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "cancel an export", err)
	}
	s.stopExport(id)
	return connect.NewResponse(&pkgv1.CancelCampaignExportResponse{Export: exportToProto(row)}), nil
}

func errExportNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("export not found"))
}

// validExportID is the shape of an export id: 32 lowercase hex digits.
func validExportID(id string) bool {
	if len(id) != exportIDLength {
		return false
	}
	for i := range len(id) {
		if c := id[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
