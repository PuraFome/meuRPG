package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master's by-hand "Ataque de oportunidade" on a combat with a grid (MR-034, RN-20, RN-25, #643).
// The server still finds the opportunity attacks of a move; this offer is the master's word that
// someone left someone's reach, without a square, a reach check or the fog. These tests need the
// database (MEURPG_TEST_DATABASE_URL).

// declineAs calls DeclineOpportunity as u.
func (a *armed) declineAs(t *testing.T, u *user, e *playv1.Encounter, offerID string) error {
	t.Helper()
	_, err := u.combat.DeclineOpportunity(t.Context(), connect.NewRequest(&playv1.DeclineOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: offerID, IdempotencyKey: newKey(),
	}))
	return err
}

// squareOf is where the master sees a combatant stand.
func (a *armed) squareOf(t *testing.T, label string) [2]int32 {
	t.Helper()
	c := byLabel(t, a.get(t, a.master), label)
	return [2]int32{c.GetCol(), c.GetRow()}
}

// TestMR034_TheMasterOffersAnAttackOnAGrid: on a map the master offers an opportunity attack by hand,
// which the reactor's player answers with RollAttack (from any distance: nobody checks the reach),
// declines, or the master takes back; a hit on the mover moves nobody and a lethal one defeats it.
func TestMR034_TheMasterOffersAnAttackOnAGrid(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t) // Toren (3,3), Goblin (4,3), Pensantus (10,3), Brisa (8,8)
	a.passTo(t, e, "Goblin")
	offers := func(u *user) []*playv1.OpportunityOffer { return a.get(t, u).GetOpportunityOffers() }

	// A player cannot offer; the master can, once per pair.
	_, err := a.offer(t, a.caio, e, "Goblin", "Toren")
	wantCode(t, "a player's OfferOpportunity", err, connect.CodePermissionDenied)
	offerID := a.mustOffer(t, e, "Goblin", "Toren").GetOpportunityOfferId()
	_, err = a.offer(t, a.master, e, "Goblin", "Toren")
	wantBlockedBy(t, "the same offer twice", err, blockedNoOpportunity)
	if got := offers(a.caio); len(got) != 1 || got[0].GetId() != offerID || !got[0].GetForYou() || !got[0].GetByHand() || len(got[0].GetAttacks()) == 0 {
		t.Errorf("Toren's player reads %v, want the offer for him, by hand, with his attacks", got)
	}
	if got := offers(a.master); len(got) != 1 || got[0].GetReactorLabel() != "Toren" || got[0].GetMoverLabel() != "Goblin" {
		t.Errorf("the master reads %v, want Goblin leaving Toren's reach", got)
	}
	for who, u := range map[string]*user{"Pensantus's player": a.ana, "Brisa's player": a.bia} {
		if got := offers(u); len(got) != 0 {
			t.Errorf("%s reads %v, want nothing", who, got)
		}
	}

	// Toren's player attacks with the offer. The hit moves the goblin nowhere.
	before := a.squareOf(t, "Goblin")
	hit, err := a.attackWithOffer(t, a.caio, e, "Toren", battleaxe, "Goblin", offerID, d20(15))
	if err != nil {
		t.Fatalf("the opportunity attack error = %v", err)
	}
	if !byLabel(t, hit.GetEncounter(), "Toren").GetReactionUsed() {
		t.Error("the opportunity attack did not spend Toren's reaction")
	}
	a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), typedDamage(1))
	if after := a.squareOf(t, "Goblin"); after != before {
		t.Errorf("the goblin stands at %v after the by-hand attack, want %v", after, before)
	}
	_, err = a.offer(t, a.master, e, "Goblin", "Toren")
	wantBlockedBy(t, "an offer to a reactor whose reaction is used", err, blockedReactionUsed)

	// Brisa stands 6 squares away and attacks all the same; a lethal hit defeats the goblin, still where it was.
	far := a.mustOffer(t, e, "Goblin", "Brisa").GetOpportunityOfferId()
	hit, err = a.attackWithOffer(t, a.bia, e, "Brisa", rapier, "Goblin", far, d20(15))
	if err != nil {
		t.Fatalf("the opportunity attack from afar error = %v", err)
	}
	a.mustDamage(t, a.bia, e, hit.GetPendingDamage().GetId(), typedDamage(8))
	gob := byLabel(t, a.get(t, a.master), "Goblin")
	if !gob.GetDefeated() || a.squareOf(t, "Goblin") != before {
		t.Errorf("the goblin after a lethal by-hand attack = defeated %v at %v, want defeated at %v", gob.GetDefeated(), a.squareOf(t, "Goblin"), before)
	}
	_, err = a.offer(t, a.master, e, "Goblin", "Pensantus")
	wantBlockedBy(t, "an offer for a defeated mover", err, blockedNoOpportunity)
}

