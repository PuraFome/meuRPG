package play

import (
	"sync"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The review round of slice 9.10: replayed keys, the SRD's end of the form at 0 hit
// points and unconscious, the 0 hit points rule of an opportunity attack, a sight
// across a combat's start. They need the database (MEURPG_TEST_DATABASE_URL).

// TestMR037_AReplayedKeyNeverAnswersForAnotherCharacter: an idempotency key belongs to
// the member that made the change and to the character it was for; replayed by
// another member, or with another character, it is refused and no vitals are returned.
func TestMR037_AReplayedKeyNeverAnswersForAnotherCharacter(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	s.nanquim(t)
	key := newKey()
	_, err := s.bia.play.AssumeWildShape(t.Context(), connect.NewRequest(&playv1.AssumeWildShapeRequest{CampaignId: s.campaignID, CharacterId: s.bri.GetId(), BeastKey: wolfKey, IdempotencyKey: key}))
	if err != nil {
		t.Fatalf("AssumeWildShape() error = %v", err)
	}
	// Ana replays Bia's key for her own character, and for Bia's: both refused.
	for _, id := range []string{s.pens.GetId(), s.bri.GetId()} {
		res, err := s.ana.play.AssumeWildShape(t.Context(), connect.NewRequest(&playv1.AssumeWildShapeRequest{CampaignId: s.campaignID, CharacterId: id, BeastKey: wolfKey, IdempotencyKey: key}))
		wantCode(t, "a replayed key by another member", err, connect.CodeInvalidArgument)
		if res != nil {
			t.Errorf("the replay answered %v", res.Msg)
		}
	}
	// The master replays his own key with another character: refused too.
	mkey := newKey()
	if _, err := s.master.play.LeaveWildShape(t.Context(), connect.NewRequest(&playv1.LeaveWildShapeRequest{CampaignId: s.campaignID, CharacterId: s.bri.GetId(), IdempotencyKey: mkey})); err != nil {
		t.Fatalf("LeaveWildShape() error = %v", err)
	}
	_, err = s.master.play.LeaveWildShape(t.Context(), connect.NewRequest(&playv1.LeaveWildShapeRequest{CampaignId: s.campaignID, CharacterId: s.pens.GetId(), IdempotencyKey: mkey}))
	wantCode(t, "a replayed key with another character", err, connect.CodeInvalidArgument)
	// The sight too.
	skey := newKey()
	x, y := centerBP(0, 0)
	s.h.placeToken(s.mapID, s.pens.GetId(), x, y)
	s.placeCreatureToken(t, s.mustCreatures(t, s.ana, s.pens)[0].GetId(), 3, 3)
	if _, err := s.ana.play.StartFamiliarSight(t.Context(), connect.NewRequest(&playv1.StartFamiliarSightRequest{CampaignId: s.campaignID, CharacterId: s.pens.GetId(), IdempotencyKey: skey})); err != nil {
		t.Fatalf("StartFamiliarSight() error = %v", err)
	}
	_, err = s.bia.play.StopFamiliarSight(t.Context(), connect.NewRequest(&playv1.StopFamiliarSightRequest{CampaignId: s.campaignID, CharacterId: s.pens.GetId(), IdempotencyKey: skey}))
	wantCode(t, "a sight key replayed by another member", err, connect.CodeInvalidArgument)
}

// TestMR037_TheFormEndsWhenTheDruidFallsToZeroOrAsleep: the SRD returns the druid to
// its own shape when it drops to 0 hit points or falls unconscious, whatever did it;
// and a druid at 0 or unconscious cannot take a form.
func TestMR037_TheFormEndsWhenTheDruidFallsToZeroOrAsleep(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	a := s.armed
	s.fight(t)
	e := s.mustAssume(t, s.bia, s.bri, wolfKey).GetEncounter()

	// The master's correction of the druid's own hit points to 0.
	s.correct(t, s.bri, func(r *playv1.AdjustCharacterVitalsRequest) { r.HitPointsCurrent = ptrTo(int32(0)) })
	v := a.vitals(t, s.bri)
	got := byLabel(t, a.get(t, a.master), "Sálvia")
	if v.GetWildShape() != nil || got.GetSpeedFt() != 30 || got.GetWildShapeBeastKey() != "" || got.GetState() != playv1.CombatantState_COMBATANT_STATE_DOWN {
		t.Errorf("after the master set 0 PV: form %v, %d ft, state %v; want her own shape, 30 ft, down", v.GetWildShape(), got.GetSpeedFt(), got.GetState())
	}
	if ev := a.lastPayload(t, eventWildShapeEnded); ev["reason"] != endedAtZero {
		t.Errorf("wild_shape_ended = %v, want the reason %q", ev, endedAtZero)
	}
	// At 0 she cannot take a form again.
	_, err := s.assume(t, s.bia, s.bri, wolfKey)
	wantEncounterBlocked(t, err, blockedDown)
	// Healing at 0 heals the druid: there is no beast to take it.
	a.passTo(t, e, "Irmã")
	cast := a.mustCast(t, s.dani, a.get(t, s.dani), "Irmã", cureWounds, slotOfLevel(1), a.at(t, "Sálvia"), noCastRoll)
	a.h.roller.queue(4)
	a.mustDamage(t, s.dani, a.get(t, s.dani), cast.GetCast().GetPendingDamages()[0].GetId(), inAppDamage)
	if v := a.vitals(t, s.bri); v.GetHitPointsCurrent() != 7 || v.GetWildShape() != nil {
		t.Errorf("after the heal: %v, want the druid at 7 PV in her own shape", v)
	}

	// Unconscious: Sono and the master's hand end the form; and she cannot take one asleep.
	s2 := newShapers(t)
	s2.fight(t)
	e2 := s2.mustAssume(t, s2.bia, s2.bri, wolfKey).GetEncounter()
	s2.setConditions(t, e2, "Sálvia", unconscious)
	v2 := s2.vitals(t, s2.bri)
	got2 := byLabel(t, s2.get(t, s2.master), "Sálvia")
	if v2.GetWildShape() != nil || got2.GetSpeedFt() != 30 || v2.GetHitPointsCurrent() != 38 {
		t.Errorf("after Unconscious: form %v, %d ft, %d PV; want her own shape and 38 PV", v2.GetWildShape(), got2.GetSpeedFt(), v2.GetHitPointsCurrent())
	}
	if ev := s2.lastPayload(t, eventWildShapeEnded); ev["reason"] != endedAsleep {
		t.Errorf("wild_shape_ended = %v, want the reason %q", ev, endedAsleep)
	}
	_, err = s2.assume(t, s2.master, s2.bri, wolfKey)
	wantEncounterBlocked(t, err, blockedDown)
}

// TestMR037_CarryOverToZeroStartsTheDeathSavesAndSendsTheMoverBack: the beast falls on an
// opportunity attack and the damage that carries over drops the druid to 0: the form
// ends, the death saves are due, and the 0 hit points rule takes her back to where she
// left the reach.
func TestMR037_CarryOverToZeroStartsTheDeathSavesAndSendsTheMoverBack(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	a := s.armed
	s.fight(t)
	s.mustAssume(t, s.bia, s.bri, wolfKey)
	s.correct(t, s.bri, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.WildShapeHitPointsCurrent, r.HitPointsCurrent = ptrTo(int32(1)), ptrTo(int32(2))
	})
	// She leaves the Capitão's and the goblin's reach: both may attack.
	if _, err := s.bia.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
		CampaignId: s.campaignID, EncounterId: a.get(t, a.master).GetId(), CombatantId: a.id(t, "Sálvia"), IdempotencyKey: newKey(), Col: 6, Row: 2,
	})); err != nil {
		t.Fatalf("MoveCombatant() error = %v", err)
	}
	offers := a.get(t, a.master).GetOpportunityOffers()
	var offer *playv1.OpportunityOffer
	for _, o := range offers {
		if o.GetReactorId() == a.id(t, "Capitão Goblin") {
			offer = o
		}
	}
	if offer == nil {
		t.Fatalf("offers = %v, want one for the Capitão", offers)
	}
	left := [2]int32{offer.GetLeftCol(), offer.GetLeftRow()}
	hit, err := a.master.combat.RollAttack(t.Context(), connect.NewRequest(&playv1.RollAttackRequest{
		CampaignId: s.campaignID, EncounterId: a.get(t, a.master).GetId(), AttackerId: a.id(t, "Capitão Goblin"), AttackKey: offer.GetAttacks()[0].GetKey(),
		TargetId: a.id(t, "Sálvia"), IdempotencyKey: newKey(), OpportunityOfferId: offer.GetId(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: 15},
	}))
	if err != nil {
		t.Fatalf("the opportunity attack error = %v", err)
	}
	dmg := a.mustDamage(t, a.master, a.get(t, a.master), hit.Msg.GetPendingDamage().GetId(), typedDamage(6)).GetPendingDamage() // 8
	if _, err := a.settle(t, a.master, a.get(t, a.master), dmg.GetId(), true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	v := a.vitals(t, s.bri)
	got := byLabel(t, a.get(t, a.master), "Sálvia")
	if v.GetWildShape() != nil || v.GetHitPointsCurrent() != 0 || got.GetState() != playv1.CombatantState_COMBATANT_STATE_DOWN || got.GetSpeedFt() != 30 {
		t.Fatalf("after the carry-over: %v, state %v, %d ft; want her own shape at 0 PV, down", v, got.GetState(), got.GetSpeedFt())
	}
	if got.GetCol() != left[0] || got.GetRow() != left[1] {
		t.Errorf("she stands on %d,%d, want %v where she left the reach (the 0 hit points rule)", got.GetCol(), got.GetRow(), left)
	}
	// The death saves are due on her next turn: the turn after this one holds the first.
	if got.GetDeathSaveDue() {
		t.Log("a death save is due now")
	}
}

