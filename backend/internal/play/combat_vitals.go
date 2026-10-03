package play

import (
	"context"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// How a combat changes a player's character's vitals (RN-02): the hit points
// (damage, healing), the spell slots and the class resources go through the
// same VitalsKeeper the master's correction uses, inside the change's own
// transaction, so there is one source of truth for them. Everything here runs
// after the session's row is locked.

// changeVitals applies a change to a character's vitals and, when it takes the
// character from 0 hit points to above 0, resets the death save counts of its
// combatant in the session's combat (RN-03: any healing above 0 does it: a
// spell, Retomar o fôlego, a natural 20, the master's correction, an undo). A
// drop from above 0 to 0 on the character's own turn holds its death save to
// its next turn. It returns the vitals before and after.
func (s *Service) changeVitals(ctx context.Context, q *playdb.Queries, tx pgx.Tx, sessionID, campaignID, characterID string, req *playv1.AdjustCharacterVitalsRequest) (before, after *playv1.CharacterVitals, err error) {
	before, after, err = s.vitals.AdjustVitals(ctx, tx, campaignID, characterID, req)
	if err != nil {
		return nil, nil, err
	}
	if before.GetHitPointsCurrent() > 0 && after.GetHitPointsCurrent() == 0 {
		if err := q.MarkDeathSaveRolledOnTurn(ctx, playdb.MarkDeathSaveRolledOnTurnParams{CharacterID: characterID, GameSessionID: sessionID}); err != nil {
			return nil, nil, fmt.Errorf("hold the death save to the next turn: %w", err)
		}
	}
	if before.GetHitPointsCurrent() == 0 && after.GetHitPointsCurrent() > 0 {
		if err := q.ResetDeathSavesOfCharacter(ctx, playdb.ResetDeathSavesOfCharacterParams{CharacterID: characterID, GameSessionID: sessionID}); err != nil {
			return nil, nil, fmt.Errorf("reset the death saves: %w", err)
		}
	}
	return before, after, nil
}

// vitalsOf changes a character's vitals inside the combat's transaction.
func (s *Service) vitalsOf(ctx context.Context, c *combatTx, characterID string, req *playv1.AdjustCharacterVitalsRequest) (before, after *playv1.CharacterVitals, err error) {
	return s.changeVitals(ctx, c.q, c.tx, c.session.ID, c.session.CampaignID, characterID, req)
}

// spendSlot spends one spell slot of a player's character (delta 1) or gives it
// back (delta -1: an undo), and returns its vitals after. NO_SLOT when none is
// free.
func (s *Service) spendSlot(ctx context.Context, c *combatTx, characterID string, slot slotRef, delta int32) (*playv1.CharacterVitals, error) {
	now, err := s.vitals.GetVitalsTx(ctx, c.tx, c.session.CampaignID, characterID)
	if err != nil {
		return nil, err
	}
	noSlot := func() error {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_SLOT, "there is no free spell slot of that level",
			func(b *playv1.EncounterBlocked) { b.MinLevel = slot.Level })
	}
	req := &playv1.AdjustCharacterVitalsRequest{}
	if slot.Pact {
		p := now.GetPactSlots()
		if p == nil || p.GetSlotLevel() != slot.Level {
			return nil, noSlot()
		}
		used := p.GetUsed() + delta
		if used > p.GetTotal() {
			return nil, noSlot()
		}
		used = max(used, 0)
		req.PactSlotsUsed = &used
	} else {
		i := slices.IndexFunc(now.GetSpellSlots(), func(u *playv1.SpellSlotUsage) bool { return u.GetLevel() == slot.Level })
		if i < 0 {
			return nil, noSlot()
		}
		u := now.GetSpellSlots()[i]
		used := u.GetUsed() + delta
		if used > u.GetTotal() {
			return nil, noSlot()
		}
		req.SpellSlotsUsed = []*playv1.SpellSlotsUsed{{Level: slot.Level, Used: max(used, 0)}}
	}
	_, after, err := s.vitalsOf(ctx, c, characterID, req)
	return after, err
}

// spendResource spends one use of a class or race resource of a player's
// character (delta 1) or gives it back (delta -1), and returns its vitals
// after. NO_USES when none is left.
func (s *Service) spendResource(ctx context.Context, c *combatTx, characterID, key string, delta int32) (*playv1.CharacterVitals, error) {
	now, err := s.vitals.GetVitalsTx(ctx, c.tx, c.session.CampaignID, characterID)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(now.GetResources(), func(r *playv1.ResourceUsage) bool { return r.GetKey() == key })
	if i < 0 {
		return nil, fmt.Errorf("character %s has no resource %q", characterID, key) // the options said it had
	}
	r := now.GetResources()[i]
	used := r.GetUsed() + delta
	if used > r.GetTotal() {
		return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES, "no uses left",
			func(b *playv1.EncounterBlocked) { b.Recharge = r.GetRecharge() })
	}
	_, after, err := s.vitalsOf(ctx, c, characterID, &playv1.AdjustCharacterVitalsRequest{
		ResourcesUsed: []*playv1.ResourceUsed{{Key: key, Used: max(used, 0)}},
	})
	return after, err
}
