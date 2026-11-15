package play

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/platform/uid"
	"github.com/PuraFome/meuRPG/backend/internal/play/consumable"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The use of a consumable (ChargesService.UseConsumable): a potion, a scroll or another single-use item of the sheet is
// spent, and what it does is carried out or, for the consumables the app has no effect for, left to the table's text.

// eventConsumableUsed: a consumable was used up. Like the charge events it is not an undoable kind, so it closes the
// undo chain; its payload (consumableUsedEvent) holds ids, numbers and the catalog's names, never free text.
const eventConsumableUsed = "consumable_used"

// consumableUsedEvent is the payload of consumable_used (consumable.Used).
type consumableUsedEvent = consumable.Used

// consumableCall is what UseConsumable reads from its request: the call (the user's character and the key) and the receiver.
type consumableCall struct {
	chargeCall
	req    *playv1.UseConsumableRequest
	target string
}

// consumableRun is what the transaction leaves for the answer: the event (which holds the outcome), the text of a use with
// no effect and the receiver's vitals.
type consumableRun struct {
	ev       consumableUsedEvent
	text     []string
	vitals   []*playv1.CharacterVitals
	target   string
	repeated bool
}

// UseConsumable implements playv1connect.ChargesServiceHandler: the character's player or the master uses up a consumable of
// the sheet, out of a combat. The receiver is the character or, for a gift, another one.
func (s *Service) UseConsumable(ctx context.Context, req *connect.Request[playv1.UseConsumableRequest]) (*connect.Response[playv1.UseConsumableResponse], error) {
	call, err := readChargeCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetCharacterId(), req.Msg.GetIdempotencyKey(), req.Msg)
	if err != nil {
		return nil, err
	}
	if call.keyText == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key must be a UUID"))
	}
	use := consumableCall{chargeCall: call, req: req.Msg, target: call.charText}
	if t := req.Msg.GetTargetCharacterId(); t != "" {
		id, ok := uid.Canonical(t)
		if !ok {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("character not found"))
		}
		use.target = id
	}
	var run consumableRun
	err = s.txOpen(ctx, call.m.CampaignID, func(tx pgx.Tx, h *hints, session playdb.GameSession) error {
		run = consumableRun{} // a retry starts over
		return s.useConsumableTx(ctx, tx, h, session, use, &run)
	})
	if err != nil {
		return nil, s.dbError(ctx, "use a consumable", err)
	}
	if !run.repeated {
		logging.Event(ctx, s.logger, "consumable.used", slog.String("character_id", call.charText), slog.String("item", run.ev.Item), slog.String("outcome", run.ev.Outcome))
	}
	return connect.NewResponse(consumable.Response(run.ev, run.target, run.text, run.vitals)), nil
}

// useConsumableTx is the transaction of UseConsumable (the session is locked): retry, who may, the combat, the spent unit,
// the effect, the event and the hints.
func (s *Service) useConsumableTx(ctx context.Context, tx pgx.Tx, h *hints, session playdb.GameSession, use consumableCall, run *consumableRun) error {
	q := s.queries.WithTx(tx)
	done, again, err := chargeEventByKey(ctx, q, session.ID, use.chargeCall, eventConsumableUsed)
	if err != nil {
		return err
	}
	if again {
		return s.consumableRetry(ctx, tx, h, done, use, run)
	}
	if err := s.canUseConsumable(ctx, tx, q, session.ID, use); err != nil {
		return err
	}
	item, err := s.vitals.RemoveConsumable(ctx, tx, use.m.CampaignID, use.charText, use.req.GetEntryId())
	if err != nil {
		return err
	}
	run.target = use.target
	run.ev = consumableUsedEvent{Entry: item.GetId(), Item: item.GetKey(), Name: item.GetNamePt()}
	if use.target != use.charText {
		run.ev.Target = use.target
	}
	if err := s.applyConsumable(ctx, tx, q, session, use, run); err != nil {
		return err
	}
	if err := s.appendKeyedEvent(ctx, q, keyedEvent{
		sessionID: session.ID, kind: eventConsumableUsed, userID: use.m.UserID, characterID: &use.charText, key: use.keyText, hash: use.hash,
	}, run.ev); err != nil {
		return err
	}
	if run.vitals, err = s.receiverVitals(ctx, tx, use); err != nil {
		return err
	}
	s.tellVitalsAfter(h, use.m.CampaignID, slices.Compact([]string{use.charText, use.target})...)
	h.touch()
	return nil
}

