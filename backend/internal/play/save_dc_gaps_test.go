package play

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The two saves the table rule "Os jogadores veem a CD dos testes de resistência" (RN-24, RN-10) left
// out in train 36, and now covers: the Constitution save of a concentration (a player is asked to roll it
// when damage threatens a spell), and the Dexterity save against a Hellish Rebuke an NPC casts on a
// player's character (the app rolls it for them). The DC reaches the player who rolls only when the
// rule is on; the master always reads it; a player who does not roll never does. Turn Undead is not
// here: only undead creatures roll it.

// keepDC is the DC of the concentration tests: half of the 32 damage the Capitão's blow
// and the fire of the roll come to, which no other number of these answers is.
const keepDC = 16

// concentrationTable is a party whose Pensantus (a wizard of the 8th level, so she survives it)
// concentrates on Teia and is hurt, with the rule on or off: the damage that asks her the save.
func concentrationTable(t *testing.T, on bool) (*armed, *playv1.Encounter, *playv1.ReactionWindow) {
	t.Helper()
	a := newCastersAt(t, 8)
	if on {
		a.setRules(t, showSaveDC)
	}
	a, e := concentratingHitOn(t, a, 30)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want a concentration save", a.windows(t, a.ana))
	}
	return a, e, w
}

// wantConcentrationDC checks a concentration DC against what the rule says the reader gets.
func wantConcentrationDC(t *testing.T, what string, got int32, on bool) {
	t.Helper()
	want := int32(0)
	if on {
		want = keepDC
	}
	if got != want {
		t.Errorf("rule %v: %s = %d, want %d", on, what, got, want)
	}
}

// The question: Pensantus's player reads the DC only with the rule on, the master always, another
// player never gets the question.
func TestSaveDCTheQuestionOfAConcentrationSave(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		a, _, w := concentrationTable(t, on)
		wantConcentrationDC(t, "the DC in Pensantus's player's question", w.GetConcentrationSave().GetDc(), on)
		mw := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
		if mw == nil || mw.GetConcentrationSave().GetDc() != keepDC {
			t.Errorf("rule %v: the master's question = %v, want the DC %d", on, mw, keepDC)
		}
		if ws := a.windows(t, a.caio); len(ws) != 0 {
			t.Errorf("rule %v: a bystander reads the windows %v", on, ws)
		}
	}
}

// The answer to the roll, a kept concentration and a lost one: the roller's result carries the DC only
// with the rule on; a roll the master makes (the player handed it over) carries it always.
func TestSaveDCTheResultOfAConcentrationSave(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		for _, face := range []int32{20, 2} {
			a, e, w := concentrationTable(t, on)
			res, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(face))
			if err != nil {
				t.Fatalf("ResolveConcentrationSave(%d) error = %v", face, err)
			}
			wantConcentrationDC(t, "the DC in the roller's result", res.GetResult().GetDc(), on)
		}
		a, e, w := concentrationTable(t, on)
		res, err := a.resolveConcentration(t, a.master, e, w.GetId(), keepConcentration)
		if err != nil {
			t.Fatalf("ResolveConcentrationSave(keep) error = %v", err)
		}
		if got := res.GetResult().GetDc(); got != keepDC {
			t.Errorf("rule %v: the master's result of a concentration kept as it is carries the DC %d, want %d", on, got, keepDC)
		}

		a, e, w = concentrationTable(t, on)
		if _, err := a.resolveConcentration(t, a.ana, e, w.GetId(), func(r *playv1.ResolveConcentrationSaveRequest) {
			r.Roll = &playv1.ResolveConcentrationSaveRequest_HandToMaster{HandToMaster: true}
		}); err != nil {
			t.Fatalf("ResolveConcentrationSave(hand_to_master) error = %v", err)
		}
		res, err = a.resolveConcentration(t, a.master, e, w.GetId(), func(r *playv1.ResolveConcentrationSaveRequest) {
			r.Roll = &playv1.ResolveConcentrationSaveRequest_RollInApp{RollInApp: true}
		})
		if err != nil {
			t.Fatalf("ResolveConcentrationSave(master) error = %v", err)
		}
		if got := res.GetResult().GetDc(); got != keepDC {
			t.Errorf("rule %v: the master's result carries the DC %d, want %d", on, got, keepDC)
		}
	}
}

// concentrationLine is the log line of the save as u reads it ("" when there is none).
func (a *armed) concentrationLine(t *testing.T, u *user, e *playv1.Encounter) string {
	t.Helper()
	for _, en := range logEntries(a.log(t, u, e)) {
		if txt := en.GetReactionTextPt(); strings.Contains(txt, "concentração em") {
			return txt
		}
	}
	return ""
}

