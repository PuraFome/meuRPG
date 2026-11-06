package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Legendary Resistance on the save of a hit rider (SRD 5.1, Monk, Stunning Strike): the answer of a rider that rolls
// the save of an adult red dragon waits in a window of the master's alone, with the d20 kept, whether the save passed
// or failed; the master turns the failure into a success or not, and the rider goes on with the very same dice.
// These tests need the database (MEURPG_TEST_DATABASE_URL).

const (
	stunChoice  = playv1.HitRiderChoice_HIT_RIDER_CHOICE_STUN
	proneChoice = playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE
	stunnedCond = "condition:stunned"
)

// riderDragonFight is a combat of a monk of level 5 (Toren, the Open Hand, Wisdom 20: the ki save DC is 16, so the
// dragon's Constitution save of +13 fails on a 1 or a 2 and its Dexterity save of +6 on a 9 or less), a goblin and
// the named creature, side by side east of the monk. The monk's strikes hit; the offers it leaves open are returned by
// kind: the Attack action's first strike offers a Stunning Strike, the strike after Flurry of Blows an Open Hand.
func riderDragonFight(t *testing.T, creature string) (*armed, *playv1.Encounter, map[playv1.HitRiderKind]string) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:monk", 5, "subclass:open-hand"), abilities(10, 16, 14, 20, 8), nil, nil, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{1},
		players: map[string]int32{"Toren": 20, "Pensantus": 10, "Brisa": 5},
		reveal:  []string{"Goblin"}, setup: true,
	})
	a.h.roller.queue(1)
	a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Name, r.Hidden = creature, 1, "Draco", new(false)
	})
	a.begin(t, a.get(t, a.master))
	a.put(t, "Toren", 3, 3)
	a.put(t, "Draco", 4, 3)
	a.put(t, "Goblin", 3, 4)
	a.put(t, "Pensantus", 10, 3)
	a.put(t, "Brisa", 8, 8)
	e = a.get(t, a.master)
	if e.GetCurrentCombatantId() != byLabel(t, e, "Toren").GetId() {
		t.Fatalf("on turn = %s, want Toren", e.GetCurrentCombatantId())
	}
	wantHit(t, "the Attack action", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Draco", d20(15)), hit)
	offers := a.riderOffers(t)
	return a, e, offers
}

// riderOffers is the monk's open offers by kind (one of each: the first of its kind).
func (a *armed) riderOffers(t *testing.T) map[playv1.HitRiderKind]string {
	t.Helper()
	out := map[playv1.HitRiderKind]string{}
	for _, o := range a.get(t, a.caio).GetHitRiders() {
		if _, ok := out[o.GetKind()]; !ok {
			out[o.GetKind()] = o.GetId()
		}
	}
	return out
}

// stunOffer is the monk's open Stunning Strike offer.
func (a *armed) stunOffer(t *testing.T) string {
	t.Helper()
	id := a.riderOffers(t)[playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE]
	if id == "" {
		t.Fatalf("the monk reads no Stunning Strike offer: %v", a.get(t, a.caio).GetHitRiders())
	}
	return id
}

func (a *armed) kiUsed(t *testing.T) int32 {
	t.Helper()
	used, _ := resourceUsed(a.vitals(t, a.toren), "ki")
	return used
}

// openRiders counts the offers the monk's player still reads.
func (a *armed) openRiders(t *testing.T) int {
	t.Helper()
	return len(a.get(t, a.caio).GetHitRiders())
}

// stunHeld answers the Stunning Strike offer and returns the master's window of the held answer.
func (a *armed) stunHeld(t *testing.T, e *playv1.Encounter, d20s ...int) *playv1.ReactionWindow {
	t.Helper()
	a.h.roller.queue(d20s...)
	if _, err := a.useRider(t, a.caio, e, a.stunOffer(t), stunChoice); err != nil {
		t.Fatalf("UseHitRider() error = %v", err)
	}
	w := a.windowOf(t, a.master, resistKind)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the Legendary Resistance window", a.windows(t, a.master))
	}
	return w
}

