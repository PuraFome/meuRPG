package play

import (
	"context"
	"fmt"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The one way to drop a held roll. A spend of the Inspiration deletes the character's row and the hold remembers it
// (inspiration_holds.favor_spent); a hold dropped before it is answered is a roll that never happens, with no event to undo, so the
// Inspiration is lost unless it is given back here. Every delete of inspiration_holds goes through this file, before the row is gone:
// the generated Delete* queries are called nowhere else, and no path may rely on the foreign key's cascade (combatants and
// encounters delete their holds by it), so a path that deletes a combatant calls forgetHoldsOf first (archtest).

// favorGiveBackIf gives the Inspiration back when it was spent on a held roll that is dropped before it was answered: no roll event
// exists for that spend, so nothing else would put it back. A hold that was answered, or that spent nothing, gives nothing.
func favorGiveBackIf(ctx context.Context, q *playdb.Queries, unsettled bool, campaignID, characterID string) error {
	if !unsettled || characterID == "" {
		return nil
	}
	return favorGiveBack(ctx, q, campaignID, characterID)
}

// forgetHoldsOf drops every held roll of an owner (a combatant, or a character outside a combat), giving back the Inspiration the
// unanswered ones spent.
func forgetHoldsOf(ctx context.Context, q *playdb.Queries, owner holdOwner) error {
	rows, err := q.DeleteInspirationHoldsOf(ctx, playdb.DeleteInspirationHoldsOfParams(owner))
	if err != nil {
		return fmt.Errorf("delete the held rolls: %w", err)
	}
	for _, r := range rows {
		if err := favorGiveBackIf(ctx, q, r.UnsettledFavor, r.CampaignID, r.CharacterID); err != nil {
			return err
		}
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
		if err := favorGiveBackIf(ctx, q, r.UnsettledFavor, r.CampaignID, r.CharacterID); err != nil {
			return err
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
		if err := favorGiveBackIf(ctx, q, r.UnsettledFavor, r.CampaignID, r.CharacterID); err != nil {
			return err
		}
	}
	return nil
}
