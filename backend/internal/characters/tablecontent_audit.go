package characters

import (
	"context"
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
			logger.ErrorContext(ctx, "table content audit: campaign not checked", slog.String("campaign_id", id), slog.Any("error", err))
			continue
		}
		report.LeftOut += n
	}
	logger.InfoContext(ctx, "table content audit done",
		slog.Int("campaigns", report.Campaigns), slog.Int("left_out", report.LeftOut), slog.Int("failed", report.Failed))
	return report, nil
}

// auditCampaign logs the entries of one campaign the rules would leave out, and counts them.
func auditCampaign(ctx context.Context, pool *pgxpool.Pool, srd *rules.Content, logger *slog.Logger, id string) (int, error) {
	var refused []rules.Refused
	kinds := map[string]string{}
	err := db.InTx(ctx, pool, func(tx pgx.Tx) error {
		q := charactersdb.New(pool).WithTx(tx)
		revision, err := q.GetContentRevision(ctx, id)
		if err != nil {
			return fmt.Errorf("read the content revision: %w", err)
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
			return err
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