// A save the dragon passes holds the answer all the same: the monk's player reads only the wait any held action gives,
// nothing was spent (no ki, no use), the offer stays open and no line is written; the window offers nothing, and the
// master's PASS lets the rider go on with the very same d20.
func TestHitRiderResistance_APassedSaveIsHeldAnyway(t *testing.T) {
	t.Parallel()
	a, e, _ := riderDragonFight(t, adultRedDragon)
	before := a.events(t)
	w := a.stunHeld(t, e, 20, 20)
	asked := a.h.roller.taken()
	if offers := w.GetLegendaryResistance().GetOffers(); len(offers) != 0 {
		t.Errorf("the offers = %v, want none: the save passed", offers)
	}
	if n := a.events(t); n != before+1 {
		t.Errorf("the held answer wrote %d events, want 1 (the hold's)", n-before)
	}
	if a.kiUsed(t) != 0 || a.resistSpentBy(t, "Draco") != 0 || a.openRiders(t) != 1 {
		t.Errorf("while it waits: ki used %d, uses spent %d, open offers %d; want 0, 0, 1", a.kiUsed(t), a.resistSpentBy(t, "Draco"), a.openRiders(t))
	}
	for _, u := range []*user{a.caio, a.ana, a.bia} {
		if a.windowOf(t, u, resistKind) != nil || len(a.windows(t, u)) != 0 {
			t.Errorf("a player reads the windows %v, want none", a.windows(t, u))
		}
		if wait := a.get(t, u).GetReactionWait(); wait.GetTitlePt() != "Esperando o mestre" {
			t.Errorf("a player's wait = %q, want \"Esperando o mestre\"", wait.GetTitlePt())
		}
	}
	if _, err := a.resistUse(t, a.master, e, w); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if again := a.h.roller.taken(); again != asked {
		t.Errorf("the replay rolled %d dice more, want none", again-asked)
	}
	line := a.lastRiderLine(t)
	if line.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_SAVED || line.GetD20() != 20 || line.GetResisted() {
		t.Errorf("the master's line = %v, want the save that passed with the kept 20", line)
	}
	if a.kiUsed(t) != 1 || a.resistSpentBy(t, "Draco") != 0 || a.openRiders(t) != 0 {
		t.Errorf("after the answer: ki used %d, uses spent %d, open offers %d; want 1, 0, 0", a.kiUsed(t), a.resistSpentBy(t, "Draco"), a.openRiders(t))
	}
	if riderTargetHas(a.get(t, a.master), "Draco", stunnedCond) {
		t.Error("the dragon is stunned by a save that passed")
	}
}

