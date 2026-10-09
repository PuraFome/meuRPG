package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Web (SRD 5.1): a creature that starts its turn in the web makes a Dexterity save, and on a
// failure it is restrained until it frees itself.
func TestWebAsksADexteritySaveAtTheStartOfTheTurnAndRestrainsOnAFailure(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, webSpell, []string{"Goblin"}, a.rounds(t, 10, "Pensantus"))
	for range 8 { // the helper that passes turns skips the saves: this test answers them
		if a.get(t, a.master).GetCurrentCombatantId() == a.id(t, "Goblin") {
			break
		}
		if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
			t.Fatalf("EndTurn() error = %v", err)
		}
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the Goblin's Dexterity save", a.windows(t, a.master))
	}
	if p := w.GetEffectSave(); p.GetPhase() != playv1.EffectPhase_EFFECT_PHASE_START || p.GetAbility() != "dex" {
		t.Errorf("the prompt = %v, want a Dexterity save at the start of the turn", p)
	}
	res, err := a.rollEffectSave(t, a.master, e, w.GetId(), effectSaveFace(1))
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if res.GetResult().GetSaved() {
		t.Fatalf("the result = %v, want a failed save", res.GetResult())
	}
	goblin := byLabel(t, a.get(t, a.master), "Goblin")
	if !slices.Contains(goblin.GetConditions(), "condition:restrained") {
		t.Errorf("the Goblin's conditions = %v, want restrained", goblin.GetConditions())
	}
	if goblin.GetMovementLeftDft() != 0 {
		t.Errorf("a restrained Goblin has %d movement, want 0", goblin.GetMovementLeftDft())
	}
}

// Faerie Fire (SRD 5.1): attack rolls against an outlined creature have advantage if the
// attacker sees it.
func TestFaerieFireGivesAdvantageToAttacksAgainstTheOutlinedCreature(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, "spell:faerie-fire", []string{"Goblin"}, a.rounds(t, 10, "Pensantus"))
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{12, 3} })
	found := false
	for _, s := range res.GetRoll().GetSources() {
		found = found || s.GetKind() == playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_OUTLINED_TARGET
	}
	if !found || res.GetRoll().GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE {
		t.Errorf("the attack = %v, want advantage from the outline", res.GetRoll())
	}
}

// Hideous Laughter (SRD 5.1): the creature falls prone and incapacitated; it repeats the save at
// the end of each turn and again each time it takes damage, with advantage.
func TestHideousLaughterKnocksDownAndTheSaveEndsIt(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, "spell:hideous-laughter", []string{"Goblin"}, a.rounds(t, 10, "Pensantus"))
	goblin := byLabel(t, a.get(t, a.master), "Goblin")
	if !slices.Contains(goblin.GetConditions(), "condition:prone") || !slices.Contains(goblin.GetConditions(), "condition:incapacitated") {
		t.Fatalf("the Goblin's conditions = %v, want prone and incapacitated", goblin.GetConditions())
	}
	a.passTo(t, e, "Goblin")
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatal("the end-of-turn save did not open")
	}
	if _, err := a.rollEffectSave(t, a.master, e, w.GetId(), effectSaveFace(20)); err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if slices.Contains(byLabel(t, a.get(t, a.master), "Goblin").GetConditions(), "condition:incapacitated") {
		t.Error("the Goblin is still incapacitated after the save")
	}
}

// A window closes when its effect goes first (the master ends it): nothing is left to answer, and
// the turn that waited goes on.
func TestEndingAnEffectClosesItsSavingThrowWindow(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	res := a.mustAddEffect(t, e, "spell:hideous-laughter", []string{"Goblin"}, a.rounds(t, 10, "Pensantus"))
	a.passTo(t, e, "Goblin")
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	if a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE) == nil {
		t.Fatal("the save did not open")
	}
	if _, err := a.master.lasting.EndLastingEffect(t.Context(), connect.NewRequest(&playv1.EndLastingEffectRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), EffectId: res.GetEffects()[0].GetId(), Scope: playv1.EffectEndScope_EFFECT_END_SCOPE_THIS,
	})); err != nil {
		t.Fatalf("EndLastingEffect() error = %v", err)
	}
	if a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE) != nil {
		t.Error("the save window stayed open after its effect ended")
	}
	if cur := a.get(t, a.master).GetCurrentCombatantId(); cur == a.id(t, "Goblin") {
		t.Error("the turn is still waiting on the Goblin")
	}
}

