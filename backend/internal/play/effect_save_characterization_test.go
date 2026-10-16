package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Characterization of RollEffectSave and ResolveConcentrationSave: every refusal with its code and
// message, who may answer, and what each answer leaves behind. Frozen before the refactor of the
// two handlers, so the phases they were split into keep the first error and the writes.

// wantEffectSaveRefusal checks a refusal's code and the text of its message.
func wantEffectSaveRefusal(t *testing.T, call string, err error, code connect.Code, text string) {
	t.Helper()
	wantCode(t, call, err, code)
	if !strings.Contains(err.Error(), text) {
		t.Errorf("%s error = %q, want the message to say %q", call, err.Error(), text)
	}
}

// effectSaveWindow has Hold Person on Toren (Caio's) and his turn end, which opens the save, and
// returns the open window.
func effectSaveWindow(t *testing.T) (*armed, *playv1.Encounter, *playv1.ReactionWindow) {
	t.Helper()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, holdPerson, []string{"Toren"}, a.rounds(t, 10, "Toren"))
	a.passTo(t, e, "Toren")
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := a.windowOf(t, a.caio, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("Toren's windows = %v, want the save of Hold Person", a.windows(t, a.caio))
	}
	return a, e, w
}

func TestRollEffectSaveRefusesABadAnswerBeforeLookingAtTheWindow(t *testing.T) {
	t.Parallel()
	a, e, w := effectSaveWindow(t)
	cases := []struct {
		name string
		set  func(*playv1.RollEffectSaveRequest)
		want string
	}{
		{"roll_in_app false", func(r *playv1.RollEffectSaveRequest) {
			r.Roll = &playv1.RollEffectSaveRequest_RollInApp{RollInApp: false}
		}, "roll_in_app must be true"},
		{"d20 below 1", effectSaveFace(0), "d20_face must be 1 to 20"},
		{"d20 above 20", effectSaveFace(21), "d20_face must be 1 to 20"},
		{"delegate false", func(r *playv1.RollEffectSaveRequest) {
			r.Roll = &playv1.RollEffectSaveRequest_DelegateToMaster{DelegateToMaster: false}
		}, "delegate_to_master must be true"},
		{"skip false", func(r *playv1.RollEffectSaveRequest) {
			r.Roll = &playv1.RollEffectSaveRequest_Skip{Skip: false}
		}, "skip must be true"},
		{"nothing set", func(*playv1.RollEffectSaveRequest) {}, "set roll_in_app, d20_face, delegate_to_master or skip"},
	}
	for _, tc := range cases {
		_, err := a.rollEffectSave(t, a.caio, e, w.GetId(), tc.set)
		wantEffectSaveRefusal(t, tc.name, err, connect.CodeInvalidArgument, tc.want)
	}
	// The window id is read before the answer, and a malformed one reads as a window that is not there.
	_, err := a.rollEffectSave(t, a.caio, e, "not-a-uuid", effectSaveFace(0))
	wantEffectSaveRefusal(t, "bad answer and bad window", err, connect.CodeNotFound, "window not found")
	_, err = a.rollEffectSave(t, a.caio, e, "not-a-uuid", effectSaveFace(5))
	wantEffectSaveRefusal(t, "a malformed window id", err, connect.CodeNotFound, "window not found")
}

