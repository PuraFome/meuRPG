package play

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// How far a spell cast outside a combat reaches (SRD 5.1, "Spell Range": Touch is within
// reach, 5 ft; a ranged spell its distance). On the session's current map the server
// measures the squares between the tokens (5 ft a square); without a map, a grid or a
// token the master judges the distance and nothing is refused (as in a combat without a map).

// tokenSquares are the squares where the characters' tokens stand on the session's current
// map; empty without a map, a grid or tokens.
func (s *Service) tokenSquares(ctx context.Context, tx pgx.Tx, session playdb.GameSession) (map[string]grid.Square, error) {
	out := map[string]grid.Square{}
	if session.CurrentMapID == nil {
		return out, nil
	}
	g, err := s.maps.MapGrid(ctx, tx, session.CampaignID, *session.CurrentMapID)
	if err != nil || !g.OK() {
		return out, nil //nolint:nilerr // a map that is gone, or has no grid, has no distances
	}
	tokens, err := s.maps.MapTokens(ctx, tx, *session.CurrentMapID)
	if err != nil {
		return nil, fmt.Errorf("read the tokens: %w", err)
	}
	for _, t := range tokens {
		if t.CharacterID == "" {
			continue
		}
		col, row := squareOf(g, t.XBP, t.YBP)
		out[t.CharacterID] = grid.Square{Col: int(col), Row: int(row)}
	}
	return out, nil
}

// between is the distance in feet between two characters, false when either has no square.
func between(pos map[string]grid.Square, a, b string) (int32, bool) {
	pa, okA := pos[a]
	pb, okB := pos[b]
	if !okA || !okB {
		return 0, false
	}
	return clamp32(grid.RangeFt(pa, pb), 0, math.MaxInt32), true
}

// reachLimit is how far a spell of this range reaches, and whether it has a limit at
// all: the caster alone for a spell that stays on the caster, 5 ft for touch, the
// distance for a ranged spell, and no limit for sight, unlimited and special ranges, which
// are the table's to judge.
func reachLimit(kind string, ft int32, casterOnly bool) (int32, bool) {
	switch kind {
	case rules.RangeSelf:
		return 0, casterOnly
	case rules.RangeTouch:
		return meleeReachFt, true
	case rules.RangeRanged:
		return ft, true
	}
	return 0, false
}

// metersPT writes feet as the app does, in meters at 1,5 m a square: "1,5 m", "9 m".
func metersPT(ft int32) string {
	m := float64(ft) * 0.3
	text := strconv.FormatFloat(m, 'f', 1, 64)
	text = strings.TrimSuffix(text, ".0")
	return strings.Replace(text, ".", ",", 1) + " m"
}

// outOfReachPT is the reason a target is too far, in Portuguese.
func outOfReachPT(kind string, limit int32) string {
	if kind == rules.RangeTouch {
		return "Fora do alcance do toque (" + metersPT(limit) + ")."
	}
	return "Fora do alcance da magia (" + metersPT(limit) + ")."
}

// wearsArmorPT is the reason Mage Armor refuses a target.
const wearsArmorPT = "Está de armadura: a Armadura Arcana não funciona."

// reachOfTargets says, for each target, whether the spell reaches it from the caster.
func (s *Service) reachOfTargets(ctx context.Context, campaignID string, spell *playv1.CastingSpell, casterID string, targets []*playv1.CastingTarget) ([]*playv1.CastingReach, error) {
	limit, limited := reachLimit(spell.GetRangeKind(), spell.GetRangeFt(), spell.GetCasterOnly())
	var out []*playv1.CastingReach
	for _, t := range targets {
		r := &playv1.CastingReach{CharacterId: t.GetCharacterId(), InRange: true}
		switch {
		case t.GetCharacterId() == casterID:
		case limited && spell.GetCasterOnly():
			r.InRange, r.ReasonPt = false, "Só quem conjura."
		case limited && t.GetDistanceKnown() && t.GetDistanceFt() > limit:
			r.InRange, r.ReasonPt = false, outOfReachPT(spell.GetRangeKind(), limit)
		}
		if r.InRange && spell.GetEffect() == playv1.CastingEffectKind_CASTING_EFFECT_KIND_ARMOR_CLASS && !t.GetNpc() {
			mage, err := s.roster.MageArmorAC(ctx, nil, campaignID, t.GetCharacterId())
			if err != nil {
				return nil, err
			}
			if mage.Applies && mage.Wears {
				r.InRange, r.ReasonPt = false, wearsArmorPT
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// checkReach refuses, for a player, a target beyond the spell's reach on the map. The
// master is never held to it (the master judges, RN-25 for a combat without a map).
func (s *Service) checkReach(ctx context.Context, c *combatTx, plan castPlan, master bool) error {
	if master {
		return nil
	}
	sp := plan.osp.Spell
	limit, limited := reachOf(sp)
	if !limited {
		return nil
	}
	pos, err := s.tokenSquares(ctx, c.tx, c.session)
	if err != nil {
		return err
	}
	for _, t := range plan.g.targets {
		if t.ID == plan.g.caster.ID {
			continue
		}
		if dist, ok := between(pos, plan.g.caster.ID, t.ID); ok && dist > limit {
			return errCasting(playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_TARGET_OUT_OF_REACH, "a target is beyond the spell's reach",
				func(b *playv1.CastingBlocked) { b.MissingFt = dist - limit })
		}
	}
	return nil
}

var _ = link.EffectHeal