// canUseConsumable checks who may use it and on whom: the character's player or the master, a user and a receiver that are
// alive. A user or receiver that fights in the running combat is refused: a use in a combat is the combat's own (the seam of
// the combat's rules: it costs the action and a turn), and an effect of a combatant lives in the combat.
func (s *Service) canUseConsumable(ctx context.Context, tx pgx.Tx, q *playdb.Queries, sessionID string, use consumableCall) error {
	if _, err := s.requireItemCharacter(ctx, tx, use.m, use.charText); err != nil {
		return err
	}
	if use.target != use.charText {
		_, dead, _, err := s.vitals.ItemAccess(ctx, tx, use.m.CampaignID, use.target)
		if err != nil {
			return err
		}
		if dead {
			return characterBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CHARACTER_DEAD, use.target, "the receiver is dead")
		}
	}
	enc, cs, found, err := openCombatants(ctx, q, sessionID)
	if err != nil || !found || enc.Status != statusActive {
		return err
	}
	if wearerIn(cs, use.charText) != nil || wearerIn(cs, use.target) != nil {
		return chargesBlocked(playv1.ChargesBlockedReason_CHARGES_BLOCKED_REASON_COMBAT_RUNNING, "a character of the running combat uses its consumables in the combat", 0)
	}
	return nil
}

// applyConsumable carries out what the item does to the receiver, by the content's potion table, and fills the event.
func (s *Service) applyConsumable(ctx context.Context, tx pgx.Tx, q *playdb.Queries, session playdb.GameSession, use consumableCall, run *consumableRun) error {
	content, err := s.roster.RulesContent(ctx, tx, use.m.CampaignID)
	if err != nil {
		return err
	}
	potion, ok := content.PotionEffect(run.ev.Item)
	switch {
	case ok && potion.Kind == rules.PotionHeal:
		return s.healWithPotion(ctx, tx, q, session, use, potion, &run.ev)
	case ok && potion.Kind == rules.PotionResistance:
		run.ev.Outcome, run.ev.DamageType = consumable.OutcomeResistance, potion.DamageType
		return s.putPotionResistance(ctx, tx, q, session, use, potion, run.ev.Item)
	}
	run.ev.Outcome = consumable.OutcomeTextOnly
	item, _ := content.MagicItem(run.ev.Item)
	run.text = consumable.Text(item)
	return nil
}

