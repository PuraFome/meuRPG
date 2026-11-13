package play

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A Wild Shape form lasts half the druid's level in hours (SRD 5.1). The clock is a lasting
// effect (effect:wild-shape: owner's and master's only, no modifiers, no conditions) so it runs
// with the one clock the game already has: in a combat a combatant_states row counted in
// rounds, out of one a character_effects row counted in game time, which change home with the
// character, same id, as every effect does. When the row ends by its time, or by the master's
// hand, the form ends with it (formClockEnded); every other end of a form deletes the row.

// wildShapeEffect is the key of the effect that holds a form's clock.
const wildShapeEffect = "effect:wild-shape"

// dropFormClock takes a character's form clock away, in a combat and out of one, for a change
// that has no undo to keep it for (the form ended where only the vitals are at hand).
func dropFormClock(ctx context.Context, q *playdb.Queries, characterID string) error {
	_, err := dropFormClockRows(ctx, q, characterID)
	return err
}

func dropFormClockRows(ctx context.Context, q *playdb.Queries, characterID string) ([]playdb.CombatantState, error) {
	rows, err := q.DeleteFormClockOfCharacter(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("stop the form's clock: %w", err)
	}
	if err := q.DeleteFormClockOfCharacterOutside(ctx, characterID); err != nil {
		return nil, fmt.Errorf("stop the form's clock: %w", err)
	}
	return rows, nil
}

// takeFormClock is dropFormClock inside a combat's change: the row of this combat is kept in the
// change's event, so the undo puts the same row back (effectsBack, RN-22).
func takeFormClock(ctx context.Context, c *combatTx, characterID string) error {
	rows, err := dropFormClockRows(ctx, c.q, characterID)
	if err != nil {
		return err
	}
	for _, st := range rows {
		if st.EncounterID == c.enc.ID {
			c.endedEffects = append(c.endedEffects, st)
		}
	}
	return nil
}

// formClockLeft is the game time the form's clock has left, in seconds, and whether it has one.
// who is the character's combatant when it is in a combat (the clock is then a row of it).
func formClockLeft(ctx context.Context, c *combatTx, characterID string, who *playdb.Combatant) (left int32, found bool, err error) {
	if who != nil {
		rows, err := c.q.ListLastingEffects(ctx, c.enc.ID)
		if err != nil {
			return 0, false, fmt.Errorf("list the effects: %w", err)
		}
		for _, st := range rows {
			if st.CombatantID == who.ID && conv.OrZero(st.SourceKey) == wildShapeEffect && st.EndsRound != nil {
				return max(*st.EndsRound-max(c.enc.Round, 1), 0) * rules.SecondsPerRound, true, nil
			}
		}
		return 0, false, nil
	}
	rows, err := c.q.ListCharacterEffects(ctx, []string{characterID})
	if err != nil {
		return 0, false, fmt.Errorf("list the character's effects: %w", err)
	}
	for _, r := range rows {
		if r.SourceKey == wildShapeEffect && r.SecondsLeft != nil {
			return *r.SecondsLeft, true, nil
		}
	}
	return 0, false, nil
}

// resetFormClock sets the time the form's clock has left, in place (the row, its id and its
// group stay), and says whether there was a clock to set.
func (s *Service) resetFormClock(ctx context.Context, c *combatTx, characterID string, who *playdb.Combatant, seconds int32) (bool, error) {
	if who == nil {
		rows, err := c.q.ListCharacterEffects(ctx, []string{characterID})
		if err != nil {
			return false, fmt.Errorf("list the character's effects: %w", err)
		}
		for _, r := range rows {
			if r.SourceKey == wildShapeEffect {
				return true, c.q.SetCharacterEffectSeconds(ctx, playdb.SetCharacterEffectSecondsParams{ID: r.ID, SecondsLeft: &seconds})
			}
		}
		return false, nil
	}
	rows, err := c.q.ListLastingEffects(ctx, c.enc.ID)
	if err != nil {
		return false, fmt.Errorf("list the effects: %w", err)
	}
	for _, st := range rows {
		if st.CombatantID != who.ID || conv.OrZero(st.SourceKey) != wildShapeEffect {
			continue
		}
		_, err := c.q.SetLastingEffectEnds(ctx, playdb.SetLastingEffectEndsParams{
			ID: st.ID, DurationKind: st.DurationKind, EndsRound: new(max(c.enc.Round, 1) + max(seconds/rules.SecondsPerRound, 1)),
			EndsCombatantID: &who.ID, EndsPhase: new(phaseStartKey),
		})
		return true, err
	}
	return false, nil
}