func TestRollEffectSaveWhoMayAnswer(t *testing.T) {
	t.Parallel()
	a, e, w := effectSaveWindow(t)
	const denied = "you cannot answer this reaction"
	skip := func(r *playv1.RollEffectSaveRequest) { r.Roll = &playv1.RollEffectSaveRequest_Skip{Skip: true} }
	delegate := func(r *playv1.RollEffectSaveRequest) {
		r.Roll = &playv1.RollEffectSaveRequest_DelegateToMaster{DelegateToMaster: true}
	}
	// Another player's character: the window is not theirs.
	_, err := a.rollEffectSave(t, a.ana, e, w.GetId(), effectSaveFace(10))
	wantEffectSaveRefusal(t, "another player rolls", err, connect.CodePermissionDenied, denied)
	// A window that does not exist reads the same to a player, and "not found" to the master.
	_, err = a.rollEffectSave(t, a.caio, e, "00000000-0000-4000-8000-000000000001", effectSaveFace(10))
	wantEffectSaveRefusal(t, "a player asks for an unknown window", err, connect.CodePermissionDenied, denied)
	_, err = a.rollEffectSave(t, a.master, e, "00000000-0000-4000-8000-000000000001", effectSaveFace(10))
	wantEffectSaveRefusal(t, "the master asks for an unknown window", err, connect.CodeNotFound, "reaction window not found")
	// A window of the other kind is for AnswerReaction or the other handler, the same way.
	_, err = a.master.combat.ResolveConcentrationSave(t.Context(), connect.NewRequest(&playv1.ResolveConcentrationSaveRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), WindowId: w.GetId(), IdempotencyKey: newKey(),
		Roll: &playv1.ResolveConcentrationSaveRequest_Keep{Keep: true},
	}))
	wantEffectSaveRefusal(t, "an effect's window answered as a concentration save", err, connect.CodeInvalidArgument, "this window is not a concentration save: use AnswerReaction")
	// Skipping the save is the master's option.
	_, err = a.rollEffectSave(t, a.caio, e, w.GetId(), skip)
	wantEffectSaveRefusal(t, "a player skips", err, connect.CodePermissionDenied, denied)
	// The master rolls it himself: "leave it to the master" is for the player.
	_, err = a.rollEffectSave(t, a.master, e, w.GetId(), delegate)
	wantEffectSaveRefusal(t, "the master hands it to himself", err, connect.CodeInvalidArgument, "the master rolls it: use roll_in_app or d20_face")
	// Left to the master, the player can no longer roll it, nor skip it; the master can.
	res, err := a.rollEffectSave(t, a.caio, e, w.GetId(), delegate)
	// Finding: handing the save over answers with an empty result (savedResultOf of an event nobody
	// wrote), not with no result; the web reads Saved=false from it. lasting_effects_save.go:239.
	if err != nil || res.GetResult() == nil || res.GetResult().GetD20() != 0 || res.GetResult().GetSaved() || res.GetInspirationOffer() != nil || res.GetLuckyOffer() != nil {
		t.Fatalf("the player hands the save over = %v, %v; want an answer with an empty result", res, err)
	}
	_, err = a.rollEffectSave(t, a.caio, e, w.GetId(), effectSaveFace(10))
	wantEffectSaveRefusal(t, "the player rolls a handed save", err, connect.CodePermissionDenied, denied)
	if a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE) == nil {
		t.Fatal("the handed save is not open for the master")
	}
	res, err = a.rollEffectSave(t, a.master, e, w.GetId(), effectSaveFace(2))
	if err != nil {
		t.Fatalf("the master rolls the handed save: %v", err)
	}
	if r := res.GetResult(); r.GetSaved() || r.GetD20() != 2 || r.GetDc() == 0 {
		t.Errorf("the master's result = %v, want a failed 2 with the DC shown to the master", r)
	}
}

func TestRollEffectSaveResultByViewer(t *testing.T) {
	t.Parallel()
	a, e, w := effectSaveWindow(t)
	// The player's typed d20: the result carries the roll but never the DC; a pass ends the hold.
	res, err := a.rollEffectSave(t, a.caio, e, w.GetId(), effectSaveFace(20))
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	r := res.GetResult()
	if !r.GetSaved() || !r.GetEffectEnded() || r.Dc != nil || r.GetD20() != 20 || r.GetPhysical() != true || r.GetTextPt() != "Imobilizar Pessoa acabou." {
		t.Errorf("the player's result = %v, want a typed 20 that saved, ended the effect and hid the DC", r)
	}
	if len(r.GetD20Faces()) != 1 || r.GetModifier() != r.GetTotal()-20 {
		t.Errorf("the player's result faces/modifier = %v, want one face and total = 20 + modifier", r)
	}
	// The window is answered: a new key is refused.
	req := &playv1.RollEffectSaveRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), WindowId: w.GetId(), IdempotencyKey: newKey(), Roll: &playv1.RollEffectSaveRequest_D20Face{D20Face: 20}}
	if _, err := a.caio.lasting.RollEffectSave(t.Context(), connect.NewRequest(req)); err == nil {
		t.Error("the window is answered: a new key must be refused")
	}
}