// healWithPotion rolls the potion's dice (the app's, or the faces typed from real dice by the table's rule, RN-18) and heals the
// receiver up to the maximum.
func (s *Service) healWithPotion(ctx context.Context, tx pgx.Tx, q *playdb.Queries, session playdb.GameSession, use consumableCall, potion rules.PotionEffect, ev *consumableUsedEvent) error {
	if err := s.mustRollThisWay(ctx, tx, use.m, rollInput{inApp: use.req.GetRollInApp()}); err != nil {
		return err
	}
	faces, total, err := consumable.Roll(s.roller, potion.Heal, use.req.GetRollInApp(), use.req.GetTypedFaces())
	if errors.Is(err, consumable.ErrFaces) {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err != nil {
		return err
	}
	before, err := s.vitals.GetVitalsTx(ctx, tx, use.m.CampaignID, use.target)
	if err != nil {
		return err
	}
	healed := consumable.Healed(before.GetHitPointsCurrent(), before.GetHitPointsMax(), total)
	if healed > 0 {
		now := before.GetHitPointsCurrent() + healed
		req := &playv1.AdjustCharacterVitalsRequest{CampaignId: use.m.CampaignID, CharacterId: use.target, HitPointsCurrent: &now}
		if _, _, err := s.changeVitals(ctx, q, tx, session.ID, use.m.CampaignID, use.target, req); err != nil {
			return err
		}
	}
	ev.Outcome, ev.Dice, ev.Faces, ev.Total, ev.Healed, ev.Physical = consumable.OutcomeHealed, potion.Heal.Text(), faces, total, healed, !use.req.GetRollInApp()
	return nil
}

// putPotionResistance gives the receiver the potion's resistance as an effect of the character outside a combat: one hour of
// game time, the owner's and the master's to read. A second potion of the same kind starts the hour again.
func (s *Service) putPotionResistance(ctx context.Context, tx pgx.Tx, q *playdb.Queries, session playdb.GameSession, use consumableCall, potion rules.PotionEffect, key string) error {
	if err := s.requireOutsideCombat(ctx, tx, q, use.m.CampaignID, session.ID, []string{use.target}); err != nil {
		return err
	}
	group := uuid.New().String()
	if _, err := q.DeleteCharacterEffectsOfSpell(ctx, playdb.DeleteCharacterEffectsOfSpellParams{CharacterID: use.target, SourceKey: key, GroupID: group}); err != nil {
		return fmt.Errorf("replace the same potion's resistance: %w", err)
	}
	body, err := consumable.ResistanceJSON(potion)
	if err != nil {
		return err
	}
	seconds := consumable.Seconds(potion)
	if _, err := q.InsertCharacterEffect(ctx, playdb.InsertCharacterEffectParams{
		CampaignID: use.m.CampaignID, CharacterID: use.target, GroupID: group, SourceKey: key, SourceKind: "master",
		ConditionKeys: []string{}, Modifiers: body, DurationKind: rules.EffectDurationRounds, SecondsLeft: &seconds,
		PlayerVisible: true, Audience: audienceOwner, CreatedAt: s.now(),
	}); err != nil {
		return fmt.Errorf("put the resistance on the character: %w", err)
	}
	return nil
}

// receiverVitals are the receiver's vitals as they stand, none for a character that is no player's.
func (s *Service) receiverVitals(ctx context.Context, tx pgx.Tx, use consumableCall) ([]*playv1.CharacterVitals, error) {
	v, err := s.vitals.GetVitalsTx(ctx, tx, use.m.CampaignID, use.target)
	if connect.CodeOf(err) == connect.CodeNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []*playv1.CharacterVitals{v}, nil
}

// consumableRetry answers the retry of a key found stored: what the first call answered, with the receiver's vitals as they
// stand, and the hints the first call may have died before telling.
func (s *Service) consumableRetry(ctx context.Context, tx pgx.Tx, h *hints, done playdb.GetSessionEventByIdempotencyKeyRow, use consumableCall, run *consumableRun) error {
	if err := replayCharacterEvent(done, use.charText, "consumable", &run.ev); err != nil {
		return err
	}
	if use.target != cmp.Or(run.ev.Target, use.charText) {
		return errKeyReused()
	}
	content, err := s.roster.RulesContent(ctx, tx, use.m.CampaignID)
	if err != nil {
		return err
	}
	if item, ok := content.MagicItem(run.ev.Item); ok && run.ev.Outcome == consumable.OutcomeTextOnly {
		run.text = consumable.Text(item)
	}
	if run.vitals, err = s.receiverVitals(ctx, tx, use); err != nil {
		return err
	}
	run.target, run.repeated = use.target, true
	h.replay()
	s.tellVitalsAfter(h, use.m.CampaignID, slices.Compact([]string{use.charText, use.target})...)
	return nil
}
