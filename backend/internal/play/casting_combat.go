package play

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Where the casts outside a combat meet a combat, and a rest (SRD 5.1, "Spellcasting",
// "Resting").
//
//   - A casting that takes time and is still going when its caster joins a combat goes
//     on: the caster spends the action of each turn on it (SRD 5.1, "Longer Casting
//     Times"), and the table decides when it completes.
//   - The concentration the caster holds, a casting's too, goes into the combat: the
//     combatant concentrates on the spell, and gives it back when the combat ends (or the
//     spell ends there).
//   - Mage Armor on the character goes to its combatant, which the combat reads.
//   - A rest ends the spells whose whole duration it covers.

// What changed in the casts, for the stream after the commit: nothing, only the
// master's, or everyone's (a change to a cast the players see).
const (
	castsUnchanged = iota
	castsMasterOnly
	castsPublic
)

// castsTold notes that the casts changed, for write to tell the streams once the change
// is committed.
func (c *combatTx) castsTold(secret bool) {
	level := castsPublic
	if secret {
		level = castsMasterOnly
	}
	c.castsChanged = max(c.castsChanged, level)
}

// carryCasts runs when a character joins a combat (addParticipants): it fails the
// casting that was going, carries the concentration into the combatant and puts Mage
// Armor on it. Nothing happens for a character with no cast.
func (s *Service) carryCasts(ctx context.Context, c *combatTx, cb playdb.Combatant) error {
	live, err := c.q.ListLiveSpellCastsOfCaster(ctx, cb.CharacterID)
	if err != nil {
		return fmt.Errorf("list the caster's casts: %w", err)
	}
	for _, r := range live {
		// A cast that takes time goes on in the combat: the caster spends the action of each
		// of its turns on it and keeps concentrating (SRD 5.1, "Longer Casting Times"). Both a
		// casting and a concentration spell are carried by the combatant, which gives them
		// back when the combat ends.
		if r.Concentrating && r.CarriedEncounterID == nil {
			spell := r.SpellKey
			if err := c.q.SetCombatantConcentration(ctx, playdb.SetCombatantConcentrationParams{ID: cb.ID, ConcentrationSpell: &spell}); err != nil {
				return fmt.Errorf("carry the concentration into the combat: %w", err)
			}
			if err := c.q.SetSpellCastConcentration(ctx, playdb.SetSpellCastConcentrationParams{ID: r.ID, Concentrating: true, CarriedEncounterID: &c.enc.ID}); err != nil {
				return fmt.Errorf("mark the concentration as carried: %w", err)
			}
			c.castsTold(r.Secret)
		}
	}
	return s.carryMageArmor(ctx, c, cb)
}

// carryMageArmor puts the armor class of the best Mage Armor on the character on its
// combatant.
func (s *Service) carryMageArmor(ctx context.Context, c *combatTx, cb playdb.Combatant) error {
	probe, err := json.Marshal([]map[string]string{{"id": cb.CharacterID, "effect": castArmor}})
	if err != nil {
		return fmt.Errorf("encode the target: %w", err)
	}
	rows, err := c.q.ListLiveSpellCastsOnTarget(ctx, playdb.ListLiveSpellCastsOnTargetParams{CampaignID: c.session.CampaignID, Target: probe})
	if err != nil {
		return fmt.Errorf("list the casts on a target: %w", err)
	}
	var best int32
	for _, r := range rows {
		if r.SpellKey == rules.MageArmorSpell {
			best = max(best, armorOf(r, cb.CharacterID))
		}
	}
	if best == 0 {
		return nil
	}
	if err := c.q.SetCombatantMageArmorAC(ctx, playdb.SetCombatantMageArmorACParams{ID: cb.ID, MageArmorAc: &best}); err != nil {
		return fmt.Errorf("put Mage Armor on the combatant: %w", err)
	}
	return nil
}

// endCarriedCasts runs when a combat ends (endEncounter): the concentration its
// combatants held goes back to the casts. A spell whose concentration ended in the
// fight (broken, or replaced by another) ends with it; a casting whose concentration
// broke fails, and no slot was spent (SRD 5.1, "Longer Casting Times").
func (s *Service) endCarriedCasts(ctx context.Context, c *combatTx, cs []playdb.Combatant) error {
	carried, err := c.q.ListCarriedSpellCasts(ctx, &c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the carried casts: %w", err)
	}
	for _, r := range carried {
		i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.CharacterID == r.CasterID && !isCreature(o) })
		if i >= 0 && !holdsConcentration(cs[i], r.SpellKey) {
			status, reason := castEnded, endConcentration
			if r.Status == castCasting {
				status, reason = castFailed, endInterrupted
			}
			if _, _, _, err := s.closeCast(ctx, c, r, status, reason); err != nil {
				return err
			}
		} else if err := c.q.SetSpellCastConcentration(ctx, playdb.SetSpellCastConcentrationParams{ID: r.ID, Concentrating: true}); err != nil {
			return fmt.Errorf("give the concentration back: %w", err)
		}
		c.castsTold(r.Secret)
	}
	return nil
}

