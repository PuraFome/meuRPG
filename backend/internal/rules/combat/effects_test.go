package combat

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

func TestExhaustionLevelsAddUp(t *testing.T) {
	t.Parallel()
	want := []Exhaustion{
		{},
		{DisadvantageOnChecks: true},
		{DisadvantageOnChecks: true, SpeedHalved: true},
		{DisadvantageOnChecks: true, SpeedHalved: true, DisadvantageOnAttacksAndSaves: true},
		{DisadvantageOnChecks: true, SpeedHalved: true, DisadvantageOnAttacksAndSaves: true, MaxHPHalved: true},
		{DisadvantageOnChecks: true, SpeedHalved: true, DisadvantageOnAttacksAndSaves: true, MaxHPHalved: true, SpeedZero: true},
		{DisadvantageOnChecks: true, SpeedHalved: true, DisadvantageOnAttacksAndSaves: true, MaxHPHalved: true, SpeedZero: true, Dead: true},
	}
	for level, w := range want {
		if got := ExhaustionAt(level); got != w {
			t.Errorf("level %d = %+v, want %+v", level, got, w)
		}
	}
	if ExhaustionAt(9) != want[6] || ExhaustionAt(-3) != want[0] {
		t.Error("a level out of 0 to 6 is clamped")
	}
}

func TestExhaustionHalvesTheMaximumAndTheSpeed(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ max, level, want int }{
		{40, 3, 40}, {40, 4, 20}, {41, 4, 20}, {1, 4, 1}, {40, 6, 20}, {0, 4, 0},
	} {
		if got := ExhaustedMaxHP(tt.max, tt.level); got != tt.want {
			t.Errorf("ExhaustedMaxHP(%d, %d) = %d, want %d", tt.max, tt.level, got, tt.want)
		}
	}
	for _, tt := range []struct{ speed, level, want int }{
		{30, 1, 30}, {30, 2, 15}, {35, 2, 17}, {30, 4, 15}, {30, 5, 0},
	} {
		if got := ExhaustedSpeedFt(tt.speed, tt.level); got != tt.want {
			t.Errorf("ExhaustedSpeedFt(%d, %d) = %d, want %d", tt.speed, tt.level, got, tt.want)
		}
	}
}

func TestBlessAndBaneAddOrTakeAD4OnAttacksAndSavesOnly(t *testing.T) {
	t.Parallel()
	bless := []rules.EffectModifier{{Kind: rules.ModifierRollDie, Die: 4, Sign: 1, AppliesTo: []string{rules.RollAppliesAttack, rules.RollAppliesSave}}}
	bane := []rules.EffectModifier{{Kind: rules.ModifierRollDie, Die: 4, Sign: -1, AppliesTo: []string{rules.RollAppliesAttack, rules.RollAppliesSave}}}
	got := EffectDice("spell:bless", bless, rules.RollAppliesAttack)
	if len(got) != 1 || got[0].Faces != 4 || got[0].Signed(3) != 3 {
		t.Fatalf("Bless on an attack = %+v", got)
	}
	got = EffectDice("spell:bane", bane, rules.RollAppliesSave)
	if len(got) != 1 || got[0].Signed(3) != -3 || got[0].Source != "spell:bane" {
		t.Fatalf("Bane on a save = %+v", got)
	}
	if got := EffectDice("spell:bless", bless, "check"); len(got) != 0 {
		t.Errorf("neither spell touches ability checks, got %+v", got)
	}
}

func TestHasteModifiers(t *testing.T) {
	t.Parallel()
	haste := []rules.EffectModifier{
		{Kind: rules.ModifierACBonus, Value: 2},
		{Kind: rules.ModifierSpeedMultiplier, Value: 200},
		{Kind: rules.ModifierSaveAdvantage, Abilities: []string{"dex"}},
	}
	if got := ArmorClassBonus(haste); got != 2 {
		t.Errorf("ArmorClassBonus = %d, want 2", got)
	}
	if got := SpeedPercent(haste); got != 200 {
		t.Errorf("SpeedPercent = %d, want 200", got)
	}
	if got := SpeedPercent(nil); got != 100 {
		t.Errorf("SpeedPercent with no modifier = %d, want 100", got)
	}
	if !SaveAdvantage(haste, "dex") || SaveAdvantage(haste, "wis") {
		t.Error("Haste gives advantage on Dexterity saves only")
	}
}

func TestTheClockCountsRoundsToTheCastersTurn(t *testing.T) {
	t.Parallel()
	// Bless cast in round 1: a minute ends at the start of the caster's turn in round 11.
	e := EndsAtStartOf(1, 10, "tavo")
	if e != (EndsAt{Round: 11, CombatantID: "tavo", Phase: PhaseStart}) {
		t.Fatalf("EndsAtStartOf = %+v", e)
	}
	if got := e.RoundsLeft(3); got != 8 {
		t.Errorf("RoundsLeft(3) = %d, want 8 (\"Restam 8 rodadas\")", got)
	}
	if got := e.RoundsLeft(11); got != 0 {
		t.Errorf("RoundsLeft(11) = %d, want 0", got)
	}
	if (EndsAt{}).RoundsLeft(3) != 0 || (EndsAt{}).Timed() {
		t.Error("an effect with no end has no rounds left")
	}
}

func TestAnEffectExpiresAtTheStartOfItsAnchorsTurn(t *testing.T) {
	t.Parallel()
	e := EndsAtStartOf(1, 10, "tavo")
	if e.ExpiresAtStart([]string{"tavo"}, 10) {
		t.Error("round 10 is too early")
	}
	if e.ExpiresAtStart([]string{"toren"}, 11) {
		t.Error("another combatant's turn does not end it")
	}
	if !e.ExpiresAtStart([]string{"tavo"}, 11) {
		t.Error("the anchor's turn in round 11 ends it")
	}
	if !e.ExpiresAtStart([]string{"toren"}, 12) {
		t.Error("a round past the end settles an anchor that never took its turn")
	}
	if e.ExpiresAtEnd("tavo", 11) {
		t.Error("an effect that ends at a start does not end at an end")
	}
	// The web that burns ends at the start of the next round: no anchor.
	burn := EndsAt{Round: 5, Phase: PhaseStart}
	if !burn.ExpiresAtStart([]string{"anyone"}, 5) || burn.ExpiresAtStart([]string{"anyone"}, 4) {
		t.Error("an effect with no anchor ends at the start of its round")
	}
}

func TestAnEffectEndsAtTheEndOfItsAnchorsTurn(t *testing.T) {
	t.Parallel()
	e := EndsAt{Round: 3, CombatantID: "toren", Phase: PhaseEnd}
	if e.ExpiresAtEnd("toren", 2) || e.ExpiresAtEnd("tavo", 3) {
		t.Error("too early, or not the anchor")
	}
	if !e.ExpiresAtEnd("toren", 3) || !e.ExpiresAtEnd("toren", 4) {
		t.Error("the anchor's turn from round 3 on ends it")
	}
	if e.ExpiresAtStart([]string{"toren"}, 3) {
		t.Error("an end-phase effect does not end at a start")
	}
}

func TestOnlyAHumanoidIsHeldByHoldPerson(t *testing.T) {
	t.Parallel()
	for typ, want := range map[string]bool{"humanoid": true, "": true, "beast": false, "undead": false, "dragon": false} {
		if got := IsHumanoid(typ); got != want {
			t.Errorf("IsHumanoid(%q) = %v, want %v", typ, got, want)
		}
	}
}