// The failed save and USE: the dragon is not stunned, one use is spent, the d20 is the one that was rolled (the roller
// is not asked again), the master reads the roll that failed with the choice and the uses left, and a player reads a
// save that passed with no number. The ki is spent and the offer closed only by the replay.
func TestHitRiderResistance_TheMasterTurnsTheFailedSaveIntoASuccess(t *testing.T) {
	t.Parallel()
	a, e, _ := riderDragonFight(t, adultRedDragon)
	w := a.stunHeld(t, e, 1, 20, 20) // a 1 and 13: 14 against DC 16
	asked := a.h.roller.taken()
	offers := w.GetLegendaryResistance().GetOffers()
	if len(offers) != 1 || offers[0].GetCombatantId() != a.id(t, "Draco") || offers[0].GetD20() != 1 || offers[0].GetTotal() != 14 || offers[0].GetDc() != 16 || offers[0].GetUsesLeft() != 3 {
		t.Fatalf("the offers = %v, want Draco's 1 (14 against 16) with 3 uses", offers)
	}
	if a.kiUsed(t) != 0 || a.openRiders(t) != 1 || riderTargetHas(a.get(t, a.master), "Draco", stunnedCond) {
		t.Errorf("while it waits: ki used %d, open offers %d; want 0 and 1, and no stun", a.kiUsed(t), a.openRiders(t))
	}
	res, err := a.resistUse(t, a.master, e, w, "Draco")
	if err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if r := res.GetResult(); !r.GetUsed() || len(r.GetLegendaryResistance().GetCombatantIds()) != 1 {
		t.Errorf("the result = %v, want the trait used on Draco", r)
	}
	if again := a.h.roller.taken(); again != asked {
		t.Errorf("the replay rolled %d dice more, want none: the d20 is the one that was kept", again-asked)
	}
	if got := a.resistSpentBy(t, "Draco"); got != 1 {
		t.Errorf("the dragon spent %d uses, want 1", got)
	}
	if riderTargetHas(a.get(t, a.master), "Draco", stunnedCond) {
		t.Error("the dragon is stunned although the master made the save a success")
	}
	if a.kiUsed(t) != 1 || a.openRiders(t) != 0 || a.windowOf(t, a.master, resistKind) != nil {
		t.Errorf("after the answer: ki used %d, open offers %d, window open %v; want 1, 0, false", a.kiUsed(t), a.openRiders(t), a.windowOf(t, a.master, resistKind) != nil)
	}
	master := a.lastRiderLineOf(t, a.master)
	if master.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_SAVED || !master.GetResisted() || master.GetResistedLeft() != 2 || master.GetD20() != 1 || master.GetSaveTotal() != 14 || master.GetDc() != 16 {
		t.Errorf("the master's line = %v, want the failed 1 (14 against 16) made a success with 2 uses left", master)
	}
	for _, u := range []*user{a.caio, a.ana, a.bia} {
		line := a.lastRiderLineOf(t, u)
		if line.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_SAVED || line.GetResisted() || line.GetResistedLeft() != 0 || line.GetNumbers() || line.GetD20() != 0 || line.GetSaveTotal() != 0 || line.GetDc() != 0 {
			t.Errorf("a player's line = %v, want a save that passed and nothing else", line)
		}
	}
	if l := a.legendaryOf(t, a.master); l.GetResistanceLeft() != 2 {
		t.Errorf("the master reads %v, want 2 uses left", l)
	}
}

// The failed save and PASS: the rider lands with the roll that was kept (the dragon is stunned), nothing is spent, and
// the line is the failure.
func TestHitRiderResistance_DecliningLeavesTheFailedSaveFailed(t *testing.T) {
	t.Parallel()
	a, e, _ := riderDragonFight(t, adultRedDragon)
	w := a.stunHeld(t, e, 2, 20)
	asked := a.h.roller.taken()
	if _, err := a.resistUse(t, a.master, e, w); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if again := a.h.roller.taken(); again != asked {
		t.Errorf("the replay rolled %d dice more, want none", again-asked)
	}
	if !riderTargetHas(a.get(t, a.master), "Draco", stunnedCond) {
		t.Error("the dragon is not stunned by a save the master left failed")
	}
	if a.resistSpentBy(t, "Draco") != 0 || a.kiUsed(t) != 1 || a.openRiders(t) != 0 {
		t.Errorf("uses spent %d, ki used %d, open offers %d; want 0, 1, 0", a.resistSpentBy(t, "Draco"), a.kiUsed(t), a.openRiders(t))
	}
	if line := a.lastRiderLine(t); line.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED || line.GetResisted() || line.GetD20() != 2 {
		t.Errorf("the master's line = %v, want the failure of the kept 2", line)
	}
	if line := a.lastRiderLineOf(t, a.caio); line.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED || line.GetNumbers() {
		t.Errorf("the monk's player reads %v, want the failure with no number", line)
	}
}

