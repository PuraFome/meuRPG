package campaignpackage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage/packagedb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

// sweepEvery is how often the scheduled task looks for what expired. The cloud store's
// own lifecycle rule (see docs/operations/reference.md, "Campaign package") is the last
// word on the exports.
const (
	sweepEvery = 10 * time.Minute
	sweepBatch = 50
)

// sweepDeadline is how long one pass may run.
const sweepDeadline = 5 * time.Minute

// handleSweep is the handler of sweepKind: one pass. A pass is idempotent by nature (it
// deletes what expired, and a second one finds nothing more), so a duplicate is harmless.
func (s *Service) handleSweep(ctx context.Context, _ json.RawMessage) error {
	s.Sweep(ctx)
	return nil
}

// RunSweeper starts the scheduled kinds of the runner this service owns (the sweep: every
// sweepEvery, and once now). With a runner of the caller, whose Start the caller calls, it
// does nothing. The pass deletes what expired: the finished exports 24 hours after they
// were done, and the parts of the uploads an hour after the last one. It deletes the file
// first and the row after, so a failure leaves a row that is tried again, never a file
// nobody names.
func (s *Service) RunSweeper(ctx context.Context) {
	if s.ownTasks != nil && s.blobs != nil {
		s.ownTasks.Start(ctx)
	}
}

// Sweep does one pass.
func (s *Service) Sweep(ctx context.Context) {
	if s.blobs == nil {
		return
	}
	now := s.now()
	if !s.sweepExports(ctx, now) {
		return
	}
	s.sweepImports(ctx, now)
}

// sweepExports deletes the expired exports, a batch at a time. It reports
// false when it could not list them, which ends the pass.
func (s *Service) sweepExports(ctx context.Context, now time.Time) bool {
	for {
		rows, err := s.queries.ListExpiredCampaignExports(ctx, packagedb.ListExpiredCampaignExportsParams{ExpiresAt: now, Limit: sweepBatch})
		if err != nil {
			s.logger.WarnContext(ctx, "campaignpackage: cannot list the expired exports", "error", err)
			return false
		}
		for _, e := range rows {
			s.deleteExport(ctx, e)
		}
		if len(rows) < sweepBatch {
			return true
		}
	}
}

// sweepImports deletes the expired uploads, parts first, a batch at a time.
func (s *Service) sweepImports(ctx context.Context, now time.Time) {
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
			return
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