func TestRollEffectSaveSkippedAndFailed(t *testing.T) {
	t.Parallel()
	a, e, w := effectSaveWindow(t)
	// The master skips: nothing is rolled, the effect stays, and the master reads the DC.
	res, err := a.rollEffectSave(t, a.master, e, w.GetId(), func(r *playv1.RollEffectSaveRequest) {
		r.Roll = &playv1.RollEffectSaveRequest_Skip{Skip: true}
	})
	if err != nil {
		t.Fatalf("RollEffectSave(skip) error = %v", err)
	}
	if r := res.GetResult(); !r.GetSkipped() || r.GetSaved() || r.GetD20() != 0 || r.GetDc() == 0 || r.GetEffectEnded() {
		t.Errorf("the skipped result = %v, want skipped with the DC and no roll", r)
	}
	if cardOf(t, byLabel(t, a.get(t, a.master), "Toren"), holdPerson) == nil {
		t.Error("a skipped save ended the effect")
	}
	if a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE) != nil {
		t.Error("the window is still open after the skip")
	}
	// The log says it was skipped, to a player too.
	var skipped bool
	for _, l := range logEntries(a.log(t, a.caio, e)) {
		skipped = skipped || l.GetEffect().GetChange() == playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVE_SKIPPED
	}
	if !skipped {
		t.Error("Toren's player's log has no line for the skipped save")
	}
}

func TestRollEffectSaveInAppWithTheMastersDiceAndAFailure(t *testing.T) {
	t.Parallel()
	a, e, w := effectSaveWindow(t)
	a.h.roller.queue(3)
	res, err := a.rollEffectSave(t, a.master, e, w.GetId(), func(r *playv1.RollEffectSaveRequest) {
		r.Roll = &playv1.RollEffectSaveRequest_RollInApp{RollInApp: true}
	})
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if r := res.GetResult(); r.GetSaved() || r.GetPhysical() || r.GetD20() != 3 || r.GetEffectEnded() || r.GetDc() == 0 {
		t.Errorf("the result = %v, want a failed in-app 3 that keeps the hold", r)
	}
	if cardOf(t, byLabel(t, a.get(t, a.master), "Toren"), holdPerson) == nil {
		t.Error("a failed save ended Hold Person")
	}
}

// concentrationSaveRefusal checks a refusal of ResolveConcentrationSave.
func (a *armed) concentrationSaveFor(t *testing.T, u *user, e *playv1.Encounter, windowID string, set func(*playv1.ResolveConcentrationSaveRequest)) error {
	t.Helper()
	_, err := a.resolveConcentration(t, u, e, windowID, set)
	return err
}

func TestResolveConcentrationSaveRefusesABadAnswerBeforeLookingAtTheWindow(t *testing.T) {
	t.Parallel()
	a, e := concentratingHit(t, 6)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want a concentration save", a.windows(t, a.ana))
	}
	cases := []struct {
		name string
		set  func(*playv1.ResolveConcentrationSaveRequest)
		want string
	}{
		{"roll_in_app false", func(r *playv1.ResolveConcentrationSaveRequest) {
			r.Roll = &playv1.ResolveConcentrationSaveRequest_RollInApp{RollInApp: false}
		}, "roll_in_app must be true"},
		{"d20 below 1", concentrationFace(0), "d20_face must be 1 to 20"},
		{"d20 above 20", concentrationFace(21), "d20_face must be 1 to 20"},
		{"hand false", func(r *playv1.ResolveConcentrationSaveRequest) {
			r.Roll = &playv1.ResolveConcentrationSaveRequest_HandToMaster{HandToMaster: false}
		}, "hand_to_master must be true"},
		{"keep false", func(r *playv1.ResolveConcentrationSaveRequest) {
			r.Roll = &playv1.ResolveConcentrationSaveRequest_Keep{Keep: false}
		}, "keep must be true"},
		{"nothing set", func(*playv1.ResolveConcentrationSaveRequest) {}, "set roll_in_app, d20_face, hand_to_master or keep"},
	}
	for _, tc := range cases {
		wantEffectSaveRefusal(t, tc.name, a.concentrationSaveFor(t, a.ana, e, w.GetId(), tc.set), connect.CodeInvalidArgument, tc.want)
	}
	wantEffectSaveRefusal(t, "a malformed window id", a.concentrationSaveFor(t, a.ana, e, "not-a-uuid", concentrationFace(0)), connect.CodeNotFound, "window not found")
}

