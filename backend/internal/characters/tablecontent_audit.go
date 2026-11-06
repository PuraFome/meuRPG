package characters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// AuditReport is what AuditTableContent found.
type AuditReport struct {
	// Campaigns is how many campaigns with table content were checked.
	Campaigns int
	// LeftOut is how many entries the rules would leave out, over all campaigns.
	LeftOut int
	// Failed is how many campaigns could not be checked or would not build at all.
	Failed int
}

// AuditTableContent runs the stored table content of every campaign through the
// rules in srd, one campaign at a time, and logs what the rules would leave out.
// It only reads and keeps nothing: a campaign's content is dropped before the next.
// A line has the campaign ID, the entry key and kind and the reason code, never the entry's text.
// A campaign it cannot check is logged and counted, and the audit goes on.
// It returns an error when the list of campaigns cannot be read or ctx ends before the last campaign.
func AuditTableContent(ctx context.Context, pool *pgxpool.Pool, srd *rules.Content, logger *slog.Logger) (AuditReport, error) {
	q := charactersdb.New(pool)
	ids, err := q.ListCampaignsWithContent(ctx)
	if err != nil {
		return AuditReport{}, wrap("list the campaigns with table content", err)
	}
	var report AuditReport
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		report.Campaigns++
		n, err := auditCampaign(ctx, pool, srd, logger, id)
		if err != nil {
			report.Failed++
			// The error text can echo what a master typed: log the reason code only.
			logger.ErrorContext(ctx, "table content audit: campaign not checked", slog.String("campaign_id", id), slog.String("reason", auditFailureReason(err)))
			continue
		}
		report.LeftOut += n
	}
	logger.InfoContext(ctx, "table content audit done",
		slog.Int("campaigns", report.Campaigns), slog.Int("left_out", report.LeftOut), slog.Int("failed", report.Failed))
	return report, nil
}

// buildFailureError is a campaign whose content does not build, whatever is left out.
type buildFailureError struct{ reason string }

func (f *buildFailureError) Error() string { return "the content does not build: " + f.reason }

// auditFailureReason is the reason code of a campaign the audit could not check:
// the engine's, or "database" for a failed read.
func auditFailureReason(err error) string {
	if bf, ok := errors.AsType[*buildFailureError](err); ok {
		return bf.reason
	}
	return "database"
}

// auditCampaign logs the entries of one campaign the rules would leave out, and counts them.
func auditCampaign(ctx context.Context, pool *pgxpool.Pool, srd *rules.Content, logger *slog.Logger, id string) (int, error) {
	var refused []rules.Refused
	kinds := map[string]string{}
	err := db.ReadTx(ctx, pool, func(tx pgx.Tx) error {
		q := charactersdb.New(pool).WithTx(tx)
		revision, err := contentRevision(ctx, q, id)
		if err != nil {
			return err
		}
		rows, err := q.ListCampaignContent(ctx, id)
		if err != nil {
			return fmt.Errorf("read the table's content: %w", err)
		}
		off, err := q.ListContentOff(ctx, id)
		if err != nil {
			return fmt.Errorf("read the options switched off: %w", err)
		}
		for _, r := range rows {
			kinds[r.ContentKey] = r.Kind
		}
		c, err := buildFromRows(srd, rows, int(revision), off)
		if err != nil {
			return &buildFailureError{reason: buildFailureReason(err)}
		}
		refused = c.Refused()
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, r := range refused {
		logger.WarnContext(ctx, "table content audit: entry would be left out",
			slog.String("campaign_id", id), slog.String("entry_key", r.Key),
			slog.String("kind", kinds[r.Key]), slog.String("reason", r.Reason))
	}
	return len(refused), nil
}

// buildFailureReason is the reason code of the first violation of err, without its message.
func buildFailureReason(err error) string {
	if oe, ok := asOverlayError(err); ok {
		if vs := oe.Violations(); len(vs) > 0 && vs[0].Reason != "" {
			return vs[0].Reason
		}
	}
	return rules.ReasonOverlay
}
