package reaction

import "testing"

func TestCounterspellFollowsTheSlot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		slot, spell int
		auto        bool
		dc          int
	}{
		{"a cantrip is countered", 3, 0, true, 0},
		{"a 3rd level spell with the 3rd slot", 3, 3, true, 0},
		{"a 4th level spell with the 3rd slot needs a check of 14", 3, 4, false, 14},
		{"a 4th level spell with the 4th slot fails by itself", 4, 4, true, 0},
		{"a 9th level spell with the 5th slot needs a check of 19", 5, 9, false, 19},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			auto, dc := Counterspell(tt.slot, tt.spell)
			if auto != tt.auto || dc != tt.dc {
				t.Errorf("Counterspell(%d, %d) = %v, %d; want %v, %d", tt.slot, tt.spell, auto, dc, tt.auto, tt.dc)
			}
		})
	}
}

func TestUncannyDodgeRoundsDown(t *testing.T) {
	t.Parallel()
	for in, want := range map[int]int{9: 4, 8: 4, 1: 0, 0: 0, -3: 0} {
		if got := UncannyDodge(in); got != want {
			t.Errorf("UncannyDodge(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestDeflectMissilesNeverGoesBelowZero(t *testing.T) {
	t.Parallel()
	if left, red := DeflectMissiles(9, 7, 3, 5); left != 0 || red != 15 {
		t.Errorf("DeflectMissiles(9, 7, 3, 5) = %d, %d; want 0, 15", left, red)
	}
	if left, red := DeflectMissiles(20, 2, 0, 3); left != 15 || red != 5 {
		t.Errorf("DeflectMissiles(20, 2, 0, 3) = %d, %d; want 15, 5", left, red)
	}
	if left, red := DeflectMissiles(4, 1, -3, 3); left != 3 || red != 1 {
		t.Errorf("a negative Dexterity modifier reduces less: got %d, %d; want 3, 1", left, red)
	}
}

func TestHellishRebukeAddsADieForEachSlotLevelAboveTheFirst(t *testing.T) {
	t.Parallel()
	for slot, want := range map[int]int{1: 2, 2: 3, 5: 6} {
		if got := HellishRebukeDice(slot); got != want {
			t.Errorf("HellishRebukeDice(%d) = %d, want %d", slot, got, want)
		}
	}
}

func TestCuttingWordsDieGrowsWithTheBardLevel(t *testing.T) {
	t.Parallel()
	for level, want := range map[int]int{1: 6, 4: 6, 5: 8, 9: 8, 10: 10, 14: 10, 15: 12, 20: 12} {
		if got := CuttingWordsDie(level); got != want {
			t.Errorf("CuttingWordsDie(%d) = %d, want %d", level, got, want)
		}
	}
}

func TestConcentrationDCIsTenOrHalfTheDamage(t *testing.T) {
	t.Parallel()
	for dmg, want := range map[int]int{1: 10, 9: 10, 20: 10, 21: 10, 22: 11, 28: 14, 0: 10} {
		if got := ConcentrationDC(dmg); got != want {
			t.Errorf("ConcentrationDC(%d) = %d, want %d", dmg, got, want)
		}
	}
}

func TestClosureReasonsAreTheReactorsOwnFirst(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		kind Kind
		f    Facts
		want Reason
		ok   bool
	}{
		{"nothing changed", Shield, Facts{}, ReasonNone, false},
		{"the reaction was spent", FeatherFall, Facts{ReactionUsed: true}, ReasonReactionSpent, true},
		{"the reactor fell unconscious", HellishRebukeKind, Facts{Incapacitated: true}, ReasonReactorIncapacitated, true},
		{"incapacitated beats spent", CounterspellKind, Facts{Incapacitated: true, ReactionUsed: true}, ReasonReactorIncapacitated, true},
		{"the spell was countered", CounterspellKind, Facts{TriggerGone: true}, ReasonTriggerGone, true},
		{"spent beats a gone trigger", CounterspellKind, Facts{ReactionUsed: true, TriggerGone: true}, ReasonReactionSpent, true},
		{"a concentration save ignores the reaction", Concentration, Facts{ReactionUsed: true}, ReasonNone, false},
		{"a concentration save ends when the concentration did", Concentration, Facts{TriggerGone: true}, ReasonTriggerGone, true},
		{"the master's check never closes for a state", MasterCheck, Facts{ReactionUsed: true, Incapacitated: true}, ReasonNone, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Closure(tt.kind, tt.f)
			if got != tt.want || ok != tt.ok {
				t.Errorf("Closure(%v, %+v) = %q, %v; want %q, %v", tt.kind, tt.f, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestAnswerNowIsTheFirstOpenWindowOfItsGroup(t *testing.T) {
	t.Parallel()
	all := []Window{
		{ID: "a", Group: "g1", Seq: 1, Open: false},
		{ID: "b", Group: "g1", Seq: 2, Open: true},
		{ID: "c", Group: "g1", Seq: 3, Open: true},
		{ID: "d", Group: "g2", Seq: 4, Open: true},
	}
	for id, want := range map[string]bool{"a": false, "b": true, "c": false, "d": true, "zz": false} {
		if got := AnswerNow(all, id); got != want {
			t.Errorf("AnswerNow(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestWaitSentences(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		wait   Wait
		holds  Holds
		trap   string
		title  string
		detail string
	}{
		{"the master", Wait{Master: true}, HoldsYourAttack, "", "Esperando o mestre", "O resultado do seu ataque sai quando ele responder."},
		{"a visible player", Wait{Reactors: []string{"Pensantus"}}, HoldsFall, "Fosso", "Esperando a reação de Pensantus", "A queda no Fosso só é resolvida quando responder."},
		{"both", Wait{Master: true, Reactors: []string{"Sálvia"}}, HoldsTurn, "", "Esperando o mestre e a reação de Sálvia", "O turno continua quando ele responder."},
		{"two players", Wait{Reactors: []string{"Sálvia", "Brisa"}}, HoldsYourSpell, "", "Esperando a reação de Brisa e Sálvia", "A sua conjuração se resolve quando responder."},
		{"a save", Wait{Savers: []string{"Sálvia"}}, HoldsTurn, "", "Esperando o teste de Constituição de Sálvia", "O turno continua quando responder."},
		{"the master's own", Wait{Self: true, Reactors: []string{"Pensantus"}}, HoldsTurn, "", "Esperando a sua reação e a reação de Pensantus", "O turno continua quando responder."},
		{"nothing", Wait{}, HoldsTurn, "", "", "O turno continua quando responder."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.wait.Title(); got != tt.title {
				t.Errorf("Title() = %q, want %q", got, tt.title)
			}
			if got := Detail(tt.holds, tt.wait, tt.trap); got != tt.detail {
				t.Errorf("Detail() = %q, want %q", got, tt.detail)
			}
		})
	}
}