// A dragon with no use left is never held: the save is rolled and applied at once, as for any creature.
func TestHitRiderResistance_NoUseLeftIsNeverHeld(t *testing.T) {
	t.Parallel()
	a, e, _ := riderDragonFight(t, adultRedDragon)
	if _, err := bumpCreatureSpent(t.Context(), playdb.New(a.h.pool), a.id(t, "Draco"), spentLegendaryResistance, 3); err != nil {
		t.Fatalf("bumpCreatureSpent() error = %v", err)
	}
	a.h.roller.queue(1)
	after, err := a.useRider(t, a.caio, e, a.stunOffer(t), stunChoice)
	if err != nil {
		t.Fatalf("UseHitRider() error = %v", err)
	}
	if len(after.GetReactionWindows()) != 0 || a.windowOf(t, a.master, resistKind) != nil || a.get(t, a.ana).GetReactionWait() != nil {
		t.Errorf("the answer is held: windows %v", a.windows(t, a.master))
	}
	if !riderTargetHas(after, "Draco", stunnedCond) || a.kiUsed(t) != 1 || a.openRiders(t) != 0 {
		t.Errorf("stunned %v, ki used %d, open offers %d; want the stun at once, 1, 0", riderTargetHas(after, "Draco", stunnedCond), a.kiUsed(t), a.openRiders(t))
	}
}

