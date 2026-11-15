package play

import (
	"context"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A grapple needs at least one free hand, and a shove needs none (SRD 5.1, "Grappling", "Shoving a Creature"). A player's character in a
// beast form has no hand to spare, whatever the beast: the app keeps no list of hands, the form is the signal. The one way a form grapples
// is an attack of its own stat block that does (a constrictor's Constrict): that grapple is the master's, with the attack's escape DC
// (kind ESCAPE_DC), and the form is not handless for it. The turn options leave the grapple out and StartContest refuses it.

// beastHoldsByAttack says a stat block has an attack that grapples on a hit.
func beastHoldsByAttack(creature rules.Creature) bool {
	for _, a := range creature.Actions {
		if a.Plan != nil && a.Plan.HitCondition != nil {
			return true
		}
	}
	return false
}

// noFreeHand says the combatant is a player's character in a beast form whose stat block has no attack that grapples. tx may be nil.
func (s *Service) noFreeHand(ctx context.Context, tx pgx.Tx, campaignID string, who playdb.Combatant) (bool, error) {
	if who.Kind != kindPlayer {
		return false, nil
	}
	var (
		v   *playv1.CharacterVitals
		err error
	)
	if tx == nil {
		v, err = s.vitals.GetVitals(ctx, campaignID, who.CharacterID)
	} else {
		v, err = s.vitals.GetVitalsTx(ctx, tx, campaignID, who.CharacterID)
	}
	if err != nil {
		return false, err
	}
	w := v.GetWildShape()
	if w == nil {
		return false, nil
	}
	content, err := s.roster.RulesContent(ctx, tx, campaignID)
	if err != nil {
		return false, err
	}
	creature, ok := content.CreatureByKey(w.GetBeastKey())
	return !ok || !beastHoldsByAttack(creature), nil
}

// mustHaveFreeHand refuses a grapple of a combatant with no free hand. Nothing is spent.
func (s *Service) mustHaveFreeHand(ctx context.Context, c *combatTx, who playdb.Combatant) error {
	handless, err := s.noFreeHand(ctx, c.tx, c.session.CampaignID, who)
	if err != nil {
		return err
	}
	if handless {
		return errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NO_FREE_HAND, "a beast form has no free hand to grapple")
	}
	return nil
}
