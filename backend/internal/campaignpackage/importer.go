package campaignpackage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage/packagedb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
)

// readResult is what reading a package ended with: what it holds and what is
// wrong with it.
type readResult struct {
	manifest *pkgv1.PackageManifest
	name     string
	problems Problems
	counts   *pkgv1.PackageCounts
}

func (r *readResult) preview() *pkgv1.PreviewCampaignImportResponse {
	out := &pkgv1.PreviewCampaignImportResponse{Problems: r.problems.List, Counts: r.counts, CampaignName: r.name}
	if out.Counts == nil {
		out.Counts = &pkgv1.PackageCounts{}
	}
	if r.manifest != nil {
		out.FormatVersion = r.manifest.GetFormatVersion()
		out.ExportedAt = r.manifest.GetExportedAt()
		if out.CampaignName == "" {
			out.CampaignName = r.manifest.GetCampaignName()
		}
	}
	return out
}

// read opens the package of an upload and runs every part's Stage over it. With
// commit it then creates the campaign, in one transaction; without, it stops
// at the stage and writes nothing. In both, the files already written to the
// blob store are deleted when the package is refused or the import fails.
func (s *Service) read(ctx context.Context, imp packagedb.CampaignImport, userID string, createKey, createHash *string, commit bool) (*readResult, error) {
	complete, err := s.uploadComplete(ctx, imp)
	if err != nil {
		return nil, err
	}
	if !complete {
		return nil, blocked(pkgv1.CampaignPackageBlockedReason_CAMPAIGN_PACKAGE_BLOCKED_REASON_INCOMPLETE, "some parts of the upload have not arrived", nil)
	}
	reader := newPartsReader(ctx, s.blobs, imp.ID, imp.TotalBytes)
	defer func() { _ = reader.Close() }()
	res := &readResult{}
	pkg, problems := Open(reader, imp.TotalBytes)
	if pkg == nil {
		res.problems.List = problems
		return res, nil
	}
	res.manifest = pkg.Manifest

	in := NewImport(pkg, uuid.New().String(), userID, commit, s.blobs)
	in.CreateKey, in.CreateHash = createKey, createHash
	cleanup := func() { s.deleteBlobs(context.WithoutCancel(ctx), in.Written) }

	for _, p := range s.parts {
		if err := p.Stage(ctx, in); err != nil {
			cleanup()
			return nil, s.stageError(ctx, err)
		}
	}
	in.IDs.Unresolved(&in.Problems)
	res.problems, res.counts, res.name = in.Problems, in.Counts, in.CampaignName
	if in.Problems.Any() {
		cleanup()
		return res, nil
	}
	if !commit {
		return res, nil
	}
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		for _, p := range s.parts {
			if err := p.Apply(ctx, tx, in); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, ErrReplayed) {
		cleanup() // the other call's campaign stands; its files are its own
		return res, nil
	}
	if err != nil {
		cleanup()
		return nil, s.dbError(ctx, "create a campaign from a package", err)
	}
	logging.Event(ctx, s.logger, "campaign.imported",
		slog.String("campaign_id", in.CampaignID), slog.Int("images", int(in.Counts.GetImages())), slog.Int("maps", int(in.Counts.GetMaps())))
	return res, nil
}

// stageError is the answer to a part that could not even read its entries (a
// store or a database failure): the cause goes to the log, the client gets a
// fixed sentence.
func (s *Service) stageError(ctx context.Context, err error) error {
	if ce, ok := errors.AsType[*connect.Error](err); ok {
		return ce
	}
	return s.dbError(ctx, "read a package", fmt.Errorf("stage: %w", err))
}

// deleteBlobs deletes the files an import wrote before it ended without a
// campaign. A key that fails is logged: it names an id nobody has.
func (s *Service) deleteBlobs(ctx context.Context, keys []string) {
	for _, k := range keys {
		if err := s.blobs.Delete(ctx, k); err != nil {
			s.logger.WarnContext(ctx, "campaignpackage: cannot delete a file of an import that did not end", "key", k, "error", err)
		}
	}
}
