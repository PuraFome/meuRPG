package characters

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/tablerules"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// ContentSource gives the rules content and the table rules a campaign plays
// with (MR-025, RN-23, RN-24; ADR-0018). Every read of the content in this
// package goes through it, so the table's own content (slice 10.1c) can differ
// per campaign without touching the call sites again.
type ContentSource interface {
	// For returns the rules content and the table rules of a campaign. tx is
	// the caller's transaction (nil outside one): an implementation that reads
	// the campaign's content revision does it in tx, never through the pool
	// while a transaction is open (PR #121's rule).
	For(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, TableRules, error)
	// ContentFor is For for a caller that only needs the content: it skips the read
	// of the table rules, which is a database read per call.
	ContentFor(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, error)
}

// TableRules are the rules a table chooses (RN-23, RN-24): the shared value of
// package tablerules. Its zero value is the SRD defaults, so a campaign that
// chose nothing is TableRules{}.
type TableRules = tablerules.Rules

// HitPointsRule is how a level-up's hit points are decided. The zero value is the
// SRD's: the player chooses.
type HitPointsRule = tablerules.HitPointsRule

// The hit points rules.
const (
	// HitPointsPlayerChooses lets the player pick rolling or the average.
	HitPointsPlayerChooses = tablerules.HitPointsPlayerChooses
	// HitPointsRoll makes everybody roll; the average is refused.
	HitPointsRoll = tablerules.HitPointsRoll
	// HitPointsAverage makes everybody take the average; a roll is refused.
	HitPointsAverage = tablerules.HitPointsAverage
)

// AbilityMethods is a set of ways of making ability scores.
type AbilityMethods = tablerules.AbilityMethods

// DefaultTableRules returns the SRD defaults: the zero value.
func DefaultTableRules() TableRules { return TableRules{} }

// SRDSource is the ContentSource that gives every campaign the one SRD
// content and the default table rules. It never touches the database.
type SRDSource struct{ content *rules.Content }

// NewSRDSource returns the SRD source over content (rules.LoadSRD).
func NewSRDSource(content *rules.Content) SRDSource { return SRDSource{content: content} }

// ContentFor implements ContentSource.
func (s SRDSource) ContentFor(context.Context, pgx.Tx, string) (*rules.Content, error) {
	return s.content, nil
}

// For implements ContentSource.
func (s SRDSource) For(context.Context, pgx.Tx, string) (*rules.Content, TableRules, error) {
	return s.content, DefaultTableRules(), nil
}

// TableRulesReader reads the rules a table saved (package campaigns implements
// it: campaigns.Service.StoredTableRules).
type TableRulesReader interface {
	// StoredTableRules returns the table rules of a campaign, the defaults when
	// it never saved any. It reads inside tx when the caller has one (nil: the
	// pool).
	StoredTableRules(ctx context.Context, tx pgx.Tx, campaignID string) (TableRules, error)
}

// TableSource is the ContentSource that gives every campaign the SRD content and
// the table rules its master saved (MR-025, RN-24). The table's own content
// (RN-23) joins in slice 10.1c. It reads the rules through the tx it receives
// (PR #121): one primary-key read of campaign_table_rules.
type TableSource struct {
	content *rules.Content
	tables  TableRulesReader
}

// NewTableSource returns the source over content (rules.LoadSRD) and the
// reader of the saved rules.
func NewTableSource(content *rules.Content, tables TableRulesReader) TableSource {
	return TableSource{content: content, tables: tables}
}

// ContentFor implements ContentSource: no read of the rules.
func (s TableSource) ContentFor(context.Context, pgx.Tx, string) (*rules.Content, error) {
	return s.content, nil
}

// For implements ContentSource.
func (s TableSource) For(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, TableRules, error) {
	r, err := s.tables.StoredTableRules(ctx, tx, campaignID)
	if err != nil {
		return nil, TableRules{}, fmt.Errorf("read the table rules: %w", err)
	}
	return s.content, r, nil
}

// contentFor is the content of a campaign, read in tx (nil outside one), without
// the table rules (a caller that needs them asks s.content.For).
func (s *Service) contentFor(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, error) {
	return s.content.ContentFor(ctx, tx, campaignID)
}

// TableRulesFor is the table rules of a campaign, read in tx (nil outside one).
// Package play reads the critical and the death saves through it in slice 10.4b
// (cmd/api connects them), and nothing else needs the content with it.
func (s *Service) TableRulesFor(ctx context.Context, tx pgx.Tx, campaignID string) (TableRules, error) {
	_, r, err := s.content.For(ctx, tx, campaignID)
	return r, err
}

// maxCatalogs is how many contents' catalogs are kept: the plan's budget of
// cached contents (D1). The oldest is dropped first, so the catalogs of the
// contents a table edited away do not pile up.
const maxCatalogs = 8

// catalogCache is ListContent's answer per content, built on first use and
// bounded to maxCatalogs, so a content that is no longer in use is not kept alive.
type catalogCache struct {
	mu      sync.Mutex
	order   []*rules.Content // oldest first
	catalog map[*rules.Content]*rulesv1.Content
}

// catalogFor is ListContent's answer for a content: the same content gives the
// same catalog, built once (while it is among the last maxCatalogs).
func (s *Service) catalogFor(c *rules.Content) *rulesv1.Content {
	cc := &s.catalogs
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if v, ok := cc.catalog[c]; ok {
		return v
	}
	v := catalogToProto(c.Catalog())
	if cc.catalog == nil {
		cc.catalog = map[*rules.Content]*rulesv1.Content{}
	}
	if len(cc.order) >= maxCatalogs {
		delete(cc.catalog, cc.order[0])
		cc.order = cc.order[1:]
	}
	cc.order = append(cc.order, c)
	cc.catalog[c] = v
	return v
}
