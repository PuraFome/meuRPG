package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The gate of a turn: whether a combatant may act now, and the refusals the writes answer with when it may not.

// gate says why the combatant cannot act now, or UNSPECIFIED when it can: the
// combat is not running, the combatant is out of the fight or not on turn, or
// a player's character is down. GetTurnOptions disables every option with it,
// and the writes refuse with the matching EncounterBlocked.
func (s *Service) gate(ctx context.Context, tx pgx.Tx, campaignID string, e playdb.Encounter, c playdb.Combatant) (rulesv1.DisabledReasonCode, error) {
	switch {
	case e.Status != statusActive:
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBAT_NOT_ACTIVE, nil
	case c.Defeated:
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DEFEATED, nil
	case !actsNow(e, c): // its group is not on turn, or its part of a joint turn ended
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NOT_YOUR_TURN, nil
	}
	down, err := s.isDown(ctx, tx, campaignID, c)
	if err != nil {
		return 0, err
	}
	if down {
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DOWN, nil
	}
	// An effect or a condition that takes the actions away (RN-22).
	if code := cannotActCode(c); code != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED {
		return code, nil
	}
	// A surprised combatant does not move, act or react until its first turn ends.
	surprised, err := s.surprisedNow(ctx, tx, e, c)
	if err != nil {
		return 0, err
	}
	if surprised {
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_SURPRISED, nil
	}
	return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED, nil
}

// isDown says whether a player's character is at 0 hit points ("Caído"): its
// vitals say so. A character that left the sheet's active state (it died, and the
// combat was not told) is out of the fight too: down, never an error. An NPC and a
// creature are never down: they are defeated.
// A write passes its transaction, so it reads what it will overwrite; a read
// passes nil.
func (s *Service) isDown(ctx context.Context, tx pgx.Tx, campaignID string, c playdb.Combatant) (bool, error) {
	if c.Kind != kindPlayer {
		return false, nil
	}
	var v *playv1.CharacterVitals
	var err error
	if tx != nil {
		v, err = s.vitals.GetVitalsTx(ctx, tx, campaignID, c.CharacterID)
	} else {
		v, err = s.vitals.GetVitals(ctx, campaignID, c.CharacterID)
	}
	if connect.CodeOf(err) == connect.CodeNotFound {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return v.GetHitPointsMax() > 0 && v.GetHitPointsCurrent() == 0, nil
}

// gateError is the failed_precondition a write answers with for a reason of
// gate.
func gateError(code rulesv1.DisabledReasonCode) error {
	switch code {
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBAT_NOT_ACTIVE:
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ACTIVE, "the combat is not running")
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DOWN:
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_COMBATANT_DOWN, "the character is down")
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_SURPRISED:
		return errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED, "the combatant is surprised")
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_INCAPACITATED, rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_EFFECT_LETHARGY:
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CANNOT_ACT, "the combatant cannot act now")
	}
	// Not on turn, or out of the fight (a defeated combatant has no turn).
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN, "it is not this combatant's turn")
}

// mustNotBeDown refuses a player's character at 0 hit points: a down character
// does not move (RN-03, RN-21). tx is the open transaction, or nil for a read.
func (s *Service) mustNotBeDown(ctx context.Context, tx pgx.Tx, campaignID string, who playdb.Combatant) error {
	down, err := s.isDown(ctx, tx, campaignID, who)
	if err != nil {
		return err
	}
	if down {
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DOWN)
	}
	return nil
}

// mustActNow checks, inside a write, that the combatant is the one that may
// act now (on turn, in a running combat, not down).
func (s *Service) mustActNow(ctx context.Context, c *combatTx, who playdb.Combatant) error {
	if err := notEnded(c.enc); err != nil {
		return err
	}
	code, err := s.gate(ctx, c.tx, c.session.CampaignID, c.enc, who)
	if err != nil {
		return err
	}
	if code != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED {
		return gateError(code)
	}
	if err := s.mustNotHold(ctx, c); err != nil { // a question waits for the master: nobody's turn goes on
		return err
	}
	return s.mustNotWait(ctx, c, who)
}

func turnOf(c playdb.Combatant) link.Turn {
	return link.Turn{
		ActionUsed: c.ActionUsed, BonusActionUsed: c.BonusActionUsed, ReactionUsed: c.ReactionUsed, AttacksMade: int(c.AttacksMade), AttackKey: conv.OrZero(c.ActionAttackKey), FlurryLeft: int(c.BonusAttacksLeft), ActionSurged: c.ActionSurged, SpellCast: c.SpellCast, BonusSpellCast: c.BonusSpellCast,
		Dashed: c.Dashed, SpeedFt: int(max(c.SpeedFt, c.SpeedFlyFt)), MovementUsedFt: int(c.MovementUsedDft) / 10,
		MovementUsedDFt: int(c.MovementUsedDft), LastMoveDFt: int(c.LastMoveDft), Disengaged: c.Disengaged,
		JumpLongDFt: int(c.JumpLongDft), JumpHighDFt: int(c.JumpHighDft),
		JumpLongBonusDFt: int(c.JumpLongBonusDft), JumpDoubled: c.JumpDoubled,
	}
}

// mustReactNow checks, inside a write, that the combatant may spend its
// reaction now: the combat is running, it is not out of the fight or down, and
// it is not its own turn (then an attack spends the action).
func (s *Service) mustReactNow(ctx context.Context, c *combatTx, who playdb.Combatant, onOffer bool) error {
	if err := notEnded(c.enc); err != nil {
		return err
	}
	switch {
	case c.enc.Status != statusActive:
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBAT_NOT_ACTIVE)
	case who.Defeated:
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DEFEATED)
	case actsNow(c.enc, who) && !onOffer: // an opportunity attack may come in a joint turn, on any turn (SRD)
		return connect.NewError(connect.CodeInvalidArgument, errors.New("on its own turn an attack spends the action, not the reaction"))
	}
	down, err := s.isDown(ctx, c.tx, c.session.CampaignID, who)
	if err != nil {
		return err
	}
	if down {
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DOWN)
	}
	return s.mustNotBeSurprised(ctx, c, who)
}

// mustReactNowOrOnTurn checks that a combatant may take a reaction now: the
// combat is running and it is not out of the fight or down; whether it is its
// own turn does not matter.
func (s *Service) mustReactNowOrOnTurn(ctx context.Context, c *combatTx, who playdb.Combatant) error {
	if err := notEnded(c.enc); err != nil {
		return err
	}
	switch {
	case c.enc.Status != statusActive:
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBAT_NOT_ACTIVE)
	case who.Defeated:
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DEFEATED)
	}
	down, err := s.isDown(ctx, c.tx, c.session.CampaignID, who)
	if err != nil {
		return err
	}
	if down {
		return gateError(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_COMBATANT_DOWN)
	}
	return s.mustNotBeSurprised(ctx, c, who)
}
