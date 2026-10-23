package play

import (
	"slices"
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A gnome's Gnome Cunning (SRD 5.1): advantage on Intelligence, Wisdom and Charisma saving throws against magic. The
// save of a spell's effect that lasts says what it is against, so the window shows the advantage and says why, and the
// d20 is rolled with it; a save of the same effect on a creature without the trait is a normal roll.
func TestAGnomeSavesWithAdvantageAgainstAnEffectOfASpell(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, holdPerson, []string{"Pensantus"}, a.rounds(t, 10, "Pensantus"))
	a.passTo(t, e, "Pensantus")
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want the save of Hold Person", a.windows(t, a.ana))
	}
	p := w.GetEffectSave()
	if p.GetAbility() != "wis" || p.GetMode() != "advantage" || len(p.GetModeSourcesPt()) != 1 ||
		!strings.Contains(p.GetModeSourcesPt()[0], "Esperteza Gnômica") || !strings.Contains(p.GetModeSourcesPt()[0], "contra magia") {
		t.Fatalf("the prompt = %v, want advantage from Gnome Cunning against magic", p)
	}
	a.h.roller.queue(3, 17)
	res, err := a.rollEffectSave(t, a.ana, e, w.GetId(), func(r *playv1.RollEffectSaveRequest) {
		r.Roll = &playv1.RollEffectSaveRequest_RollInApp{RollInApp: true}
	})
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	r := res.GetResult()
	if r.GetD20() != 17 || !slices.Equal(r.GetD20Faces(), []int32{3, 17}) {
		t.Errorf("the save = %v, want two dice, the 17 counting", r)
	}
}

// The control: a creature without the trait saves against the same effect with no advantage.
func TestAnotherCreatureSavesNormallyAgainstTheSameEffect(t *testing.T) {
	t.Parallel()
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
	if p := w.GetEffectSave(); p.GetMode() != "normal" || len(p.GetModeSourcesPt()) != 0 {
		t.Errorf("the prompt = %v, want a normal roll", p)
	}
}
