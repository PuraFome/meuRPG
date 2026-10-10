package combat

import (
	"slices"
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

func TestExhaustionAndEffectsChangeTheModeOfARoll(t *testing.T) {
	t.Parallel()
	has := func(sources []Source, kind string, effect RollMode) bool {
		return slices.ContainsFunc(sources, func(s Source) bool { return s.Kind == kind && s.Effect == effect })
	}
	// Level 3 and up: disadvantage on attack rolls and saving throws; level 1: on ability checks.
	atk := AttackMode(AttackScene{Attacker: Creature{Exhaustion: 3}, Target: Creature{}}).Sources
	if !has(atk, SourceExhaustionAttack, ModeDisadvantage) {
		t.Errorf("attack sources at level 3 = %v, want the exhaustion", atk)
	}
	if has(AttackMode(AttackScene{Attacker: Creature{Exhaustion: 2}, Target: Creature{}}).Sources, SourceExhaustionAttack, ModeDisadvantage) {
		t.Error("level 2 gives disadvantage on attack rolls")
	}
	if !has(SaveMode(SaveScene{Creature: Creature{Exhaustion: 3}, Ability: "wis"}), SourceExhaustionSave, ModeDisadvantage) {
		t.Error("level 3 gives no disadvantage on saving throws")
	}
	if !has(SaveMode(SaveScene{Creature: Creature{Exhaustion: 1}, Ability: "wis", Check: true}), SourceExhaustionCheck, ModeDisadvantage) {
		t.Error("level 1 gives no disadvantage on ability checks")
	}
	if has(SaveMode(SaveScene{Creature: Creature{Exhaustion: 1}, Ability: "wis"}), SourceExhaustionSave, ModeDisadvantage) {
		t.Error("level 1 gives disadvantage on saving throws")
	}
	// Haste: advantage on Dexterity saves; Faerie Fire: advantage against the outlined target.
	if !has(SaveMode(SaveScene{Creature: Creature{SaveAdvantage: []string{"dex"}}, Ability: "dex"}), SourceEffectSave, ModeAdvantage) {
		t.Error("an effect's advantage on Dexterity saves is not a source")
	}
	if !has(AttackMode(AttackScene{Attacker: Creature{}, Target: Creature{Outlined: true}}).Sources, SourceOutlinedTarget, ModeAdvantage) {
		t.Error("an outlined target gives no advantage to an attacker who sees it")
	}
}

func TestSpeedAddAndCheckBonus(t *testing.T) {
	t.Parallel()
	mods := []rules.EffectModifier{{Kind: rules.ModifierSpeedAdd, Value: 10}, {Kind: rules.ModifierCheckBonus, Skill: "skill:stealth", Value: 10}}
	if got := SpeedAddFt(mods); got != 10 {
		t.Errorf("SpeedAddFt = %d, want 10", got)
	}
	if CheckBonus(mods, "skill:stealth") != 10 || CheckBonus(mods, "skill:perception") != 0 {
		t.Error("the bonus goes to Stealth alone")
	}
}

func TestCheckAdvantageImmunityAndTurnTempHPOfTheModifiers(t *testing.T) {
	t.Parallel()
	mods := []rules.EffectModifier{
		{Kind: rules.ModifierCheckAdvantage, Abilities: []string{"str"}},
		{Kind: rules.ModifierConditionImmunity, Conditions: []string{"condition:frightened"}},
		{Kind: rules.ModifierTurnTempHP, Value: 3},
		{Kind: rules.ModifierTurnTempHP, Value: 2},
		{Kind: rules.ModifierRollDie, Die: 4, Sign: 1, AppliesTo: []string{rules.RollAppliesCheck}, Once: true},
	}
	if !CheckAdvantage(mods, "str") || CheckAdvantage(mods, "dex") {
		t.Error("CheckAdvantage should hold for Strength only")
	}
	if !ImmuneTo(mods, "condition:frightened") || ImmuneTo(mods, "condition:charmed") {
		t.Error("ImmuneTo should hold for Frightened only")
	}
	if got := TurnTempHP(mods); got != 3 {
		t.Errorf("TurnTempHP() = %d, want the highest (3): temporary hit points do not stack", got)
	}
	dice := EffectDice("spell:guidance", mods, rules.RollAppliesCheck)
	if len(dice) != 1 || !dice[0].Once {
		t.Errorf("EffectDice() = %v, want a d4 that is spent by the roll", dice)
	}
	if got := EffectDice("spell:guidance", mods, rules.RollAppliesSave); len(got) != 0 {
		t.Errorf("a die of checks was added to a save: %v", got)
	}
	sources := SaveMode(SaveScene{Creature: Creature{CheckAdvantage: []string{"str"}}, Ability: "str", Check: true})
	if Resolve(sources) != ModeAdvantage {
		t.Errorf("a Strength check with Enhance Ability = %v, want advantage", Resolve(sources))
	}
	if got := Resolve(SaveMode(SaveScene{Creature: Creature{CheckAdvantage: []string{"str"}}, Ability: "str"})); got != ModeNormal {
		t.Errorf("a Strength save with Enhance Ability = %v, want a normal roll", got)
	}
}
