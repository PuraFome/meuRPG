package campaignpackage

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage/packagedb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/platform/safego"
)

// exportKey is where an export's zip lives: under the campaign's prefix, with
// the rest of its files.
func exportKey(campaignID, exportID string) string {
	return "campaigns/" + campaignID + "/exports/" + exportID + ".zip"
}

// progressEvery is the least time between two writes of the export's progress.
const progressEvery = time.Second

// launchExport runs the export in the background. The export is a row first,
// so the page can ask about it at once and a restart leaves a trace.
func (s *Service) launchExport(exportID, campaignID string) {
	ctx, cancel := context.WithTimeout(context.Background(), exportTimeout)
	s.mu.Lock()
	s.running[exportID] = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			cancel()
			s.mu.Lock()
			delete(s.running, exportID)
			s.mu.Unlock()
		}()
		defer safego.Recover(s.logger, "export a campaign", func() { s.failExport(exportID, "interrupted") })
		s.runExport(ctx, exportID, campaignID)
	}()
}

// stopExport cancels an export running in this process.
func (s *Service) stopExport(exportID string) {
	s.mu.Lock()
	cancel := s.running[exportID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// errNoLongerRunning ends an export that was canceled.
var errNoLongerRunning = errors.New("campaignpackage: the export was canceled")

func (s *Service) runExport(ctx context.Context, exportID, campaignID string) {
	// One export at a time: each holds a snapshot and a zip being written.
	select {
	case s.exportSlot <- struct{}{}:
		defer func() { <-s.exportSlot }()
	case <-ctx.Done():
		s.failExport(exportID, "interrupted")
		return
	}
	start := s.now()
	size, entries, name, err := s.writeExport(ctx, exportID, campaignID)
	switch {
	case err == nil:
		s.finishExport(exportID, name, size, entries, start)
	case errors.Is(err, errNoLongerRunning):
		_ = s.blobs.Delete(context.WithoutCancel(ctx), exportKey(campaignID, exportID))
	case errors.Is(err, ErrTooBig):
		_ = s.blobs.Delete(context.WithoutCancel(ctx), exportKey(campaignID, exportID))
		s.failExport(exportID, "too_big")
	default:
		s.logger.ErrorContext(ctx, "campaignpackage: an export failed", "export_id", exportID, "error", err)
		_ = s.blobs.Delete(context.WithoutCancel(ctx), exportKey(campaignID, exportID))
		s.failExport(exportID, "interrupted")
	}
}

// snapshot reads the campaign, in one transaction: the same moment for every
// table. The transaction may run again after a conflict, so each attempt starts
// a new snapshot.
func (s *Service) snapshot(ctx context.Context, campaignID string) (*Snapshot, error) {
	var snap *Snapshot
	err := db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		snap = NewSnapshot(campaignID)
		for _, p := range s.parts {
			if err := p.Export(ctx, tx, campaignID, snap); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read the campaign: %w", err)
	}
	return snap, nil
}

// writeExport reads the campaign and streams the zip into the blob store. It
// returns the zip's size, its number of entries and the file name.
func (s *Service) writeExport(ctx context.Context, exportID, campaignID string) (size int64, entries int, fileName string, err error) {
	snap, err := s.snapshot(ctx, campaignID)
	if err != nil {
		return 0, 0, "", err
	}
	if snap.Entries()+1 > MaxEntries || snap.BlobBytes() > MaxPackageBytes {
		return 0, 0, "", ErrTooBig
	}
	pr, pw := io.Pipe()
	var written *Writer
	werr := make(chan error, 1)
	go func() {
		defer safego.Recover(s.logger, "write an export's zip", func() { _ = pw.CloseWithError(errors.New("campaignpackage: the zip writer panicked")) })
		w, err := s.writeZip(ctx, pw, exportID, snap)
		written = w
		_ = pw.CloseWithError(err)
		werr <- err
	}()
	putErr := s.blobs.Put(ctx, exportKey(campaignID, exportID), "application/zip", pr)
	_ = pr.CloseWithError(putErr) // lets the writer stop when the store gave up
	zipErr := <-werr
	switch {
	case zipErr != nil:
		return 0, 0, "", zipErr
	case putErr != nil:
		return 0, 0, "", fmt.Errorf("store the export: %w", putErr)
	}
	return written.Written(), written.Entries() + 1, FileName(snap.CampaignName), nil
}

// writeZip writes the snapshot as a package to w, updating the progress.
func (s *Service) writeZip(ctx context.Context, dst io.Writer, exportID string, snap *Snapshot) (*Writer, error) {
	w := NewWriter(dst, s.now())
	total := snap.BlobBytes() + 1 // +1: never divide by zero
	var done int64
	lastReport := time.Time{}
	report := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Since(lastReport) < progressEvery {
			return nil
		}
		lastReport = time.Now()
		pct := int32(min(99, done*100/total)) //nolint:gosec // G115: 0 to 99
		stillRunning, err := s.reportProgress(ctx, exportID, pct)
		if err != nil {
			return err
		}
		if !stillRunning {
			return errNoLongerRunning
		}
		return nil
	}
	for _, e := range snap.entries {
		if err := report(); err != nil {
			return w, err
		}
		if e.blob == "" {
			if err := w.AddBytes(e.kind, e.name, e.data, zip.Deflate); err != nil {
				return w, err
			}
			continue
		}
		obj, err := s.blobs.Open(ctx, e.blob)
		if err != nil {
			return w, fmt.Errorf("open a file of the campaign: %w", err)
		}
		err = w.AddFile(e.kind, e.name, obj.Content)
		_ = obj.Close()
		if err != nil {
			return w, err
		}
		done += e.size
	}
	if err := w.Close(s.version, snap.CampaignName); err != nil {
		return w, err
	}
	return w, nil
}

// reportProgress writes the percentage and says whether the export is still
// running (a cancel from another request changes the row).
func (s *Service) reportProgress(ctx context.Context, exportID string, pct int32) (bool, error) {
	running := true
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		row, err := q.GetCampaignExport(ctx, exportID)
		if err != nil {
			return fmt.Errorf("read the export: %w", err)
		}
		running = row.State == "running"
		if !running {
			return nil
		}
		return q.SetCampaignExportProgress(ctx, packagedb.SetCampaignExportProgressParams{ID: exportID, Percent: pct, UpdatedAt: s.now()})
	})
	if err != nil {
		return false, fmt.Errorf("report the progress: %w", err)
	}
	return running, nil
}