// The log line of the save: the roller's numbers (the d20, the bonus and the total) are theirs, the DC
// only with the rule on; the master's line always has it; another player reads neither.
func TestSaveDCTheLogLineOfAConcentrationSave(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		for _, face := range []int32{20, 2} {
			a, e, w := concentrationTable(t, on)
			if _, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(face)); err != nil {
				t.Fatalf("ResolveConcentrationSave(%d) error = %v", face, err)
			}
			ana := a.concentrationLine(t, a.ana, e)
			if !strings.Contains(ana, "1d20") {
				t.Fatalf("rule %v: Pensantus's player reads %q, want her dice", on, ana)
			}
			if got := strings.Contains(ana, "contra CD 16"); got != on {
				t.Errorf("rule %v, d20 %d: Pensantus's player reads %q, DC shown = %v", on, face, ana, got)
			}
			if got := a.concentrationLine(t, a.master, e); !strings.Contains(got, "contra CD 16") {
				t.Errorf("rule %v: the master reads %q, want the DC", on, got)
			}
			if got := a.concentrationLine(t, a.caio, e); strings.Contains(got, "CD") || strings.Contains(got, "1d20") {
				t.Errorf("rule %v: a bystander reads %q", on, got)
			}
		}
	}
}

// damageDCOf is the concentration DC of the applied damage in the log u reads (0: none).
func (a *armed) damageDCOf(t *testing.T, u *user, e *playv1.Encounter) int32 {
	t.Helper()
	for _, en := range logEntries(a.log(t, u, e)) {
		if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && en.GetDamage().GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
			return en.GetDamage().GetConcentrationDc()
		}
	}
	return 0
}

// The reminder on the damage that threatens the concentration ("Teste de Constituição, CD 16"): the
// target's player reads it only with the rule on, the master always, another player never. The answer
// to the master's apply is the master's.
func TestSaveDCTheDamageThatThreatensAConcentration(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		a, e, _ := concentrationTable(t, on)
		wantConcentrationDC(t, "the DC on the damage in Pensantus's player's log", a.damageDCOf(t, a.ana, e), on)
		if got := a.damageDCOf(t, a.master, e); got != keepDC {
			t.Errorf("rule %v: the master's log DC = %d, want %d", on, got, keepDC)
		}
		if got := a.damageDCOf(t, a.caio, e); got != 0 {
			t.Errorf("rule %v: a bystander's log DC = %d, want none", on, got)
		}
	}
}

// ---- Hellish Rebuke on a player's character ----

// rebukeLineOf is the Hellish Rebuke line of the log u reads ("" when there is none).
func (a *armed) rebukeLineOf(t *testing.T, u *user, e *playv1.Encounter) string {
	t.Helper()
	for _, en := range logEntries(a.log(t, u, e)) {
		if txt := en.GetReactionTextPt(); strings.Contains(txt, "Repreensão Infernal") {
			return txt
		}
	}
	return ""
}

// Diabrete, an NPC tiefling, answers Toren's blow with a Hellish Rebuke and the app rolls Toren's
// Dexterity save at once. Toren's player (Ana) reads the DC of that save in the log only with the rule
// on; the master's line always has it; a player whose character is not the target never does.
func TestSaveDCAHellishRebukeOnAPlayersCharacter(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		a, e := rebukeFight(t, "Toren")
		if on {
			a.setRules(t, showSaveDC)
		}
		r := a.rebukeOn(t, e, "Toren", 12, 5, 5, 5)
		dc := r.GetSaveDc()
		if dc == 0 {
			t.Fatal("the master's result carries no DC")
		}
		want := "CD " + itoa(dc)
		if got := a.rebukeLineOf(t, a.master, e); !strings.Contains(got, want) {
			t.Errorf("rule %v: the master reads %q, want %q", on, got, want)
		}
		ana := a.rebukeLineOf(t, a.ana, e)
		if ana == "" {
			t.Fatalf("rule %v: Toren's player reads no Hellish Rebuke line", on)
		}
		if got := strings.Contains(ana, want); got != on {
			t.Errorf("rule %v: Toren's player reads %q, DC shown = %v", on, ana, got)
		}
		if got := a.rebukeLineOf(t, a.caio, e); strings.Contains(got, "CD") {
			t.Errorf("rule %v: a bystander reads %q", on, got)
		}
	}
}

// The caster's own player keeps the DC of their spell with the rule off: the first question of a
// warlock's Hellish Rebuke carries it.
func TestSaveDCTheCastersPlayerKeepsTheDCOfHellishRebuke(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, pactWarlock)
	w := hurtReator(t, a, e)
	if w.GetHellishRebuke().GetSaveDc() == 0 {
		t.Errorf("the warlock's player reads the prompt %v, want the DC of their own spell with the rule off", w.GetHellishRebuke())
	}
}