func TestResolveConcentrationSaveWhoMayAnswer(t *testing.T) {
	t.Parallel()
	a, e := concentratingHit(t, 6)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want a concentration save", a.windows(t, a.ana))
	}
	const denied = "you cannot answer this reaction"
	hand := func(r *playv1.ResolveConcentrationSaveRequest) {
		r.Roll = &playv1.ResolveConcentrationSaveRequest_HandToMaster{HandToMaster: true}
	}
	wantEffectSaveRefusal(t, "another player", a.concentrationSaveFor(t, a.caio, e, w.GetId(), concentrationFace(10)), connect.CodePermissionDenied, denied)
	unknown := "00000000-0000-4000-8000-000000000001"
	wantEffectSaveRefusal(t, "a player asks for an unknown window", a.concentrationSaveFor(t, a.ana, e, unknown, concentrationFace(10)), connect.CodePermissionDenied, denied)
	wantEffectSaveRefusal(t, "the master asks for an unknown window", a.concentrationSaveFor(t, a.master, e, unknown, concentrationFace(10)), connect.CodeNotFound, "reaction window not found")
	// "Keep" is the master's option.
	wantEffectSaveRefusal(t, "a player keeps", a.concentrationSaveFor(t, a.ana, e, w.GetId(), keepConcentration), connect.CodePermissionDenied, denied)
	wantEffectSaveRefusal(t, "the master hands it to himself", a.concentrationSaveFor(t, a.master, e, w.GetId(), hand), connect.CodeInvalidArgument, "the master rolls it: use roll_in_app or d20_face")
	// A window of the other kind is refused the same way.
	_, err := a.master.lasting.RollEffectSave(t.Context(), connect.NewRequest(&playv1.RollEffectSaveRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), WindowId: w.GetId(), IdempotencyKey: newKey(),
		Roll: &playv1.RollEffectSaveRequest_RollInApp{RollInApp: true},
	}))
	wantEffectSaveRefusal(t, "a concentration window answered as an effect's save", err, connect.CodeInvalidArgument, "this window is not an effect's saving throw: use AnswerReaction")
	// Left to the master: the player's roll is refused, the window stays, the master rolls.
	res, err := a.resolveConcentration(t, a.ana, e, w.GetId(), hand)
	// Finding: handing the save over answers with a nil result (the closure returns nothing), where
	// RollEffectSave answers with an empty one. combat_concentration.go:201.
	if err != nil || res.GetResult() != nil || res.GetInspirationOffer() != nil || res.GetLuckyOffer() != nil {
		t.Fatalf("the player hands the save over = %v, %v; want an answer with no result", res, err)
	}
	wantEffectSaveRefusal(t, "the player rolls a handed save", a.concentrationSaveFor(t, a.ana, e, w.GetId(), concentrationFace(20)), connect.CodePermissionDenied, denied)
	if a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE) == nil {
		t.Fatal("the handed save is not open for the master")
	}
	a.h.roller.queue(2)
	res, err = a.resolveConcentration(t, a.master, e, w.GetId(), func(r *playv1.ResolveConcentrationSaveRequest) {
		r.Roll = &playv1.ResolveConcentrationSaveRequest_RollInApp{RollInApp: true}
	})
	if err != nil {
		t.Fatalf("the master rolls the handed save: %v", err)
	}
	if r := res.GetResult(); r.GetKept() || r.GetDc() != 10 || r.GetSave().GetTotal() != 3 || r.GetSave().GetPhysical() || r.GetSpellKey() != webSpell || r.GetSpellNamePt() == "" {
		t.Errorf("the master's result = %v, want an in-app 2 + 1 against 10 that failed Teia", r)
	}
	if got := byLabel(t, res.GetEncounter(), "Pensantus").GetConcentrationSpell(); got != "" {
		t.Errorf("Pensantus still concentrates on %q", got)
	}
}

func TestResolveConcentrationSaveMasterKeepsAndTypedPass(t *testing.T) {
	t.Parallel()
	a, e := concentratingHit(t, 6)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want a concentration save", a.windows(t, a.ana))
	}
	res, err := a.resolveConcentration(t, a.master, e, w.GetId(), keepConcentration)
	if err != nil {
		t.Fatalf("ResolveConcentrationSave(keep) error = %v", err)
	}
	if r := res.GetResult(); !r.GetKept() || r.GetDc() != 10 || r.GetSave() != nil || r.GetSpellKey() != webSpell || r.GetSpellNamePt() == "" {
		t.Errorf("the kept result = %v, want kept at DC 10 with no roll", r)
	}
	if got := byLabel(t, res.GetEncounter(), "Pensantus").GetConcentrationSpell(); got != webSpell {
		t.Errorf("Pensantus concentrates on %q, want Teia kept", got)
	}
	if a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE) != nil {
		t.Error("the window is still open after the keep")
	}
	// The log says the master kept it, to the owner too.
	var lines []string
	for _, l := range logEntries(a.log(t, a.ana, e)) {
		if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION_WINDOW {
			lines = append(lines, l.GetReactionTextPt())
		}
	}
	if len(lines) != 1 || lines[0] != "Pensantus manteve a concentração em Teia. (respondido pelo mestre)" {
		t.Errorf("the log's reaction lines = %q, want the master's keep", lines)
	}
}