// A target without the trait (the goblin) is never held, and neither is a decline.
func TestHitRiderResistance_ATargetWithoutTheTraitAndADeclineAreNeverHeld(t *testing.T) {
	t.Parallel()
	a, e, _ := riderDragonFight(t, adultRedDragon)
	// A decline on the dragon: no save, no hold.
	if _, err := a.useRider(t, a.caio, e, a.stunOffer(t), playv1.HitRiderChoice_HIT_RIDER_CHOICE_DECLINE); err != nil {
		t.Fatalf("UseHitRider(decline) error = %v", err)
	}
	if a.windowOf(t, a.master, resistKind) != nil || a.kiUsed(t) != 0 || a.openRiders(t) != 0 {
		t.Errorf("a decline: window %v, ki used %d, open offers %d; want none, 0, 0", a.windowOf(t, a.master, resistKind) != nil, a.kiUsed(t), a.openRiders(t))
	}
	// The next strike lands on the goblin.
	wantHit(t, "the Extra Attack", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	a.h.roller.queue(1)
	after, err := a.useRider(t, a.caio, e, a.stunOffer(t), stunChoice)
	if err != nil {
		t.Fatalf("UseHitRider(stun) error = %v", err)
	}
	if a.windowOf(t, a.master, resistKind) != nil || len(after.GetReactionWindows()) != 0 {
		t.Error("an answer against a goblin is held")
	}
	if !riderTargetHas(after, "Goblin", stunnedCond) || a.kiUsed(t) != 1 {
		t.Errorf("the goblin is stunned = %v, ki used %d; want the stun at once and 1", riderTargetHas(after, "Goblin", stunnedCond), a.kiUsed(t))
	}
}

// A rider with no save (no reactions) is never held.
func TestHitRiderResistance_ARiderWithNoSaveIsNeverHeld(t *testing.T) {
	t.Parallel()
	a, e, _ := riderDragonFight(t, adultRedDragon)
	wantHit(t, "the Extra Attack", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Draco", d20(15)), hit)
	if _, err := a.action(t, a.caio, e, "Toren", flurry); err != nil {
		t.Fatalf("Flurry of Blows error = %v", err)
	}
	wantHit(t, "a flurry strike", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Draco", d20(15)), hit)
	open := a.riderOffers(t)[playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND]
	if _, err := a.useRider(t, a.caio, e, open, playv1.HitRiderChoice_HIT_RIDER_CHOICE_NO_REACTIONS); err != nil {
		t.Fatalf("UseHitRider(no reactions) error = %v", err)
	}
	if a.windowOf(t, a.master, resistKind) != nil {
		t.Error("a rider with no save is held")
	}
	if line := a.lastRiderLine(t); line.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_APPLIED {
		t.Errorf("the line = %v, want APPLIED", line)
	}
}

// Two riders at once: the second answer is refused while the first waits; once the first is settled the second is
// held in its own window and spends its own use.
func TestHitRiderResistance_TwoRidersAreHeldOneAfterTheOther(t *testing.T) {
	t.Parallel()
	a, e, _ := riderDragonFight(t, adultRedDragon)
	wantHit(t, "the Extra Attack", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Draco", d20(15)), hit)
	if _, err := a.action(t, a.caio, e, "Toren", flurry); err != nil {
		t.Fatalf("Flurry of Blows error = %v", err)
	}
	wantHit(t, "a flurry strike", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Draco", d20(15)), hit)
	offers := a.riderOffers(t)
	stun, open := offers[playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE], offers[playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND]
	if stun == "" || open == "" {
		t.Fatalf("offers = %v, want a Stunning Strike and an Open Hand", offers)
	}
	a.h.roller.queue(1)
	if _, err := a.useRider(t, a.caio, e, stun, stunChoice); err != nil {
		t.Fatalf("UseHitRider(stun) error = %v", err)
	}
	_, err := a.useRider(t, a.caio, e, open, proneChoice)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_PENDING)
	if _, err := a.resistUse(t, a.master, e, a.windowOf(t, a.master, resistKind), "Draco"); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	a.h.roller.queue(1)
	if _, err := a.useRider(t, a.caio, e, open, proneChoice); err != nil { // a 1 and 7 against DC 16: a Dexterity save fails
		t.Fatalf("UseHitRider(prone) error = %v", err)
	}
	w := a.windowOf(t, a.master, resistKind)
	if w == nil || len(w.GetLegendaryResistance().GetOffers()) != 1 {
		t.Fatalf("the second rider's window = %v, want one with the failed save", w)
	}
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if got := a.resistSpentBy(t, "Draco"); got != 2 {
		t.Errorf("the dragon spent %d uses, want 2 (one for each rider)", got)
	}
	enc := a.get(t, a.master)
	if riderTargetHas(enc, "Draco", stunnedCond) || riderTargetHas(enc, "Draco", "condition:prone") {
		t.Error("the dragon is stunned or prone although both saves were made successes")
	}
}

// The master's answer is given once: the same window answered again is refused and the use is spent exactly once; the
// answering request sent again with its key answers as the first time and holds nothing a second time.
func TestHitRiderResistance_TheUseIsSpentOnceOnADuplicateAnswer(t *testing.T) {
	t.Parallel()
	a, e, _ := riderDragonFight(t, adultRedDragon)
	a.h.roller.queue(1, 20)
	first := &playv1.UseHitRiderRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), RiderId: a.stunOffer(t), Choice: stunChoice, IdempotencyKey: newKey()}
	if _, err := a.caio.combat.UseHitRider(t.Context(), connect.NewRequest(first)); err != nil {
		t.Fatalf("UseHitRider() error = %v", err)
	}
	holds := a.events(t)
	if _, err := a.caio.combat.UseHitRider(t.Context(), connect.NewRequest(first)); err != nil {
		t.Fatalf("UseHitRider() again error = %v", err)
	}
	if n := a.events(t); n != holds {
		t.Errorf("the repeated request wrote %d events, want none", n-holds)
	}
	w := a.windowOf(t, a.master, resistKind)
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err == nil {
		t.Error("the closed window was answered again")
	}
	if got := a.resistSpentBy(t, "Draco"); got != 1 {
		t.Errorf("the dragon spent %d uses, want 1", got)
	}
	if _, err := a.caio.combat.UseHitRider(t.Context(), connect.NewRequest(first)); err != nil {
		t.Fatalf("UseHitRider() after the replay error = %v", err)
	}
	if got, ki := a.resistSpentBy(t, "Draco"), a.kiUsed(t); got != 1 || ki != 1 {
		t.Errorf("after the repeated request: uses spent %d, ki used %d; want 1 and 1", got, ki)
	}
}

// The target leaves the combat while the answer waits: the window and the hold end alike whatever the save rolled,
// the ki is not spent and nothing is left of the rider.
func TestHitRiderResistance_ATargetThatLeavesEndsTheWindowWhateverItRolled(t *testing.T) {
	t.Parallel()
	for name, face := range map[string]int{"the save passed": 20, "the save failed": 1} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, e, _ := riderDragonFight(t, adultRedDragon)
			for _, p := range a.mustOptions(t, a.master, e, "Toren").GetPendingDamages() { // the hits' damage would keep the dragon in the combat
				if _, err := a.actionsRestDiscard(t, a.master, e, p.GetId(), nil); err != nil {
					t.Fatalf("DiscardPendingDamage() error = %v", err)
				}
			}
			a.stunHeld(t, e, face, 20)
			if window, wait := a.holdState(t); !window || wait != "Esperando o mestre" {
				t.Fatalf("while it waits: window %v, wait %q; want the window and the wait", window, wait)
			}
			a.removeReactionCombatant(t, e, "Draco")
			if window, wait := a.holdState(t); window || wait != "" {
				t.Errorf("after the removal: window %v, wait %q; want none of them", window, wait)
			}
			if a.kiUsed(t) != 0 {
				t.Errorf("ki used = %d, want 0: the held rider spent nothing", a.kiUsed(t))
			}
		})
	}
}