// TestMR034_AByHandOfferOnAGridIsDeclinedWithdrawnAndWaitedFor: the reactor's player declines and keeps
// the reaction; the master takes an offer back; the mover's turn waits for the answer.
func TestMR034_AByHandOfferOnAGridIsDeclinedWithdrawnAndWaitedFor(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	a.passTo(t, e, "Goblin")

	declined := a.mustOffer(t, e, "Goblin", "Pensantus").GetOpportunityOfferId()
	if err := a.declineAs(t, a.ana, e, declined); err != nil {
		t.Fatalf("DeclineOpportunity() error = %v", err)
	}
	if byLabel(t, a.get(t, a.master), "Pensantus").GetReactionUsed() || len(a.get(t, a.master).GetOpportunityOffers()) != 0 {
		t.Error("declining spent Pensantus's reaction or left the offer")
	}
	taken := a.mustOffer(t, e, "Goblin", "Brisa").GetOpportunityOfferId()
	if err := a.withdraw(t, a.bia, e, taken); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player's WithdrawOpportunity: %v, want permission_denied", err)
	}
	if err := a.withdraw(t, a.master, e, taken); err != nil {
		t.Fatalf("WithdrawOpportunity() error = %v", err)
	}
	if got := a.get(t, a.bia).GetOpportunityOffers(); len(got) != 0 || byLabel(t, a.get(t, a.master), "Brisa").GetReactionUsed() {
		t.Errorf("after the withdrawal Brisa's player reads %v, want nothing and her reaction kept", got)
	}

	// A player mover: the turn waits until the master answers for the goblin.
	a.passTo(t, e, "Toren")
	waiting := a.mustOffer(t, e, "Toren", "Goblin").GetOpportunityOfferId()
	_, err := a.endTurn(t, a.caio, e, false)
	wantBlockedBy(t, "EndTurn while an opportunity attack waits", err, reasonOpportunityPending)
	if err := a.withdraw(t, a.master, e, waiting); err != nil {
		t.Fatalf("WithdrawOpportunity() error = %v", err)
	}
	if _, err := a.endTurn(t, a.caio, e, false); err != nil {
		t.Errorf("EndTurn() after the withdrawal error = %v", err)
	}
}

// TestMR034_AByHandOfferOnAGridRefusesWhatCannotReact: the reactor on the mover's side, a surprised one,
// a hidden mover, and a move's own pending offer for the same pair are all refused.
func TestMR034_AByHandOfferOnAGridRefusesWhatCannotReact(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {8, 8}}, setup: true,
	})
	if _, err := a.master.contests.SetSurprised(t.Context(), connect.NewRequest(&playv1.SetSurprisedRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(), Surprised: true,
	})); err != nil {
		t.Fatalf("SetSurprised() error = %v", err)
	}
	e = a.begin(t, e)
	_, err := a.offer(t, a.master, e, "Toren", "Goblin")
	wantBlockedBy(t, "a surprised reactor", err, blockedNoOpportunity)
	_, err = a.offer(t, a.master, e, "Toren", "Pensantus")
	wantBlockedBy(t, "a reactor on the mover's side", err, blockedNoOpportunity)

	a.passTo(t, e, "Goblin")
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(), Hidden: true,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	_, err = a.offer(t, a.master, e, "Goblin", "Toren")
	wantBlockedBy(t, "a hidden mover", err, blockedNoOpportunity)

	// Control: once the goblin's first turn has ended it is no longer surprised, and the same offer is accepted.
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(), Hidden: false,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	a.passTo(t, e, "Toren")
	a.mustOffer(t, e, "Toren", "Goblin")
}

// TestMR034_AMovesPendingOfferBlocksTheSameOfferByHand: the offer a move made waits; the master cannot add a
// second one for the same mover and reactor.
func TestMR034_AMovesPendingOfferBlocksTheSameOfferByHand(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	e := c.oppFight(t)
	c.leaveGoblin(t)
	if got := c.offersOf(t, c.master); len(got) != 1 || got[0].GetByHand() {
		t.Fatalf("the move's offers = %v, want one that is not by hand", got)
	}
	_, err := c.offer(t, c.master, e, "Toren", "Goblin 1")
	wantBlockedBy(t, "a by-hand offer beside the move's", err, blockedNoOpportunity)
}

