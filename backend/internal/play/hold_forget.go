package play

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The one way to drop a held roll. A spend of the Inspiration deletes the character's row and the hold remembers it
// (inspiration_holds.favor_spent); a hold dropped before it is answered is a roll that never happens, with no event to undo, so the
// Inspiration is lost unless it is given back here. Every delete of inspiration_holds goes through this file, before the row is gone:
// the generated Delete* queries are called nowhere else, and no path may rely on the foreign key's cascade (combatants and
// encounters delete their holds by it), so a path that deletes a combatant calls forgetHoldsOf first (archtest).

// favorGiveBackIf gives the Inspiration back when it was spent on a held roll that is dropped before it was answered: no roll event
// exists for that spend, so nothing else would put it back. A hold that was answered, or that spent nothing, gives nothing. The refund
// prompt that roll opened (rollKey) closes as kept: the Inspiration is back, so there is nothing left to give.
func favorGiveBackIf(ctx context.Context, q *playdb.Queries, unsettled bool, campaignID, characterID, rollKey string) error {
	if !unsettled || characterID == "" {
		return nil
	}
	if err := favorGiveBack(ctx, q, campaignID, characterID); err != nil {
		return err
	}
	return keepRefundOfRoll(ctx, q, campaignID, rollKey)
}

// keepRefundOfRoll closes as kept the refund prompt of a roll that is gone (its undo, or its held roll forgotten): the Inspiration went back
// with the roll, so the master's question has no spend left to refund. No event is written for it: the roll's own undo is the record, and the
// transaction that drops the roll moves the session's number.
func keepRefundOfRoll(ctx context.Context, q *playdb.Queries, campaignID, rollKey string) error {
	key := rollKeyOf(rollKey)
	if key == "" {
		return nil
	}
	if _, err := q.CloseFavorRefundPromptsOfRoll(ctx, playdb.CloseFavorRefundPromptsOfRollParams{CampaignID: campaignID, RollKey: key}); err != nil {
		return fmt.Errorf("keep the refund question of the roll: %w", err)
	}
	return nil
}

// rollKeyOf is the key the refund prompt keeps for a roll: the request's, without the suffix of a reaction window's replay.
func rollKeyOf(key string) string { return strings.TrimSuffix(key, windowHoldSuffix) }

// refundSpentFavor is the master's refund of an Inspiration a roll spent for nothing (favor_refund.go): the one other way the row is given
// back. The roller must have none now: one that has it (the spend was given back already by the roll's undo, or the master granted another)
// answers ALREADY_HAS, so a spend is never refunded twice and the one-row rule is never the silent reason.
func refundSpentFavor(ctx context.Context, q *playdb.Queries, campaignID, characterID string) error {
	if _, err := q.InsertCharacterFavor(ctx, playdb.InsertCharacterFavorParams{CharacterID: characterID, CampaignID: campaignID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ALREADY_HAS, "the character has the inspiration: there is nothing to give back")
		}
		return fmt.Errorf("give the character's inspiration back: %w", err)
	}
	return nil
}

// forgetHoldsOf drops every held roll of an owner (a combatant, or a character outside a combat), giving back the Inspiration the
// unanswered ones spent. The owner's query also drops the character's holds outside any combat (combatant_id IS NULL) when its combatant
// leaves or the combat ends: those are refunded the same way.
func forgetHoldsOf(ctx context.Context, q *playdb.Queries, owner holdOwner) error {
	rows, err := q.DeleteInspirationHoldsOf(ctx, playdb.DeleteInspirationHoldsOfParams(owner))
	if err != nil {
		return fmt.Errorf("delete the held rolls: %w", err)
	}
	for _, r := range rows {
		if err := favorGiveBackIf(ctx, q, r.UnsettledFavor, r.CampaignID, r.CharacterID, r.IdempotencyKey); err != nil {
			return err
		}
	}
	return nil
}

// deleteCombatant is the one function that deletes a combatant's row (archtest keeps the generated DeleteCombatant here). The foreign key
// would delete the combatant's held rolls and the Inspiration they spent with them, so they are forgotten first, with their refund.
func deleteCombatant(ctx context.Context, q *playdb.Queries, who playdb.Combatant) error {
	if err := forgetHoldsOf(ctx, q, holdOwnerOfCombatant(who)); err != nil {
		return fmt.Errorf("forget the held rolls: %w", err)
	}
	if err := q.DeleteCombatant(ctx, who.ID); err != nil {
		return fmt.Errorf("delete the combatant: %w", err)
	}
	return nil
}

// forgetHold drops one held roll (the gate's: a question gone stale), giving back the Inspiration it spent when it was not answered.
func forgetHold(ctx context.Context, q *playdb.Queries, id string) error {
	rows, err := q.DeleteInspirationHold(ctx, id)
	if err != nil {
		return fmt.Errorf("delete the held roll: %w", err)
	}
	for _, r := range rows {
		if err := favorGiveBackIf(ctx, q, r.UnsettledFavor, r.CampaignID, r.CharacterID, r.IdempotencyKey); err != nil {
			return err
		}
	}
	return nil
}

// forgetStaleHoldsOf drops the held rolls of a roller that went stale (heldRoll.stale), giving back what the unanswered ones spent. The
// roller is read where it is (its combatant and the round, or the die its character holds); a hold that still waits stays.
func (s *Service) forgetStaleHoldsOf(ctx context.Context, env gateEnv, r heldRoller) error {
	st, err := s.rollerStateOf(ctx, env, r)
	if err != nil {
		return err
	}
	stores := env.stores()
	open, err := stores.openOf(ctx, r.in(st))
	if err != nil {
		return err
	}
	for _, h := range open {
		if h.stale(st) {
			if err := stores.forget(ctx, h); err != nil {
				return fmt.Errorf("forget the old held roll: %w", err)
			}
		}
	}
	return nil
}

// forgetOrphanHolds drops the held rolls of the campaign that wait for a Bardic Inspiration die that ran out, giving back the
// Inspiration they spent.
func forgetOrphanHolds(ctx context.Context, q *playdb.Queries, campaignID string) error {
	rows, err := q.DeleteOrphanInspirationHolds(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("forget the rolls that waited for a die that ran out: %w", err)
	}
	for _, r := range rows {
		if err := favorGiveBackIf(ctx, q, r.UnsettledFavor, r.CampaignID, r.CharacterID, r.IdempotencyKey); err != nil {
			return err
		}
	}
	return nil
}