func (s *Service) finishExport(exportID, fileName string, size int64, entries int, started time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var row packagedb.CampaignExport
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		cur, err := q.GetCampaignExport(ctx, exportID)
		if err != nil {
			return fmt.Errorf("read the export: %w", err)
		}
		if cur.State != "running" {
			row = cur
			return nil
		}
		now := s.now()
		row, err = q.FinishCampaignExport(ctx, packagedb.FinishCampaignExportParams{
			ID: exportID, FileName: fileName, BlobKey: exportKey(cur.CampaignID, exportID), ByteSize: size, EntryCount: int32(entries), //nolint:gosec // G115: at most 2,000
			UpdatedAt: now, ExpiresAt: now.Add(ExportTTL),
		})
		if err != nil {
			return fmt.Errorf("finish the export: %w", err)
		}
		return nil
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "campaignpackage: cannot record a finished export", "export_id", exportID, "error", err)
		return
	}
	if row.State != "done" {
		// Canceled while the last bytes were being stored: the file is not wanted.
		_ = s.blobs.Delete(ctx, exportKey(row.CampaignID, exportID))
		return
	}
	logging.Event(ctx, s.logger, "campaign.exported", slog.String("campaign_id", row.CampaignID), slog.Int64("bytes", size), slog.Int("entries", entries), slog.Duration("took", s.now().Sub(started)))
}

// failExport records that an export failed, for the reason failure
// ("too_big" or "interrupted").
func (s *Service) failExport(exportID, failure string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		now := s.now()
		_, err := s.queries.WithTx(tx).FailCampaignExport(ctx, packagedb.FailCampaignExportParams{
			ID: exportID, Failure: failure, UpdatedAt: now, ExpiresAt: now.Add(failedExportTTL),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // it was canceled, or already ended
		}
		return err
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "campaignpackage: cannot record a failed export", "export_id", exportID, "error", err)
	}
}
