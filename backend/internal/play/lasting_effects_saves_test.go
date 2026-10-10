package play

import (
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Bênção and Perdição add their d4 to every saving throw (SRD 5.1): the one a trap asks of a
// creature in a combat, and the Constitution save that keeps a concentration.

func TestBlessAddsItsDieToATrapSavingThrowInACombat(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	statue := r.trap(t, "Estátua de Fogo", 15, 12, func(s *mapsv1.TrapSpec) {
		s.Trigger = rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL
		s.Effect = &rulesv1.TrapEffect{
			Save: &rulesv1.TrapSaveEffect{
				Ability: rulesv1.Ability_ABILITY_DEXTERITY, Dc: 15, AppliesTo: rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_CAUGHT,
				OnFail: &rulesv1.TrapOnFail{Damage: []*rulesv1.TrapDamage{{Dice: "1d6", DamageTypeKey: "damage-type:fire"}}},
				OnPass: rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE,
			},
			Targets: rulesv1.TrapTargets_TRAP_TARGETS_MANUAL,
		}
	})
	e := r.fight(t)
	r.mustAddEffect(t, e, blessKey, []string{"Toren"}, r.rounds(t, 10, "Toren"))
	// 10 + 2 (Dexterity) alone is 12 and fails DC 15; the d4 of 4 makes it 16 and passes.
	r.h.roller.queue(10, 4, 3)
	res, err := r.fireByHand(t, statue, r.id(t, "Toren"))
	if err != nil {
		t.Fatalf("FireTrap() error = %v", err)
	}
	save := res.GetFiring().GetCaught()[0].GetSave()
	if save.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED || save.GetRoll().GetTotal() != 16 {
		t.Errorf("Toren's save = %v, want a total of 16 that passed", save)
	}
	if !hasDieSourceIn(save.GetSources(), "+1d4: 4") {
		t.Errorf("the save's sources = %v, want Bênção +1d4: 4", save.GetSources())
	}
}

func hasDieSourceIn(sources []*playv1.AdvantageSource, want string) bool {
	return hasDieSource(&playv1.SceneRoll{Sources: sources}, want)
}

func TestBlessAddsItsDieToAConcentrationSave(t *testing.T) {
	t.Parallel()
	a, e := concentratingHit(t, 6)
	a.mustAddEffect(t, e, blessKey, []string{"Pensantus"}, a.rounds(t, 10, "Pensantus"))
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want a concentration save", a.windows(t, a.ana))
	}
	// A real d20 needs the real d4.
	_, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(6))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a physical save with the d4 missing: error = %v, want invalid_argument", err)
	}
	// 6 + 1 is 7 against DC 10; the d4 of 3 makes it 10 and keeps the concentration.
	res, err := a.resolveConcentration(t, a.ana, e, w.GetId(), func(r *playv1.ResolveConcentrationSaveRequest) {
		r.Roll = &playv1.ResolveConcentrationSaveRequest_D20Face{D20Face: 6}
		r.ExtraDieFaces = []int32{3}
	})
	if err != nil || !res.GetResult().GetKept() || res.GetResult().GetSave().GetTotal() != 10 {
		t.Fatalf("the save = %v, %v; want a total of 10 that kept the concentration", res.GetResult(), err)
	}
}