// The action that follows the held answer is refused while it waits, as for a held cast.
func TestHitRiderResistance_NothingElseGoesThroughWhileItWaits(t *testing.T) {
	t.Parallel()
	a, e, _ := riderDragonFight(t, adultRedDragon)
	a.stunHeld(t, e, 20, 20)
	if _, err := a.attack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)); err == nil {
		t.Error("an attack went through while the rider waits")
	}
}

// creatureRiderFight is a combat where a pit fiend (the master's creature, on turn) bites an adult red dragon that
// fights on the party's side: the bite asks the dragon's Constitution save against DC 21 (a 7 or less fails), the
// rider of an NPC attacker that the master answers himself. The offers are the master's: the damage of the bite is rolled,
// and the rider returned.
func creatureRiderFight(t *testing.T) (*armed, *playv1.Encounter, string) {
	t.Helper()
	a := newCastingParty(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{1},
		players: map[string]int32{"Toren": 10, "Pensantus": 5, "Brisa": 1},
		reveal:  []string{"Goblin"}, setup: true,
	})
	a.h.roller.queue(1)
	a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Name, r.Hidden = adultRedDragon, 1, "Draco", new(false)
	})
	a.h.roller.queue(30)
	res := a.mustAddMonsters(t, a.get(t, a.master), func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Name, r.Hidden = "monster:pit-fiend", 1, "Belial", new(false)
	})
	if len(res.GetCombatantIds()) != 1 {
		t.Fatalf("added %d pit fiends, want 1", len(res.GetCombatantIds()))
	}
	e = a.begin(t, a.get(t, a.master))
	if _, err := a.master.combat.SetCombatantSide(t.Context(), connect.NewRequest(&playv1.SetCombatantSideRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Draco"), IdempotencyKey: newKey(), Side: playv1.CombatantSide_COMBATANT_SIDE_PARTY,
	})); err != nil {
		t.Fatalf("SetCombatantSide() error = %v", err)
	}
	a.castStand(t, map[string][2]int32{"Belial": {10, 5}, "Draco": {9, 5}, "Toren": {8, 8}, "Goblin": {7, 8}, "Pensantus": {1, 9}, "Brisa": {1, 1}})
	e = a.passTo(t, e, "Belial")
	return a, e, "Belial"
}

