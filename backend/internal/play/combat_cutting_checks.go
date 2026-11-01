package play

import (
	"context"
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// Cutting Words on an ability check (SRD, College of Lore): a creature the bard sees within 60
// feet makes an ability check, and the bard may spend the reaction and a use of Bardic
// Inspiration to take the die off its roll before the master decides how the check ends. The
// check is held as an attack roll is (combat_reaction_hold.go): the request is kept, the windows
// are opened before anything is rolled or spent, and when they are answered the server runs the
// request again, and the roll comes out with the die taken off (cutRoll). The sites: the roll of a
// contest's initiator, the answer to a contest, and a Hide. A roll made outside a combat has no
// window to wait in (no encounter), and the Help action rolls nothing.

// The reaction window kinds of a held check, as the hold keeps them.
const (
	holdContest       = "contest"
	holdContestAnswer = "contest_answer"
	holdHide          = "hide"
	holdHitRider      = "hit_rider"
)

// holdCheck holds the roll of a combatant's ability check when a Cutting Words can change it: it
// opens the window of each bard of the other side that sees the roller, keeps the request and
// answers the event that stands for the held roll. target is the other creature of a contest (nil
// for a Hide). Nothing is rolled or spent until every window is answered. It says false when nothing
// waits, and the check is rolled at once.
func (s *Service) holdCheck(ctx context.Context, c *combatTx, m authz.Membership, kind string, req proto.Message, cs []playdb.Combatant, roller playdb.Combatant, target *playdb.Combatant, in checkInput) (actionEvent, bool, error) {
	if c.replay != nil {
		return actionEvent{}, false, nil
	}
	// The answer to a question about the d20 comes after the windows: they were answered before the d20 was kept.
	if _, answering := heldReplyOf(ctx); answering {
		return actionEvent{}, false, nil
	}
	base := windowTrigger{Actor: roller.ID}
	if target != nil {
		base.Target = target.ID
	}
	specs, err := s.cuttingWindows(ctx, c, cs, roller, "test", base)
	if err != nil {
		return actionEvent{}, false, err
	}
	// "Sempre": a player's check against the enemy waits for the master's check also when no bard
	// could cut it, so the pause tells the table nothing about a bard (RN-10).
	if len(specs) == 0 && c.rules.EnemyReactionsAlways && playerChecksAgainstEnemy(cs, roller, target) {
		t := base
		t.Roll = "test"
		specs = append(specs, windowSpec{kind: reaction.MasterCheck, trigger: t})
	}
	if len(specs) == 0 {
		return actionEvent{}, false, nil
	}
	if err := s.mustRollThisWay(ctx, c.tx, m, in.rollInput()); err != nil {
		return actionEvent{}, false, err
	}
	ev := actionEvent{Round: c.enc.Round, Secret: roller.Hidden || (target != nil && target.Hidden), Actor: roller.ID, Target: base.Target}
	if ev, err = s.holdAction(ctx, c, m, kind, roller, req, holdData{}, specs, ev); err != nil {
		return ev, false, err
	}
	if err := touch(ctx, c); err != nil {
		return ev, false, err
	}
	c.characterID = &roller.CharacterID
	return ev, true, nil
}

// playerChecksAgainstEnemy says a player's check is aimed at an enemy NPC (the other creature of a
// contest), or has no other creature (a Hide) and an enemy NPC is in the combat.
func playerChecksAgainstEnemy(cs []playdb.Combatant, roller playdb.Combatant, target *playdb.Combatant) bool {
	if target != nil {
		return playerAgainstEnemy(roller, *target)
	}
	return roller.Kind == kindPlayer && slices.ContainsFunc(cs, func(o playdb.Combatant) bool { return playerAgainstEnemy(roller, o) && !o.Defeated })
}

// holdContestRoll holds the initiator's roll of a contest (a grapple, a shove, an escape).
func (s *Service) holdContestRoll(ctx context.Context, r *contestRun) (actionEvent, bool, error) {
	if r.st.byAttack {
		return actionEvent{}, false, nil // the attack was rolled already: nobody rolls
	}
	return s.holdCheck(ctx, r.c, r.st.m, holdContest, r.st.req, r.cs, r.initiator, &r.defender, r.st.in)
}

// holdContestAnswer holds the defender's roll of a contest. A roll the player leaves to the master
// is rolled when the master answers.
func (s *Service) holdContestAnswer(ctx context.Context, r *answerRun) (actionEvent, bool, error) {
	if r.ans.leave {
		return actionEvent{}, false, nil
	}
	return s.holdCheck(ctx, r.c, r.ans.m, holdContestAnswer, r.ans.req, r.cs, r.defender, &r.initiator, r.ans.in)
}

// replayHeldCheck runs the request of a held check again, as the one who made it.
func (s *Service) replayHeldCheck(ctx context.Context, m authz.Membership, h playdb.ReactionHold) error {
	var err error
	switch h.Kind {
	case holdContest:
		var req playv1.StartContestRequest
		if err = proto.Unmarshal(h.Request, &req); err == nil {
			_, err = s.startContest(ctx, m, &req)
		}
	case holdContestAnswer:
		var req playv1.RespondContestRequest
		if err = proto.Unmarshal(h.Request, &req); err == nil {
			_, err = s.respondContest(ctx, m, &req)
		}
	case holdHide:
		var req playv1.HideRequest
		if err = proto.Unmarshal(h.Request, &req); err == nil {
			_, err = s.hide(ctx, m, &req)
		}
	default:
		return fmt.Errorf("a held action of kind %q", h.Kind)
	}
	return err
}

// The circumstance a cut check keeps, to be read beside the dice.
const (
	noteCutting  = "cutting_words"
	labelCutting = "Palavras de Interrupção"
)

// cutRoll takes the die of the Cutting Words windows that were answered off the roll of a replayed
// check, and says so in the roll's notes. The roll of anything else is the same.
func cutRoll(c *combatTx, roll contestRoll) contestRoll {
	if c.replay == nil {
		return roll
	}
	if cut := c.replay.reduction; cut > 0 {
		roll.Total -= cut
		roll.Notes = append(slices.Clone(roll.Notes), rollNote{Kind: noteCutting, Label: fmt.Sprintf("%s: −%d", labelCutting, cut)})
	}
	c.replay.after = roll.Total
	return roll
}
