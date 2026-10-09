package campaignpackage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage/packagedb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/safego"
)

// sweepEvery is how often a running server looks for what expired. A server
// that scales to zero does not run it between requests, so Sweep also runs
// when the server starts, and the cloud store's own lifecycle rule (see
// docs/operations.md) is the last word on the exports.
const (
	sweepEvery = 10 * time.Minute
	sweepBatch = 50
)

// RunSweeper deletes what expired until ctx ends: the finished exports 24
// hours after they were done, and the parts of the uploads an hour after the
// last one. It deletes the file first and the row after, so a failure leaves a
// row that is tried again, never a file nobody names.
func (s *Service) RunSweeper(ctx context.Context) {
	if s.blobs == nil {
		return
	}
	go func() {
		defer safego.Recover(s.logger, "sweep the campaign packages")
		for {
			s.Sweep(ctx)
			select {
			case <-ctx.Done():
				return
			case <-time.After(sweepEvery):
			}
		}
	}()
}

// Sweep does one pass.
func (s *Service) Sweep(ctx context.Context) {
	if s.blobs == nil {
		return
	}
	now := s.now()
	for {
		rows, err := s.queries.ListExpiredCampaignExports(ctx, packagedb.ListExpiredCampaignExportsParams{ExpiresAt: now, Limit: sweepBatch})
		if err != nil {
			s.logger.WarnContext(ctx, "campaignpackage: cannot list the expired exports", "error", err)
			return
		}
		for _, e := range rows {
			s.deleteExport(ctx, e)
		}
		if len(rows) < sweepBatch {
			break
		}
	}
	for {
		rows, err := s.queries.ListExpiredCampaignImports(ctx, packagedb.ListExpiredCampaignImportsParams{ExpiresAt: now, Limit: sweepBatch})
		if err != nil {
			s.logger.WarnContext(ctx, "campaignpackage: cannot list the expired uploads", "error", err)
			return
		}
		for _, imp := range rows {
			s.deleteParts(ctx, imp)
			if err := s.deleteImportRow(ctx, imp.ID); err != nil {
				s.logger.WarnContext(ctx, "campaignpackage: cannot delete an expired upload", "import_id", imp.ID, "error", err)
			}
		}
		if len(rows) < sweepBatch {
			break
		}
	}
}

func (s *Service) deleteImportRow(ctx context.Context, id string) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.queries.WithTx(tx).DeleteCampaignImport(ctx, id)
	})
}

// deleteExport deletes an export's file, then its row.
func (s *Service) deleteExport(ctx context.Context, e packagedb.CampaignExport) {
	s.stopExport(e.ID)
	if err := s.blobs.Delete(ctx, exportKey(e.CampaignID, e.ID)); err != nil {
		s.logger.WarnContext(ctx, "campaignpackage: cannot delete an export's file", "export_id", e.ID, "error", err)
		return // the row stays, and the next pass tries again
	}
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.queries.WithTx(tx).DeleteCampaignExport(ctx, e.ID)
	})
	if err != nil {
		s.logger.WarnContext(ctx, "campaignpackage: cannot delete an export's row", "export_id", e.ID, "error", err)
	}
}

// DeleteCampaignPackages deletes every export of a campaign, files first. The
// code that deletes a campaign calls it, so a package never outlives the
// campaign it came from; the sweeper would delete the file within a day anyway.
func (s *Service) DeleteCampaignPackages(ctx context.Context, campaignID string) error {
	if s.blobs == nil {
		return nil
	}
	rows, err := s.queries.ListCampaignExportsOfCampaign(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list the campaign's exports: %w", err)
	}
	for _, e := range rows {
		s.stopExport(e.ID)
		if err := s.blobs.Delete(ctx, exportKey(e.CampaignID, e.ID)); err != nil {
			return fmt.Errorf("delete an export's file: %w", err)
		}
	}
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.queries.WithTx(tx).DeleteCampaignExportsOfCampaign(ctx, campaignID)
	})
}

// DeleteUserPackages deletes the exports a person asked for and the upload they
// have, files first. The code that deletes an account calls it.
func (s *Service) DeleteUserPackages(ctx context.Context, userID string) error {
	if s.blobs == nil {
		return nil
	}
	exports, err := s.queries.ListCampaignExportsOfUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("list the person's exports: %w", err)
	}
	for _, e := range exports {
		s.stopExport(e.ID)
		if err := s.blobs.Delete(ctx, exportKey(e.CampaignID, e.ID)); err != nil {
			return fmt.Errorf("delete an export's file: %w", err)
		}
	}
	uploads, err := s.queries.ListCampaignImportsOfUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("list the person's uploads: %w", err)
	}
	for _, u := range uploads {
		s.deleteParts(ctx, u)
	}
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		if err := q.DeleteCampaignExportsOfUser(ctx, userID); err != nil {
			return fmt.Errorf("delete the person's exports: %w", err)
		}
		for _, u := range uploads {
			if err := q.DeleteCampaignImport(ctx, u.ID); err != nil {
				return fmt.Errorf("delete the person's upload: %w", err)
			}
		}
		return nil
	})
}
