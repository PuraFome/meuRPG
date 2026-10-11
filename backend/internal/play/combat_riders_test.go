package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The monk's riders on a hit: the Open Hand technique (Flurry of Blows hits) and Stunning
// Strike (melee hits, 1 ki point). The target's save is rolled by the app, so the tests read
// its result from the log and check that what the result says is what happened.

func (a *armed) useRider(t *testing.T, u *user, e *playv1.Encounter, id string, choice playv1.HitRiderChoice) (*playv1.Encounter, error) {
	t.Helper()
	res, err := u.combat.UseHitRider(t.Context(), connect.NewRequest(&playv1.UseHitRiderRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), RiderId: id, Choice: choice, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetEncounter(), nil
}

func ridersOfKind(e *playv1.Encounter, kind playv1.HitRiderKind) []*playv1.HitRiderOffer {
	var out []*playv1.HitRiderOffer
	for _, r := range e.GetHitRiders() {
		if r.GetKind() == kind {
			out = append(out, r)
		}
	}
	return out
}

func (a *armed) lastRiderLine(t *testing.T) *playv1.CombatLogHitRider {
	t.Helper()
	var line *playv1.CombatLogHitRider
	for _, r := range a.log(t, a.master, a.get(t, a.master)).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_HIT_RIDER {
				line = en.GetHitRider()
			}
		}
	}
	if line == nil {
		t.Fatal("the log has no rider line")
	}
	return line
}

func riderTargetHas(e *playv1.Encounter, label, cond string) bool {
	for _, c := range e.GetCombatants() {
		if c.GetLabel() == label {
			for _, k := range c.GetConditions() {
				if k == cond {
					return true
				}
			}
		}
	}
	return false
}

func TestOpenHandRidersOnlyOnFlurryHits(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:monk", 3, "subclass:open-hand"), abilities(10, 16, 14, 14, 8), nil, nil, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.threeAndAGoblin(t)

	// The Attack action's strike is no Flurry strike: no Open Hand offer (and no Stunning Strike below monk 5).
	wantHit(t, "the Attack action", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	if got := a.get(t, a.caio).GetHitRiders(); len(got) != 0 {
		t.Fatalf("hit_riders after a plain hit = %v, want none", got)
	}
	if _, err := a.action(t, a.caio, e, "Toren", flurry); err != nil {
		t.Fatalf("Flurry of Blows error = %v", err)
	}
	wantHit(t, "a flurry strike", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	offers := ridersOfKind(a.get(t, a.caio), playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND)
	if len(offers) != 1 {
		t.Fatalf("Open Hand offers after a flurry hit = %d, want 1", len(offers))
	}
	if len(ridersOfKind(a.get(t, a.caio), playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE)) != 0 {
		t.Error("Stunning Strike offered to a level 3 monk")
	}
	// A player who is not the monk's never sees the offer.
	if got := a.get(t, a.ana).GetHitRiders(); len(got) != 0 {
		t.Errorf("another player reads %d riders, want 0", len(got))
	}
	// A choice of another rider is refused.
	_, err := a.useRider(t, a.caio, e, offers[0].GetId(), playv1.HitRiderChoice_HIT_RIDER_CHOICE_STUN)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("Stunning Strike choice on an Open Hand offer: code = %v, want invalid_argument", connect.CodeOf(err))
	}

	// Prone: the line says what the save did, and the condition follows it.
	after, err := a.useRider(t, a.caio, e, offers[0].GetId(), playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE)
	if err != nil {
		t.Fatalf("UseHitRider(prone) error = %v", err)
	}
	line := a.lastRiderLine(t)
	failed := line.GetResult() == playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED
	if failed != riderTargetHas(after, "Goblin", "condition:prone") {
		t.Errorf("prone = %v with the save result %v, want it only after a failed save", riderTargetHas(after, "Goblin", "condition:prone"), line.GetResult())
	}
	if !line.GetNumbers() || line.GetDc() < 10 {
		t.Errorf("the master's line numbers = %v dc %d, want the ki save DC (8 + 2 + 2 = 12)", line.GetNumbers(), line.GetDc())
	}
	// One choice per hit.
	_, err = a.useRider(t, a.caio, e, offers[0].GetId(), playv1.HitRiderChoice_HIT_RIDER_CHOICE_NO_REACTIONS)
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("a second choice on one hit: code = %v, want failed_precondition", connect.CodeOf(err))
	}
}