// startFormClock gives a form its clock: seconds of game time (0 means the form's full
// duration, from the druid's level; a form with no druid level has no clock). It replaces the
// clock the form had.
func (s *Service) startFormClock(ctx context.Context, c *combatTx, characterID string, who *playdb.Combatant, seconds, full int32) error {
	if seconds <= 0 {
		seconds = full
	}
	if err := dropFormClock(ctx, c.q, characterID); err != nil {
		return err
	}
	if seconds <= 0 {
		return nil
	}
	if who == nil {
		_, err := c.q.InsertCharacterEffect(ctx, playdb.InsertCharacterEffectParams{
			CampaignID: c.session.CampaignID, CharacterID: characterID, GroupID: uuid.New().String(), SourceKey: wildShapeEffect, SourceKind: "feature",
			ConditionKeys: []string{}, Modifiers: []byte("[]"), DurationKind: rules.EffectDurationRounds, SecondsLeft: &seconds,
			PlayerVisible: true, Audience: audienceOwner, CreatedAt: c.now,
		})
		if err != nil {
			return fmt.Errorf("start the form's clock: %w", err)
		}
		return nil
	}
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return err
	}
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return err
	}
	def, _ := s.effectDef(content, wildShapeEffect)
	_, err = s.addEffects(ctx, c, cs, effectSpec{
		key: wildShapeEffect, sourceKind: "feature", def: def, caster: who, targets: []playdb.Combatant{*who},
		dur: durationSpec{Kind: rules.EffectDurationRounds, Rounds: max(seconds/rules.SecondsPerRound, 1), Anchor: who.ID},
	})
	return err
}

// formClockEnded ends the form of each druid whose clock row just ended in a combat: by its time
// (endDuration: reason expired) or by the master's hand (endByMaster). A row that goes for any
// other reason (the combat ended, its owner left) is moving, not ending, and leaves the form.
func (s *Service) formClockEnded(ctx context.Context, c *combatTx, cs []playdb.Combatant, rows []playdb.CombatantState, reason string) error {
	ended := endedExpired
	switch reason {
	case endDuration:
	case endByMaster:
		ended = endedMasterEnded
	default:
		return nil
	}
	for _, st := range rows {
		i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == st.CombatantID })
		if i < 0 || cs[i].Kind != kindPlayer {
			continue
		}
		if err := s.formClockRan(ctx, c, cs[i].CharacterID, &cs[i], ended); err != nil {
			return err
		}
	}
	return nil
}

// formClocksGone does the same for the clocks of characters out of a combat (reason "" is a
// row going for another reason: nothing to do).
func (s *Service) formClocksGone(ctx context.Context, c *combatTx, rows []playdb.CharacterEffect, reason string) error {
	if reason == "" {
		return nil
	}
	for _, r := range rows {
		if r.SourceKey != wildShapeEffect {
			continue
		}
		if err := s.formClockRan(ctx, c, r.CharacterID, nil, reason); err != nil {
			return err
		}
	}
	return nil
}

// formClockRan ends the form whose clock is gone: the character is itself again, the combatant
// (when there is one) has its own numbers back, and a wild_shape_ended event says why. The
// clock runs out at the turn change (EndTurn) or game time moves, neither of which an undo takes
// back, so there is nothing to keep for one.
func (s *Service) formClockRan(ctx context.Context, c *combatTx, characterID string, who *playdb.Combatant, reason string) error {
	now, err := s.vitals.GetVitalsTx(ctx, c.tx, c.session.CampaignID, characterID)
	if err != nil {
		return err
	}
	w := now.GetWildShape()
	if w == nil {
		return nil
	}
	after, err := s.formEnds(ctx, c.q, c.tx, c.session.ID, c.session.CampaignID, characterID, w.GetBeastKey(), reason, 0)
	if err != nil {
		return err
	}
	c.told = append(c.told, after)
	return nil
}

// restoreFormClock is what an undo does with the clock of the form it sets: no form, no clock;
// a form gets the time it had left (left, 0 when the event did not keep it: the whole duration).
func (s *Service) restoreFormClock(ctx context.Context, c *combatTx, characterID string, who *playdb.Combatant, inForm bool, left int32, body link.Character) error {
	if !inForm {
		return dropFormClock(ctx, c.q, characterID)
	}
	if _, found, err := formClockLeft(ctx, c, characterID, who); err != nil || found {
		return err // the undo of the change that ended it already put its row back
	}
	return s.startFormClock(ctx, c, characterID, who, left, body.WildShapeSeconds)
}

// formTimeText is the time a Wild Shape form has left as a card reads it: hours and minutes
// ("1 h 52 min restantes", "52 min restantes", never hundreds of rounds); under a minute, rounds
// ("Restam 4 rodadas").
func formTimeText(seconds int32) string {
	switch {
	case seconds >= 3600:
		h, m := seconds/3600, seconds%3600/60
		if m == 0 {
			return fmt.Sprintf("%d h restantes", h)
		}
		return fmt.Sprintf("%d h %d min restantes", h, m)
	case seconds >= 60:
		return fmt.Sprintf("%d min restantes", seconds/60)
	}
	return roundsLeftText(int(seconds / rules.SecondsPerRound))
}