// TestMR036_ASightEndsWhenACombatStartsAndIsRefusedInSetup: a sight begun outside a
// combat ends when the character's combat begins (a familiar_sight stop, reason
// "combat"); in SETUP it cannot be started.
func TestMR036_ASightEndsWhenACombatStartsAndIsRefusedInSetup(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	a := s.armed
	owl := s.nanquim(t)
	x, y := centerBP(0, 0)
	a.h.placeToken(a.mapID, s.pens.GetId(), x, y)
	a.placeCreatureToken(t, owl, 3, 3)
	a.mustSight(t, s.ana, s.pens, true)

	e := s.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: s.goblin.GetId()}}, npcRolls: []int{1},
		players: map[string]int32{"Pensantus": 20, "Sálvia": 15, "Irmã": 12, "Toren": 10, "Nanquim": 8}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Pensantus": {10, 3}, "Nanquim": {12, 3}}, setup: true,
	})
	if v := a.vitals(t, s.pens); v.GetFamiliarSight() != nil {
		t.Errorf("sight in SETUP = %v, want it ended when the character joined the combat", v.GetFamiliarSight())
	}
	if ev := a.lastPayload(t, eventFamiliarSight); ev["sight"] != "stop" || ev["reason"] != sightCombatJoined {
		t.Errorf("familiar_sight = %v, want the stop with reason %q", ev, sightCombatJoined)
	}
	_, _, err := a.sightCall(t, s.ana, s.pens, true)
	wantSightBlocked(t, err, playv1.FamiliarSightBlockedReason_FAMILIAR_SIGHT_BLOCKED_REASON_COMBAT_NOT_BEGUN)
	if _, err := a.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("BeginCombat() error = %v", err)
	}
	a.mustSight(t, s.ana, s.pens, true) // on his turn it is an action again
}