func TestResolveConcentrationSaveTypedPassKeepsTheSpell(t *testing.T) {
	t.Parallel()
	a, e := concentratingHit(t, 6)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want a concentration save", a.windows(t, a.ana))
	}
	res, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(14))
	if err != nil {
		t.Fatalf("ResolveConcentrationSave() error = %v", err)
	}
	if r := res.GetResult(); !r.GetKept() || r.GetSave().GetTotal() != 15 || !r.GetSave().GetPhysical() || r.GetDc() != 10 {
		t.Errorf("the result = %v, want a typed 14 + 1 that kept the concentration", r)
	}
	if got := byLabel(t, res.GetEncounter(), "Pensantus").GetConcentrationSpell(); got != webSpell {
		t.Errorf("Pensantus concentrates on %q, want Teia kept", got)
	}
}

// A cast is one source of damage: the save its two damages ask is one, set from the sum of what
// both cost the target, and it opens with the apply that settles the last of them.
func TestAfterDamageOpensOneConcentrationSaveForTheWholeCast(t *testing.T) {
	t.Parallel()
	a, e := flameStrikeFixture(t)
	a.concentrate(t, "Toren", webSpell)
	fire, radiant := flameStrikeOn(t, a, e, "Toren")
	a.h.roller.queue(5, 5, 5, 5) // 20 fire
	a.mustDamage(t, a.master, e, fire, inAppDamage)
	a.h.roller.queue(2, 2, 2, 2) // 8 radiant
	a.mustDamage(t, a.master, e, radiant, inAppDamage)
	if _, err := a.settle(t, a.master, e, fire, true); err != nil {
		t.Fatalf("ApplyPendingDamage(fire) error = %v", err)
	}
	if a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE) != nil {
		t.Fatal("a concentration window opened with the fire damage alone")
	}
	if _, err := a.settle(t, a.master, e, radiant, true); err != nil {
		t.Fatalf("ApplyPendingDamage(radiant) error = %v", err)
	}
	w := a.windowOf(t, a.caio, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("Toren's windows = %v, want one concentration save", a.windows(t, a.caio))
	}
	if p := w.GetConcentrationSave(); p.GetDc() != 14 || p.GetDamageTaken() != 28 || p.GetSpellKey() != webSpell {
		t.Errorf("the prompt = %v, want DC 14 from 28 damage taken on Teia", p)
	}
	var n int
	for _, x := range a.windows(t, a.master) {
		if x.GetKind() == playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the master reads %d concentration windows, want 1", n)
	}
}

// Hideous Laughter's save when the target takes damage is rolled with advantage: two typed d20 are
// needed, the higher counts, and the one-d20 answer is refused.
func TestRollEffectSaveAfterDamageRollsWithAdvantage(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, "spell:hideous-laughter", []string{"Goblin"}, a.rounds(t, 10, "Pensantus"))
	hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{15, 3} })
	a.h.roller.queue(2)
	a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), inAppDamage)
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the save the damage asks", a.windows(t, a.master))
	}
	_, err := a.rollEffectSave(t, a.master, e, w.GetId(), effectSaveFace(5))
	wantEffectSaveRefusal(t, "one d20 on a save with advantage", err, connect.CodeInvalidArgument, "the roll takes 2 d20: type 2 face(s) in d20_faces")
	res, err := a.rollEffectSave(t, a.master, e, w.GetId(), func(r *playv1.RollEffectSaveRequest) {
		r.Roll = &playv1.RollEffectSaveRequest_D20Face{D20Face: 5}
		r.SecondD20Face = new(int32(18))
	})
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if r := res.GetResult(); r.GetD20() != 18 || r.GetTotal() != 18 || len(r.GetD20Faces()) != 2 || !r.GetSaved() || !r.GetEffectEnded() || r.GetTextPt() != "Riso Histérico acabou." || r.GetDc() != 10 {
		t.Errorf("the result = %v, want the higher of 5 and 18 against the default DC 10, ending the laughter", r)
	}
}