// A concentration spell's effects go with the caster's concentration, and a window that waited
// for one of them closes.
func TestTheCastersConcentrationEndingEndsTheEffectsAndTheirWindows(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, "spell:hideous-laughter", []string{"Goblin"}, func(r *playv1.AddLastingEffectRequest) {
		a.rounds(t, 10, "Pensantus")(r)
		r.CasterId = a.id(t, "Pensantus")
	})
	if _, err := a.master.combat.SetCombatantConditions(t.Context(), connect.NewRequest(&playv1.SetCombatantConditionsRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Pensantus"), IdempotencyKey: newKey(), EndConcentration: true,
	})); err != nil {
		t.Fatalf("SetCombatantConditions() error = %v", err)
	}
	if got := byLabel(t, a.get(t, a.master), "Goblin"); len(got.GetEffects()) != 0 {
		t.Errorf("the Goblin still has %v after the caster stopped concentrating", got.GetEffects())
	}
}

// Hold Person (SRD 5.1) picks a humanoid: on a beast the cast has no effect, a player reads "A
// magia não teve efeito." and only the master reads why (RN-20).
func TestHoldPersonOnABeastHasNoEffectAndOnlyTheMasterReadsWhy(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.monsterSetup(t, false)
	a.h.roller.queue(1)
	a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) { r.CreatureKey, r.Count = "monster:wolf", 1 })
	e = a.get(t, a.master)
	var wolf string
	for _, c := range e.GetCombatants() {
		if c.GetBestiaryCreatureKey() == "monster:wolf" {
			wolf = c.GetLabel()
		}
	}
	if wolf == "" {
		t.Fatalf("no wolf in %v", labels(e))
	}
	for label, sq := range map[string][2]int32{"Toren": {6, 5}, "Pensantus": {5, 5}, "Brisa": {5, 6}, "Goblin": {12, 8}, wolf: {7, 5}} {
		if _, err := a.master.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(), Col: sq[0], Row: sq[1],
		})); err != nil {
			t.Fatalf("MoveCombatant(%s) error = %v", label, err)
		}
	}
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, wolf), IdempotencyKey: newKey(), Hidden: false,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	e = a.begin(t, e)
	a.passTo(t, e, "Pensantus")
	res, err := a.cast(t, a.ana, e, "Pensantus", holdPerson, slotOfLevel(2), a.at(t, wolf), noCastRoll)
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	player := res.GetCast().GetTargets()[0]
	if player.GetLastingEffect() != playv1.LastingEffectOutcome_LASTING_EFFECT_OUTCOME_NO_EFFECT {
		t.Errorf("the target as the player reads it = %v, want no effect", player)
	}
	if player.GetNoEffectReasonMaster() != "" {
		t.Errorf("the player reads the reason %q", player.GetNoEffectReasonMaster())
	}
	if wolfNow := byLabel(t, a.get(t, a.master), wolf); len(wolfNow.GetEffects()) != 0 || slices.Contains(wolfNow.GetConditions(), "condition:paralyzed") {
		t.Errorf("the wolf is held: %v", wolfNow)
	}
}

// Hideous Laughter: each time the creature takes damage it repeats the save, with advantage.
func TestHideousLaughterAsksTheSaveAgainWhenTheTargetTakesDamage(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, "spell:hideous-laughter", []string{"Goblin"}, a.rounds(t, 10, "Pensantus"))
	hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{15, 3} })
	pending := hit.GetPendingDamage()
	if pending == nil {
		t.Fatalf("the attack = %v, want a hit", hit.GetRoll())
	}
	a.h.roller.queue(2)
	a.mustDamage(t, a.caio, e, pending.GetId(), inAppDamage)
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the save the damage asks", a.windows(t, a.master))
	}
	if p := w.GetEffectSave(); p.GetMode() != "advantage" && p.GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE.String() {
		t.Errorf("the prompt's mode = %q, want advantage", p.GetMode())
	}
}