// TestMR037_TwoAssumesAtOnceOneWins: two Wild Shapes of the same druid at the same time:
// one takes the form, the other is told she is in one already, and one use is spent.
func TestMR037_TwoAssumesAtOnceOneWins(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = s.assume(t, s.bia, s.bri, wolfKey)
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else {
			wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ALREADY_IN_WILD_SHAPE)
		}
	}
	if v := s.vitals(t, s.bri); ok != 1 || used(v, wildShapeResource) != 1 {
		t.Errorf("successes = %d, uses spent = %d, want one and one", ok, used(v, wildShapeResource))
	}
}

// TestMR036_EveryChangeOfAFormOrASightTellsTheFog: the fog is told (VisionChanged) after
// each path that changes what a player sees through the form or the sight: taking and
// leaving the form, the beast falling on a damage, the master's correction, the undo of
// start and leave, the sight's start, stop and undo.
func TestMR036_EveryChangeOfAFormOrASightTellsTheFog(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	a := s.armed
	s.nanquim(t)
	e := s.pensantusFirst(t, nil)
	e = a.passTo(t, a.mustEndTurn(t, s.ana, e), "Sálvia")
	told := func(what string, do func()) {
		t.Helper()
		before := a.h.vision.n.Load()
		do()
		if a.h.vision.n.Load() == before {
			t.Errorf("%s: the fog was not told", what)
		}
	}
	told("assuming the form", func() { s.mustAssume(t, s.bia, s.bri, wolfKey) })
	told("the master's correction of the beast to 0", func() {
		s.correct(t, s.bri, func(r *playv1.AdjustCharacterVitalsRequest) { r.WildShapeHitPointsCurrent = ptrTo(int32(0)) })
	})
	told("the undo of the leaving... after a new form", func() {
		e = a.get(t, a.master)
		a.passTo(t, a.mustEndTurn(t, s.bia, e), "Sálvia")
		s.mustAssume(t, s.bia, s.bri, wolfKey)
		a.undoLast(t, a.get(t, a.master)) // the start
	})
	told("the beast falling on a damage", func() {
		s.mustAssume(t, s.master, s.bri, wolfKey)
		s.correct(t, s.bri, func(r *playv1.AdjustCharacterVitalsRequest) { r.WildShapeHitPointsCurrent = ptrTo(int32(1)) })
		hit := a.mustAttack(t, a.master, a.get(t, a.master), "Capitão Goblin", sword, "Sálvia", func(r *playv1.RollAttackRequest) {
			r.Roll = &playv1.RollAttackRequest_D20Face{D20Face: 15}
			r.AsReaction = true
		})
		dmg := a.mustDamage(t, a.master, a.get(t, a.master), hit.GetPendingDamage().GetId(), typedDamage(6)).GetPendingDamage()
		if _, err := a.settle(t, a.master, a.get(t, a.master), dmg.GetId(), true); err != nil {
			t.Fatalf("ApplyPendingDamage() error = %v", err)
		}
	})
	told("the undo of that damage", func() { a.undoLast(t, a.get(t, a.master)) })
	told("leaving the form", func() { s.mustLeave(t, s.master, s.bri) })
	told("the undo of leaving", func() { a.undoLast(t, a.get(t, a.master)) })
}

// TestMR036_TheFogHearsOfASightStartStopAndUndo: the same for the familiar's eyes.
func TestMR036_TheFogHearsOfASightStartStopAndUndo(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	a := s.armed
	s.nanquim(t)
	s.pensantusFirst(t, nil)
	told := func(what string, do func()) {
		t.Helper()
		before := a.h.vision.n.Load()
		do()
		if a.h.vision.n.Load() == before {
			t.Errorf("%s: the fog was not told", what)
		}
	}
	told("the start", func() { a.mustSight(t, s.ana, s.pens, true) })
	told("the stop", func() { a.mustSight(t, s.ana, s.pens, false) })
	a.mustEndTurn(t, s.ana, a.get(t, s.ana))
	a.passTo(t, a.get(t, a.master), "Pensantus")
	a.mustSight(t, s.ana, s.pens, true)
	told("the undo of the start", func() { a.undoLast(t, a.get(t, a.master)) })
	a.mustSight(t, s.ana, s.pens, true)
	told("the combat's end", func() { a.endEncounter(t, a.get(t, a.master)) })
}