// openCreatureRider has the pit fiend bite the dragon and applies the damage: the master's offer is the result.
func (a *armed) openCreatureRider(t *testing.T, e *playv1.Encounter) string {
	t.Helper()
	res := a.mustAttack(t, a.master, e, "Belial", "basic:0", "Draco", d20(19))
	id := res.GetPendingDamage().GetId()
	a.mustDamage(t, a.master, e, id, typedDamage(5)) // an NPC's hit points are taken at once
	offers := ridersOfKind(a.get(t, a.master), playv1.HitRiderKind_HIT_RIDER_KIND_CREATURE)
	if len(offers) != 1 {
		t.Fatalf("creature offers = %v, want the bite's rider", a.get(t, a.master).GetHitRiders())
	}
	return offers[0].GetId()
}

// The master answers the rider of an NPC's bite against a dragon that fights on the party's side: the same hold applies
// (one path). The 3 the dragon rolls (16 against DC 21) fails; USE makes it a success with the very same d20, the
// condition is not given, one use is spent and the master reads the roll that failed.
func TestHitRiderResistance_ACreatureRiderAgainstALegendaryTargetIsHeldToo(t *testing.T) {
	t.Parallel()
	a, e, _ := creatureRiderFight(t)
	rider := a.openCreatureRider(t, e)
	a.h.roller.queue(3, 20)
	if _, err := a.useRider(t, a.master, e, rider, playv1.HitRiderChoice_HIT_RIDER_CHOICE_APPLY); err != nil {
		t.Fatalf("UseHitRider(apply) error = %v", err)
	}
	asked := a.h.roller.taken()
	w := a.windowOf(t, a.master, resistKind)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the Legendary Resistance window", a.windows(t, a.master))
	}
	if offers := w.GetLegendaryResistance().GetOffers(); len(offers) != 1 || offers[0].GetD20() != 3 || offers[0].GetDc() != 21 {
		t.Errorf("the offers = %v, want Draco's 3 against 21", offers)
	}
	if len(ridersOfKind(a.get(t, a.master), playv1.HitRiderKind_HIT_RIDER_KIND_CREATURE)) != 1 {
		t.Error("the offer closed before the replay")
	}
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if again := a.h.roller.taken(); again != asked {
		t.Errorf("the replay rolled %d dice more, want none", again-asked)
	}
	if a.resistSpentBy(t, "Draco") != 1 {
		t.Errorf("the dragon spent %d uses, want 1", a.resistSpentBy(t, "Draco"))
	}
	line := a.lastRiderLine(t)
	if line.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_SAVED || !line.GetResisted() || line.GetResistedLeft() != 2 || line.GetD20() != 3 || line.GetConditionKey() != "" {
		t.Errorf("the master's line = %v, want the failed 3 made a success with 2 uses left and no condition", line)
	}
	if len(ridersOfKind(a.get(t, a.master), playv1.HitRiderKind_HIT_RIDER_KIND_CREATURE)) != 0 {
		t.Error("the answered offer is still open")
	}
	for _, u := range []*user{a.caio, a.ana, a.bia} {
		if a.windowOf(t, u, resistKind) != nil {
			t.Error("a player reads the window")
		}
	}
}

