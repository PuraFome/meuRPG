package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A grapple needs a free hand and a shove needs none (SRD 5.1). A character in a beast form has no hand to spare, unless the beast's own
// stat block has an attack that grapples; a character that is not in a form is as it was.

// grappleKinds are the kinds of the special attacks a turn lists.
func grappleKinds(opts *playv1.GetTurnOptionsResponse) (grapple, shove bool) {
	for _, o := range opts.GetContestAttackOptions() {
		switch o.GetKind() {
		case playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_GRAPPLE:
			grapple = true
		case playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_SHOVE:
			shove = true
		case playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_UNSPECIFIED:
		}
	}
	return grapple, shove
}

// TestFreeHand_ABeastFormListsNoGrappleAndRefusesIt: before the form Sálvia lists Agarrar and Empurrar; as the wolf her turn lists only
// Empurrar, a GRAPPLE is refused with NO_FREE_HAND for her player and for the master, and the shove is taken.
func TestFreeHand_ABeastFormListsNoGrappleAndRefusesIt(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	e := s.fight(t)
	if grapple, shove := grappleKinds(s.mustOptions(t, s.bia, e, "Sálvia")); !grapple || !shove {
		t.Fatalf("Sálvia as herself lists grapple %v, shove %v; want both", grapple, shove)
	}
	res := s.mustAssume(t, s.bia, s.bri, wolfKey)
	for who, u := range map[string]*user{"her player": s.bia, "the master": s.master} {
		if grapple, shove := grappleKinds(s.mustOptions(t, u, res.GetEncounter(), "Sálvia")); grapple || !shove {
			t.Errorf("the wolf's options for %s list grapple %v, shove %v; want only the shove", who, grapple, shove)
		}
	}
	start := func(u *user, purpose playv1.ContestPurpose) error {
		_, err := u.contests.StartContest(t.Context(), connect.NewRequest(&playv1.StartContestRequest{
			CampaignId: s.campaignID, EncounterId: res.GetEncounter().GetId(), IdempotencyKey: newKey(), Roll: rollIn(),
			InitiatorId: byLabel(t, res.GetEncounter(), "Sálvia").GetId(), TargetId: byLabel(t, res.GetEncounter(), "Capitão Goblin").GetId(), Purpose: purpose,
		}))
		return err
	}
	for who, u := range map[string]*user{"her player": s.bia, "the master": s.master} {
		wantContestBlocked(t, "GRAPPLE by "+who, start(u, playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE), playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NO_FREE_HAND)
	}
	// The change of form spent the wolf's action, so the shove may be refused for that, but never for hands.
	if err := start(s.bia, playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE); err != nil && strings.Contains(err.Error(), "no free hand") {
		t.Errorf("SHOVE from a beast form was refused for hands: %v", err)
	}
}

// TestFreeHand_ACreatureWithHandsOrAGrappleAttackKeepsTheGrapple: the helper that decides is true only for a stat block with no attack
// that grapples; a stat block with one (a constrictor's Constrict) is not handless.
func TestFreeHand_ACreatureWithHandsOrAGrappleAttackKeepsTheGrapple(t *testing.T) {
	t.Parallel()
	bite := rules.CreatureAction{Name: "Bite", Plan: &rules.ActionPlan{}}
	constrict := rules.CreatureAction{Name: "Constrict", Plan: &rules.ActionPlan{HitCondition: &rules.ActionHitCondition{ConditionKey: "condition:grappled", EscapeDC: 14}}}
	for name, tc := range map[string]struct {
		creature rules.Creature
		want     bool
	}{
		"a wolf's stat block":      {rules.Creature{Actions: []rules.CreatureAction{bite}}, false},
		"a constrictor's":          {rules.Creature{Actions: []rules.CreatureAction{bite, constrict}}, true},
		"an action with no plan":   {rules.Creature{Actions: []rules.CreatureAction{{Name: "Slam"}}}, false},
		"a stat block with no one": {rules.Creature{}, false},
	} {
		if got := beastHoldsByAttack(tc.creature); got != tc.want {
			t.Errorf("%s: beastHoldsByAttack = %v, want %v", name, got, tc.want)
		}
	}
}