// holdsConcentration says the combatant concentrates on the spell.
func holdsConcentration(cb playdb.Combatant, spellKey string) bool {
	return cb.ConcentrationSpell != nil && *cb.ConcentrationSpell == spellKey
}

// endSpellsAtRest ends the spells a rest of restMinutes covers (a short rest is at least
// 60, a long rest at least 480): the ones that last, whose whole duration is no longer
// than the rest (SRD 5.1, "Resting"). The campaign's rest calls it, inside its own
// transaction. It returns the casts it ended and the vitals it changed.
func (s *Service) endSpellsAtRest(ctx context.Context, c *combatTx, restMinutes int) (ended []string, vitals []*playv1.CharacterVitals, err error) {
	live, err := c.q.ListLiveSpellCasts(ctx, c.session.CampaignID)
	if err != nil {
		return nil, nil, fmt.Errorf("list the casts: %w", err)
	}
	for _, r := range live {
		if r.Status != castActive || r.DurationSeconds == nil || int(*r.DurationSeconds) > restMinutes*secondsPerMinute {
			continue
		}
		_, v, _, err := s.closeCast(ctx, c, r, castEnded, endRest)
		if err != nil {
			return nil, nil, err
		}
		ended, vitals = append(ended, r.ID), append(vitals, v...)
		c.castsTold(r.Secret)
	}
	return ended, vitals, nil
}

const secondsPerMinute = 60

// armorWithSpells is the armor class a character has with the spells that last on it
// (Mage Armor, SRD 5.1): the better of the sheet's and the spell's. A combat reads it
// from the combatant instead; this is for what hits a character outside one (a trap).
func (s *Service) armorWithSpells(ctx context.Context, q *playdb.Queries, campaignID, characterID string, sheetAC int) (int, error) {
	probe, err := json.Marshal([]map[string]string{{"id": characterID, "effect": castArmor}})
	if err != nil {
		return sheetAC, fmt.Errorf("encode the target: %w", err)
	}
	rows, err := q.ListLiveSpellCastsOnTarget(ctx, playdb.ListLiveSpellCastsOnTargetParams{CampaignID: campaignID, Target: probe})
	if err != nil {
		return sheetAC, fmt.Errorf("list the casts on a target: %w", err)
	}
	for _, r := range rows {
		if r.SpellKey == rules.MageArmorSpell {
			sheetAC = max(sheetAC, int(armorOf(r, characterID)))
		}
	}
	return sheetAC, nil
}

// concentrationCheck is what hit points taken ask of a spell cast outside a combat that
// the character concentrates on: the cast and the Constitution saving throw DC to keep it
// (0: no save, the concentration is over).
type concentrationCheck struct {
	castID string
	dc     int32
	secret bool
}

// concentrationAfter looks at a correction of a character's hit points made outside the
// combat's own rolls: damage asks for a Constitution saving throw, DC 10 or half the
// damage, whichever is higher, and 0 hit points end the concentration (the character is
// incapacitated, SRD 5.1, "Duration"). The app only reminds: the table rolls, and the master
// ends the spell when the save fails (EndActiveSpell). A character in a combat has the
// combat's own concentration, which the combat asks for.
func (s *Service) concentrationAfter(ctx context.Context, tx pgx.Tx, q *playdb.Queries, session playdb.GameSession, m authz.Membership, before, after *playv1.CharacterVitals) (concentrationCheck, bool, error) {
	var out concentrationCheck
	taken := before.GetHitPointsCurrent() - after.GetHitPointsCurrent()
	if taken <= 0 {
		return out, false, nil
	}
	live, err := q.ListLiveSpellCastsOfCaster(ctx, after.GetCharacterId())
	if err != nil {
		return out, false, fmt.Errorf("list the caster's casts: %w", err)
	}
	for _, r := range live {
		if !r.Concentrating || r.CarriedEncounterID != nil {
			continue
		}
		out.castID, out.secret = r.ID, r.Secret
		if after.GetHitPointsCurrent() > 0 {
			out.dc = clamp32(combat.ConcentrationDC(int(taken)), 0, math.MaxInt32)
			return out, false, nil
		}
		c, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now(), actorUserID: m.UserID, svc: s, master: true})
		if err != nil {
			return out, false, err
		}
		status, reason := castEnded, endConcentration
		if r.Status == castCasting {
			status, reason = castFailed, endInterrupted
		}
		if _, _, _, err := s.closeCast(ctx, c, r, status, reason); err != nil {
			return out, false, err
		}
		return out, true, nil
	}
	return out, false, nil
}

// roundsOfSeconds is the game time in rounds an effect enters a combat with: a partial round
// does not count as a full one, so 59 seconds are 9 rounds.
func roundsOfSeconds(seconds int32) int32 { return seconds / rules.SecondsPerRound }
