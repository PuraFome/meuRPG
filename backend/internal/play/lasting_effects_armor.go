package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"connectrpc.com/connect"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The armor class an effect gives (Mage Armor, SRD 5.1: "the target's base AC becomes 13 + its
// Dexterity modifier", for a willing creature that wears no armor; the spell ends if the target
// dons armor, and lasts 8 hours). It is an effect on the character like the others: a row of
// character_effects out of a combat, of combatant_states in it. The base armor class the effects
// give is worked out again, in the transaction of every change to them, to the character's vitals
// (what the session's panels read) and to its combatants (what the combat's rolls read).

// modifiersFor are the modifiers an effect of the catalog gives a character: the ones the file
// lists, with the armor class worked out for the target when the effect sets one. false means the
// effect does not take hold (Mage Armor on a creature that wears armor, or on a basic sheet).
func (s *Service) modifiersFor(ctx context.Context, c *combatTx, def *rules.EffectDef, characterID string) ([]rules.EffectModifier, bool, error) {
	mods := slices.Clone(def.Modifiers)
	for i, m := range mods {
		if m.Kind != rules.ModifierBaseAC {
			continue
		}
		mage, err := s.roster.MageArmorAC(ctx, c.tx, c.session.CampaignID, characterID)
		if err != nil {
			return nil, false, err
		}
		if !mage.Applies || mage.Wears || mage.AC < 1 {
			return nil, false, nil
		}
		mods[i].Value = int(clamp32(mage.AC, 1, 60))
	}
	return mods, true, nil
}

// armorBaseOf is the base armor class the effects on a character give, in a combat or out of
// it, 0 for none.
func (s *Service) armorBaseOf(ctx context.Context, q *playdb.Queries, characterID string) (int32, error) {
	var best int
	out, err := q.ListCharacterEffects(ctx, []string{characterID})
	if err != nil {
		return 0, fmt.Errorf("list the character's effects: %w", err)
	}
	for _, r := range out {
		best = max(best, combat.BaseAC(modifiersOf(r.Modifiers)))
	}
	in, err := q.ListCombatEffectsOfCharacter(ctx, characterID)
	if err != nil {
		return 0, fmt.Errorf("list the effects in the combats: %w", err)
	}
	for _, st := range in {
		best = max(best, combat.BaseAC(effectModifiers(st)))
	}
	return clamp32(best, 0, 60), nil
}

// syncArmorBase works the base armor class of the characters out again and writes it to their
// vitals; the streams are told after the commit. A character that is no active player's (an NPC)
// has none.
func (s *Service) syncArmorBase(ctx context.Context, c *combatTx, characterIDs ...string) error {
	slices.Sort(characterIDs)
	for _, id := range slices.Compact(characterIDs) {
		if id == "" {
			continue
		}
		base, err := s.armorBaseOf(ctx, c.q, id)
		if err != nil {
			return err
		}
		before, after, err := s.vitals.SetArmorBase(ctx, c.tx, c.session.CampaignID, id, base)
		if connect.CodeOf(err) == connect.CodeNotFound {
			continue // an NPC, a dead or a reserved character: there are no vitals to show
		}
		if err != nil {
			return err
		}
		if after != nil && after != before {
			c.told = append(c.told, after)
		}
	}
	return nil
}

// ArmorWorn implements characters.ReviewHost: a character that dons armor loses what needs none
// (Mage Armor, SRD 5.1: "the spell ends if the target dons armor"). The effects that set a base
// armor class go, in a combat or out of it, the cast that held them ends, and the streams are
// told after the commit.
func (s *Service) ArmorWorn(ctx context.Context, tx pgx.Tx, campaignID, characterID string) (func(context.Context), error) {
	q := s.queries.WithTx(tx)
	session, err := q.GetOpenGameSessionForUpdate(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		session = playdb.GameSession{CampaignID: campaignID}
	} else if err != nil {
		return nil, fmt.Errorf("lock the open session: %w", err)
	}
	c, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), svc: s, master: true})
	if err != nil {
		return nil, err
	}
	var casts []string
	out, err := q.ListCharacterEffects(ctx, []string{characterID})
	if err != nil {
		return nil, fmt.Errorf("list the character's effects: %w", err)
	}
	for _, r := range out {
		if combat.BaseAC(modifiersOf(r.Modifiers)) == 0 {
			continue
		}
		if err := q.DeleteCharacterEffect(ctx, r.ID); err != nil {
			return nil, fmt.Errorf("end an effect that needs no armor: %w", err)
		}
		casts = append(casts, r.GroupID)
	}
	in, err := q.ListCombatEffectsOfCharacter(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list the effects in the combats: %w", err)
	}
	for _, st := range in {
		if combat.BaseAC(effectModifiers(st)) == 0 {
			continue
		}
		if err := q.DeleteLastingEffect(ctx, st.ID); err != nil {
			return nil, fmt.Errorf("end an effect that needs no armor: %w", err)
		}
		casts = append(casts, deref(st.GroupID))
	}
	if len(casts) == 0 {
		return nil, nil
	}
	if err := q.ClearMageArmorACOfCharacter(ctx, characterID); err != nil {
		return nil, fmt.Errorf("take the base armor class off the combatants: %w", err)
	}
	slices.Sort(casts)
	for _, id := range slices.Compact(casts) {
		cast, err := q.GetSpellCast(ctx, playdb.GetSpellCastParams{ID: id, CampaignID: campaignID})
		if err == nil && cast.Status == castActive {
			if _, _, _, err := s.closeCast(ctx, c, cast, castEnded, endDismissed); err != nil {
				return nil, err
			}
		}
	}
	if err := s.syncArmorBase(ctx, c, characterID); err != nil {
		return nil, err
	}
	told := c.told
	return func(ctx context.Context) {
		s.publishVitalsOf(campaignID, told)
		s.publishCastsChanged(campaignID, false)
		if c.enc.ID != "" {
			s.publishEncounterChanged(ctx, campaignID, c.enc)
		}
	}, nil
}
