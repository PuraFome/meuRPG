package characters

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// ContentSource gives the rules content a campaign plays with (MR-025, RN-23;
// ADR-0018). Every read of the content in this package goes through it, so
// the table's own content (slice 10.1c) can differ per campaign without
// touching the call sites again.
type ContentSource interface {
	// For returns the rules content and the table rules of a campaign. tx is
	// the caller's transaction (nil outside one): an implementation that reads
	// the campaign's content revision does it in tx, never through the pool
	// while a transaction is open (PR #121's rule).
	For(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, TableRules, error)
}

// TableRules are the rules a table chooses (RN-23). Its zero value is the SRD
// defaults, so a campaign that chose nothing is TableRules{}: each field says
// what a table changes, never what the SRD already does. It lives here, not in
// its own package, because only this module reads it so far; nothing reads a
// field yet, and the defaults are the behavior the code already has.
type TableRules struct {
	// HitPoints is what a player may do with the hit die of a level-up. The zero
	// value is HitPointsPlayerChooses: roll or take the average.
	HitPoints HitPointsRule
	// AbilityMethodsOff are the ways of generating ability scores the table does
	// not allow. The zero value allows all of them.
	AbilityMethodsOff AbilityMethods
	// CriticalMaxPlusRoll makes a critical hit the maximum of the dice plus a
	// roll, instead of the SRD's doubled dice.
	CriticalMaxPlusRoll bool
	// DeathSavesHidden hides the other characters' death saves from the players,
	// instead of the SRD's visible ones.
	DeathSavesHidden bool
	// Reminders is the list of the table's reminders; none by default.
	Reminders []string
}

// HitPointsRule is how a level-up's hit points are decided. The zero value is
// the SRD's: the player chooses.
type HitPointsRule string

// The hit points rules.
const (
	// HitPointsPlayerChooses lets the player pick rolling or the average.
	HitPointsPlayerChooses HitPointsRule = ""
)

// AbilityMethods is a set of ways of generating ability scores.
type AbilityMethods struct {
	StandardArray, PointBuy, Roll bool
}

// DefaultTableRules returns the SRD defaults: the zero value.
func DefaultTableRules() TableRules { return TableRules{} }

// SRDSource is the ContentSource that gives every campaign the one SRD
// content and the default table rules. It never touches the database.
type SRDSource struct{ content *rules.Content }

// NewSRDSource returns the SRD source over content (rules.LoadSRD).
func NewSRDSource(content *rules.Content) SRDSource { return SRDSource{content: content} }

// For implements ContentSource.
func (s SRDSource) For(context.Context, pgx.Tx, string) (*rules.Content, TableRules, error) {
	return s.content, DefaultTableRules(), nil
}

// contentFor is the content of a campaign, read in tx (nil outside one).
func (s *Service) contentFor(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, error) {
	c, _, err := s.content.For(ctx, tx, campaignID)
	return c, err
}

// maxCatalogs is how many contents' catalogs are kept: the plan's budget of
// cached contents (D1). The oldest is dropped first, so the catalogs of the
// contents a table edited away do not pile up.
const maxCatalogs = 8

// catalogCache is ListContent's answer per content, built on first use and
// bounded to maxCatalogs, so a content that is no longer in use is not kept alive.
type catalogCache struct {
	mu    sync.Mutex
	order []*rules.Content // oldest first
	// catalog is the master's catalog (every entry); players is the same without
	// the archived entries, made when a player first asks.
	catalog map[*rules.Content]*rulesv1.Content
	players map[*rules.Content]*rulesv1.Content
}

// catalogFor is ListContent's answer for a content: the same content gives the
// same catalog, built once (while it is among the last maxCatalogs).
func (s *Service) catalogFor(c *rules.Content, master bool) *rulesv1.Content {
	cc := &s.catalogs
	cc.mu.Lock()
	defer cc.mu.Unlock()
	v, ok := cc.catalog[c]
	if !ok {
		v = catalogToProto(c.Catalog())
		if cc.catalog == nil {
			cc.catalog, cc.players = map[*rules.Content]*rulesv1.Content{}, map[*rules.Content]*rulesv1.Content{}
		}
		if len(cc.order) >= maxCatalogs {
			delete(cc.catalog, cc.order[0])
			delete(cc.players, cc.order[0])
			cc.order = cc.order[1:]
		}
		cc.order = append(cc.order, c)
		cc.catalog[c] = v
	}
	if master {
		return v
	}
	p, ok := cc.players[c]
	if !ok {
		p = withoutArchived(v)
		cc.players[c] = p
	}
	return p
}

// withoutArchived is a catalog without the entries the table retired, for the
// players (RN-23): a copy that shares everything else. A class's list of
// subclasses lives in the SRD content, so it needs no change here.
func withoutArchived(c *rulesv1.Content) *rulesv1.Content {
	out := &rulesv1.Content{
		ContentVersion: c.ContentVersion, Attribution: c.Attribution, Abilities: c.Abilities, Skills: c.Skills,
		Armor: c.Armor, Weapons: c.Weapons, ChallengeRatings: c.ChallengeRatings,
	}
	for _, r := range c.Races {
		if !r.GetArchived() {
			out.Races = append(out.Races, r)
		}
	}
	for _, r := range c.Subraces {
		if !r.GetArchived() {
			out.Subraces = append(out.Subraces, r)
		}
	}
	for _, r := range c.Classes {
		if !r.GetArchived() {
			out.Classes = append(out.Classes, r)
		}
	}
	for _, r := range c.Subclasses {
		if !r.GetArchived() {
			out.Subclasses = append(out.Subclasses, r)
		}
	}
	for _, r := range c.Backgrounds {
		if !r.GetArchived() {
			out.Backgrounds = append(out.Backgrounds, r)
		}
	}
	for _, r := range c.Spells {
		if !r.GetArchived() {
			out.Spells = append(out.Spells, r)
		}
	}
	return out
}