// While an answer to a rider waits, every answer to a rider offer of the combat is refused the same way, whatever the
// offer, the choice or the target (REACTION_PENDING): a decline or another choice on the offer that waits, and an
// offer on a goblin. The hold stands and the master's answer still replays it with the kept d20.
func TestHitRiderResistance_NoRiderIsAnsweredWhileOneWaits(t *testing.T) {
	t.Parallel()
	a, e, _ := riderDragonFight(t, adultRedDragon)
	wantHit(t, "the Extra Attack", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	offers := ridersOfKind(a.get(t, a.caio), playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE)
	if len(offers) != 2 {
		t.Fatalf("stun offers = %v, want one on the dragon and one on the goblin", offers)
	}
	dragonOffer, goblinOffer := offers[0].GetId(), offers[1].GetId()
	if offers[0].GetTargetId() != a.id(t, "Draco") {
		dragonOffer, goblinOffer = goblinOffer, dragonOffer
	}
	a.h.roller.queue(1)
	if _, err := a.useRider(t, a.caio, e, dragonOffer, stunChoice); err != nil {
		t.Fatalf("UseHitRider() error = %v", err)
	}
	blocked := playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_PENDING
	for name, try := range map[string]struct {
		offer  string
		choice playv1.HitRiderChoice
	}{
		"the same offer declined":     {dragonOffer, playv1.HitRiderChoice_HIT_RIDER_CHOICE_DECLINE},
		"the same offer, again":       {dragonOffer, stunChoice},
		"the goblin's offer":          {goblinOffer, stunChoice},
		"the goblin's offer declined": {goblinOffer, playv1.HitRiderChoice_HIT_RIDER_CHOICE_DECLINE},
	} {
		_, err := a.useRider(t, a.caio, e, try.offer, try.choice)
		if err == nil {
			t.Errorf("%s: answered while the first waits", name)
			continue
		}
		wantEncounterBlocked(t, err, blocked)
	}
	if a.windowOf(t, a.master, resistKind) == nil || a.kiUsed(t) != 0 {
		t.Fatal("a refused answer touched the hold")
	}
	asked := a.h.roller.taken()
	if _, err := a.resistUse(t, a.master, e, a.windowOf(t, a.master, resistKind)); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if again := a.h.roller.taken(); again != asked {
		t.Errorf("the replay rolled %d dice more, want none", again-asked)
	}
	if !riderTargetHas(a.get(t, a.master), "Draco", stunnedCond) || a.kiUsed(t) != 1 {
		t.Errorf("after the replay: stunned %v, ki used %d; want the kept failure to land and 1", riderTargetHas(a.get(t, a.master), "Draco", stunnedCond), a.kiUsed(t))
	}
	// Once settled, the goblin's offer is answered as before.
	a.h.roller.queue(1)
	if _, err := a.useRider(t, a.caio, e, goblinOffer, stunChoice); err != nil {
		t.Errorf("the goblin's offer after the replay: %v", err)
	}
}

// The Open Hand push of a legendary creature whose failed save the master lets stand moves it when the answer is
// replayed, and the replay tells the streams: the master hears the creature move.
func TestHitRiderResistance_TheReplayOfAPushPublishesTheMove(t *testing.T) {
	t.Parallel()
	a, e, _ := riderDragonFight(t, adultRedDragon)
	wantHit(t, "the Extra Attack", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Draco", d20(15)), hit)
	if _, err := a.action(t, a.caio, e, "Toren", flurry); err != nil {
		t.Fatalf("Flurry of Blows error = %v", err)
	}
	wantHit(t, "a flurry strike", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Draco", d20(15)), hit)
	open := a.riderOffers(t)[playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND]
	a.h.roller.queue(1) // Strength +8 against DC 16
	if _, err := a.useRider(t, a.caio, e, open, playv1.HitRiderChoice_HIT_RIDER_CHOICE_PUSH); err != nil {
		t.Fatalf("UseHitRider(push) error = %v", err)
	}
	before := byLabel(t, a.get(t, a.master), "Draco")
	w := a.master.watch(t, a.campaignID)
	if _, err := a.resistUse(t, a.master, e, a.windowOf(t, a.master, resistKind)); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	a.master.markCurrentMap(t, a.campaignID, a.mapID, w)
	moved := false
	for _, ev := range w.beforeMarker(t) {
		moved = moved || ev.GetCombatantMoved().GetCombatantId() == before.GetId()
	}
	after := byLabel(t, a.get(t, a.master), "Draco")
	if after.GetCol() == before.GetCol() && after.GetRow() == before.GetRow() {
		t.Fatalf("the dragon stays at %d,%d, want it pushed", after.GetCol(), after.GetRow())
	}
	if !moved {
		t.Error("the replay of the push published no combatant_moved to the master")
	}
}