func TestOpenHandNoReactionsEndsWithTheMonksNextTurn(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:monk", 3, "subclass:open-hand"), abilities(10, 16, 14, 14, 8), nil, nil, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.threeAndAGoblin(t)
	wantHit(t, "the Attack action", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	if _, err := a.action(t, a.caio, e, "Toren", flurry); err != nil {
		t.Fatalf("Flurry of Blows error = %v", err)
	}
	wantHit(t, "a flurry strike", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	offer := ridersOfKind(a.get(t, a.caio), playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND)[0]
	if _, err := a.useRider(t, a.caio, e, offer.GetId(), playv1.HitRiderChoice_HIT_RIDER_CHOICE_NO_REACTIONS); err != nil {
		t.Fatalf("UseHitRider(no reactions) error = %v", err)
	}
	if line := a.lastRiderLine(t); line.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_APPLIED {
		t.Errorf("no reactions has no save: result = %v, want APPLIED", line.GetResult())
	}
	// The goblin has the effect now; it ends at the end of the monk's next turn, not before.
	hasEffect := func() bool {
		for _, c := range a.get(t, a.master).GetCombatants() {
			if c.GetLabel() == "Goblin" {
				return len(c.GetEffects()) > 0
			}
		}
		return false
	}
	if !hasEffect() {
		t.Fatal("the goblin has no effect after the no reactions choice")
	}
	e = a.turnOf(t, "Toren") // the monk's next turn begins: the effect still holds
	if !hasEffect() {
		t.Error("the effect ended before the end of the monk's next turn")
	}
	a.mustEndTurn(t, a.caio, e)
	if hasEffect() {
		t.Error("the effect lasted past the end of the monk's next turn")
	}
}

func TestStunningStrikeSpendsKiAndNeedsMonk5(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:monk", 5, ""), abilities(10, 16, 14, 14, 8), nil, nil, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.threeAndAGoblin(t)
	ki := func() int32 { used, _ := resourceUsed(a.vitals(t, a.toren), "ki"); return used }
	wantHit(t, "a melee hit", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	offers := ridersOfKind(a.get(t, a.caio), playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE)
	if len(offers) != 1 || len(ridersOfKind(a.get(t, a.caio), playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND)) != 0 {
		t.Fatalf("offers after a plain melee hit = %v, want Stunning Strike only", a.get(t, a.caio).GetHitRiders())
	}
	if offers[0].GetKiLeft() != 5 {
		t.Errorf("ki left on the offer = %d, want 5", offers[0].GetKiLeft())
	}
	after, err := a.useRider(t, a.caio, e, offers[0].GetId(), playv1.HitRiderChoice_HIT_RIDER_CHOICE_STUN)
	if err != nil {
		t.Fatalf("UseHitRider(stun) error = %v", err)
	}
	if ki() != 1 {
		t.Errorf("ki used = %d after Stunning Strike, want 1", ki())
	}
	failed := a.lastRiderLine(t).GetResult() == playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED
	if failed != riderTargetHas(after, "Goblin", "condition:stunned") {
		t.Errorf("stunned = %v with the save result %v, want it only after a failed save", riderTargetHas(after, "Goblin", "condition:stunned"), a.lastRiderLine(t).GetResult())
	}
}

func TestDecliningARiderSpendsNothing(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:monk", 5, ""), abilities(10, 16, 14, 14, 8), nil, nil, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.threeAndAGoblin(t)
	wantHit(t, "a melee hit", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	offer := ridersOfKind(a.get(t, a.caio), playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE)[0]
	if _, err := a.useRider(t, a.caio, e, offer.GetId(), playv1.HitRiderChoice_HIT_RIDER_CHOICE_DECLINE); err != nil {
		t.Fatalf("UseHitRider(decline) error = %v", err)
	}
	if used, _ := resourceUsed(a.vitals(t, a.toren), "ki"); used != 0 {
		t.Errorf("ki used = %d after declining, want 0", used)
	}
	if got := a.get(t, a.caio).GetHitRiders(); len(got) != 0 {
		t.Errorf("riders after declining = %d, want 0", len(got))
	}
}