// TestRN20_AByHandOfferOnAGridNamesNoHiddenReactor: the mover's player reads that an attack waits but not who
// makes it when the master hid the reactor; nobody else reads the offer; the master, and the reactor's
// controller, read it all.
func TestRN20_AByHandOfferOnAGridNamesNoHiddenReactor(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		at:      map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {8, 8}},
	}) // the goblin stays hidden; Toren is on turn
	gob := a.id(t, "Goblin")
	offerID := a.mustOffer(t, e, "Toren", "Goblin").GetOpportunityOfferId()

	// Positive control: the master reads who and what.
	mine := a.get(t, a.master).GetOpportunityOffers()
	if len(mine) != 1 || mine[0].GetReactorId() != gob || mine[0].GetReactorLabel() != "Goblin" {
		t.Fatalf("the master reads %v, want the goblin as the reactor", mine)
	}
	// The mover's player reads the offer but not the reactor.
	got := a.get(t, a.caio).GetOpportunityOffers()
	if len(got) != 1 || got[0].GetId() != offerID || got[0].GetReactorId() != "" || got[0].GetReactorLabel() != "" || got[0].GetForYou() {
		t.Errorf("the mover's player reads %v, want the offer without the reactor", got)
	}
	// Canary: another player reads nothing of it, in the combat or in the log.
	for what, js := range map[string]string{
		"the combat": protojson.Format(a.get(t, a.ana)),
		"the log":    protojson.Format(a.log(t, a.ana, e)),
	} {
		if strings.Contains(js, gob) || strings.Contains(js, "Goblin") || strings.Contains(js, offerID) {
			t.Errorf("another player reads %s with the hidden reactor or its offer in it: %s", what, js)
		}
	}
	for what, js := range map[string]string{
		"the combat": protojson.Format(a.get(t, a.caio)),
		"the log":    protojson.Format(a.log(t, a.caio, e)),
	} {
		if strings.Contains(js, gob) || strings.Contains(js, "Goblin") {
			t.Errorf("the mover's player reads %s with the hidden reactor in it: %s", what, js)
		}
	}
	if !strings.Contains(protojson.Format(a.log(t, a.master, e)), "Goblin") {
		t.Error("the master's log no longer has the goblin (the canaries above would pass for nothing)")
	}
}

// TestRN10_AByHandOfferOnAFoggedMoverIsTheMastersToClear: the master's offer ignores the fog, but the
// reactor's player must see the mover to be asked: on a fog map they read nothing of an unseen mover's
// offer, and the master withdraws it.
func TestRN10_AByHandOfferOnAFoggedMoverIsTheMastersToClear(t *testing.T) {
	t.Parallel()
	f := newFogCave(t)
	e := f.fight(t)
	f.stage(t) // the Capitão is in the dark corridor: Pensantus sees it, Toren does not
	f.passTo(t, e, "Capitão Goblin")
	capitao := f.id(t, "Capitão Goblin")

	offerID := f.mustOffer(t, e, "Capitão Goblin", "Toren").GetOpportunityOfferId()
	// Positive control: the master reads it.
	if got := f.get(t, f.master).GetOpportunityOffers(); len(got) != 1 || got[0].GetId() != offerID || got[0].GetReactorLabel() != "Toren" {
		t.Fatalf("the master reads %v, want the offer", got)
	}
	// Canary: Toren's player does not see the mover, so reads nothing of the offer, nor of the mover.
	if got := f.get(t, f.caio).GetOpportunityOffers(); len(got) != 0 {
		t.Errorf("Toren's player reads %v of an unseen mover's offer", got)
	}
	f.noLeak(t, "Toren's encounter", asJSON(t, f.get(t, f.caio)), "Capitão Goblin")
	if js := asJSON(t, f.get(t, f.caio)); strings.Contains(js, capitao) || strings.Contains(js, offerID) {
		t.Errorf("Toren's player reads the unseen mover or its offer: %s", js)
	}
	if err := f.withdraw(t, f.master, e, offerID); err != nil {
		t.Fatalf("WithdrawOpportunity() error = %v", err)
	}
	if got := f.get(t, f.master).GetOpportunityOffers(); len(got) != 0 {
		t.Errorf("the master still reads %v after the withdrawal", got)
	}
}
