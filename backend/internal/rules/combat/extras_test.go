package combat

import (
	"errors"
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

func byKey(extras []Extra, key string) (Extra, bool) {
	i := slices.IndexFunc(extras, func(e Extra) bool { return e.Key == key })
	if i < 0 {
		return Extra{}, false
	}
	return extras[i], true
}

func TestSneakAttackDiceByRogueLevel(t *testing.T) {
	want := map[int]int{0: 0, 1: 1, 2: 1, 3: 2, 4: 2, 5: 3, 9: 5, 11: 6, 19: 10, 20: 10}
	for level, dice := range want {
		if got := SneakAttackDice(level); got != dice {
			t.Errorf("SneakAttackDice(%d) = %d, want %d", level, got, dice)
		}
	}
}

func TestSmiteDiceByTheSlotSpent(t *testing.T) {
	want := map[int]int{1: 2, 2: 3, 3: 4, 4: 5, 5: 5, 9: 5}
	for slot, dice := range want {
		if got := SmiteDice(slot); got != dice {
			t.Errorf("SmiteDice(%d) = %d, want %d", slot, got, dice)
		}
	}
}

func TestRageBonusByBarbarianLevel(t *testing.T) {
	want := map[int]int{1: 2, 8: 2, 9: 3, 15: 3, 16: 4, 20: 4}
	for level, bonus := range want {
		if got := RageBonus(level); got != bonus {
			t.Errorf("RageBonus(%d) = %d, want %d", level, got, bonus)
		}
	}
}

func TestSneakAttackConditions(t *testing.T) {
	base := ExtraScene{Weapon: true, Finesse: true, Melee: true, SneakAttackDice: 3, WeaponName: "Espada curta", Mode: ModeNormal}
	tests := []struct {
		name      string
		edit      func(*ExtraScene)
		available bool
		selected  bool
		reason    string
	}{
		{"advantage with a finesse weapon", func(s *ExtraScene) { s.Mode = ModeAdvantage }, true, true, "Você tem vantagem neste ataque · uma vez por turno"},
		{"advantage with a ranged weapon", func(s *ExtraScene) { s.Mode = ModeAdvantage; s.Finesse = false; s.Melee = false; s.Ranged = true }, true, true, "Você tem vantagem neste ataque · uma vez por turno"},
		{"a longsword is neither finesse nor ranged", func(s *ExtraScene) {
			s.Mode = ModeAdvantage
			s.Finesse = false
			s.WeaponName = "Espada longa"
		}, false, false, "Espada longa não é de acuidade nem à distância."},
		{"a spell attack is no weapon attack", func(s *ExtraScene) { s.Mode = ModeAdvantage; s.Weapon = false }, false, false, "Espada curta não é de acuidade nem à distância."},
		{"already used this turn", func(s *ExtraScene) { s.Mode = ModeAdvantage; s.SneakUsed = true }, false, false, "Já usado neste turno (uma vez por turno)."},
		{"an enemy near the target without disadvantage", func(s *ExtraScene) { s.EnemyNearTarget = true }, true, true, "Um inimigo do alvo está a 1,5 m e você não tem desvantagem · uma vez por turno"},
		{"an enemy near the target but disadvantage", func(s *ExtraScene) { s.EnemyNearTarget = true; s.Mode = ModeDisadvantage }, false, false, "Você tem desvantagem neste ataque."},
		{"neither advantage nor an enemy near", func(s *ExtraScene) {}, false, false, "Sem vantagem e sem um inimigo do alvo a 1,5 m."},
		{"no map: the master confirms the ally", func(s *ExtraScene) { s.WithoutMap = true }, true, false, "O mestre confirma se há um aliado perto do alvo."},
		{"no map but advantage: no need of an ally", func(s *ExtraScene) { s.WithoutMap = true; s.Mode = ModeAdvantage }, true, true, "Você tem vantagem neste ataque · uma vez por turno"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := base
			tt.edit(&s)
			e, ok := byKey(Offered(s), ExtraSneakAttack)
			if !ok {
				t.Fatal("Sneak Attack is not listed")
			}
			if e.Available != tt.available || e.Selected != tt.selected || e.Reason != tt.reason {
				t.Fatalf("got available=%v selected=%v reason=%q, want %v %v %q", e.Available, e.Selected, e.Reason, tt.available, tt.selected, tt.reason)
			}
			if e.Count != 3 || e.Sides != 6 {
				t.Fatalf("dice = %dd%d, want 3d6", e.Count, e.Sides)
			}
		})
	}
	if _, ok := byKey(Offered(ExtraScene{Weapon: true}), ExtraSneakAttack); ok {
		t.Fatal("a character with no Sneak Attack is not offered it")
	}
}

func TestDivineSmiteConditions(t *testing.T) {
	base := ExtraScene{Weapon: true, Melee: true, DivineSmite: true, SmiteSlotFree: true}
	extras := Offered(base)
	smite, ok := byKey(extras, ExtraDivineSmite)
	extra, okExtra := byKey(extras, ExtraDivineSmiteExtra)
	if !ok || !okExtra {
		t.Fatalf("the smite and its extra die must be listed together: %v", extras)
	}
	if !smite.Available || smite.Selected || !smite.NeedsSlot {
		t.Fatalf("a smite is offered, never marked, and needs a slot: %+v", smite)
	}
	if !extra.Conditional || extra.Count != 1 || extra.Sides != 8 {
		t.Fatalf("the extra die is a conditional 1d8: %+v", extra)
	}
	for name, edit := range map[string]func(*ExtraScene){
		"a ranged attack":  func(s *ExtraScene) { s.Melee = false },
		"a spell attack":   func(s *ExtraScene) { s.Weapon = false },
		"no slot is free":  func(s *ExtraScene) { s.SmiteSlotFree = false },
		"a paladin of 1st": func(s *ExtraScene) { s.DivineSmite = false },
	} {
		s := base
		edit(&s)
		smite, ok := byKey(Offered(s), ExtraDivineSmite)
		if name == "a paladin of 1st" {
			if ok {
				t.Errorf("%s: listed", name)
			}
			continue
		}
		if !ok || smite.Available || smite.Reason == "" {
			t.Errorf("%s: want a disabled line with a reason, got %+v", name, smite)
		}
	}
}

func TestHuntersMarkAndColossusSlayerConditions(t *testing.T) {
	base := ExtraScene{
		Weapon: true, HuntersMarkKnown: true, HuntersMarkActive: true, TargetMarked: true,
		ColossusSlayer: true, TargetHurt: true, TargetState: "Ferido", TargetLabel: "Goblin 2",
	}
	extras := Offered(base)
	mark, _ := byKey(extras, ExtraHuntersMark)
	slayer, _ := byKey(extras, ExtraColossusSlayer)
	if !mark.Available || !mark.Selected || mark.Reason != "Goblin 2 é o alvo marcado · a sua concentração" {
		t.Fatalf("mark: %+v", mark)
	}
	if !slayer.Available || !slayer.Selected || slayer.Reason != "Uma vez por turno · alvo Ferido (abaixo do máximo de PV)" {
		t.Fatalf("slayer: %+v", slayer)
	}
	other := base
	other.TargetMarked, other.TargetHurt, other.TargetState, other.TargetLabel = false, false, "Ileso", "Goblin 1"
	extras = Offered(other)
	mark, _ = byKey(extras, ExtraHuntersMark)
	slayer, _ = byKey(extras, ExtraColossusSlayer)
	if mark.Available || mark.Reason != "Goblin 1 não é o alvo marcado." {
		t.Fatalf("mark on another target: %+v", mark)
	}
	if slayer.Available || slayer.Reason != "O alvo está Ileso: precisa estar abaixo do máximo de PV." {
		t.Fatalf("slayer on an unhurt target: %+v", slayer)
	}
	dropped := base
	dropped.HuntersMarkActive = false
	if mark, _ = byKey(Offered(dropped), ExtraHuntersMark); mark.Available || mark.Reason != "Sem a Marca do Caçador." {
		t.Fatalf("a dropped concentration: %+v", mark)
	}
	used := base
	used.ColossusUsed = true
	if slayer, _ = byKey(Offered(used), ExtraColossusSlayer); slayer.Available || slayer.Reason != "Já usado neste turno (uma vez por turno)." {
		t.Fatalf("slayer used: %+v", slayer)
	}
	spell := base
	spell.Weapon = false
	for _, key := range []string{ExtraHuntersMark, ExtraColossusSlayer} {
		if e, _ := byKey(Offered(spell), key); e.Available {
			t.Errorf("%s works on weapon attacks only", key)
		}
	}
}

func TestAutomaticLines(t *testing.T) {
	melee := ExtraScene{Weapon: true, Melee: true, UsesStrength: true, Raging: true, BarbarianLevel: 3}
	rage, ok := byKey(Automatic(melee), AutoRage)
	if !ok || rage.Flat != 2 || rage.Count != 0 || !rage.Auto {
		t.Fatalf("rage: %+v ok=%v", rage, ok)
	}
	for name, edit := range map[string]func(*ExtraScene){
		"not raging":         func(s *ExtraScene) { s.Raging = false },
		"a dexterity attack": func(s *ExtraScene) { s.UsesStrength = false },
		"a ranged attack":    func(s *ExtraScene) { s.Melee = false },
		"a spell":            func(s *ExtraScene) { s.Weapon = false },
	} {
		s := melee
		edit(&s)
		if _, ok := byKey(Automatic(s), AutoRage); ok {
			t.Errorf("%s: rage damage added", name)
		}
	}
	smite := ExtraScene{Weapon: true, Melee: true, ImprovedDivineSmite: true}
	if e, ok := byKey(Automatic(smite), AutoImprovedDivineSmite); !ok || e.Count != 1 || e.Sides != 8 {
		t.Fatalf("improved smite: %+v", e)
	}
	smite.Melee = false
	if _, ok := byKey(Automatic(smite), AutoImprovedDivineSmite); ok {
		t.Fatal("improved smite is for melee hits")
	}
}

func TestExtraDiceDoubleOnACriticalHitButFlatNumbersDoNot(t *testing.T) {
	sneak := Extra{Key: ExtraSneakAttack, Count: 3, Sides: 6}
	if c, f := ExtraDice(sneak, false, CriticalDoubledDice); c != 3 || f != 0 {
		t.Fatalf("normal: %d %d", c, f)
	}
	if c, f := ExtraDice(sneak, true, CriticalDoubledDice); c != 6 || f != 0 {
		t.Fatalf("critical, doubled dice: %d %d", c, f)
	}
	if c, f := ExtraDice(sneak, true, CriticalMaxPlusRoll); c != 3 || f != 18 {
		t.Fatalf("critical, maximum plus a roll: %d %d", c, f)
	}
	rage := Extra{Key: AutoRage, Flat: 2}
	if c, f := ExtraDice(rage, true, CriticalDoubledDice); c != 0 || f != 0 {
		t.Fatalf("a flat number rolls no dice and its Flat is not doubled: %d %d", c, f)
	}
	_ = rules.DiceFormula{}
}

func TestGreatWeaponFightingRerollsOnesAndTwosOnce(t *testing.T) {
	next := []int{9, 1, 7}
	roll := func(sides int) (int, error) {
		v := next[0]
		next = next[1:]
		return v, nil
	}
	kept, rerolls, err := GreatWeaponFighting([]int{2, 5, 1, 6}, 12, roll)
	if err != nil {
		t.Fatal(err)
	}
	// 2 -> 9, 1 -> 1 (the new roll is kept even when it is a 1).
	if !slices.Equal(kept, []int{9, 5, 1, 6}) {
		t.Fatalf("kept = %v", kept)
	}
	if len(rerolls) != 2 || rerolls[0] != (Reroll{Index: 0, From: 2, To: 9}) || rerolls[1] != (Reroll{Index: 2, From: 1, To: 1}) {
		t.Fatalf("rerolls = %v", rerolls)
	}
	boom := errors.New("boom")
	if _, _, err := GreatWeaponFighting([]int{1}, 8, func(int) (int, error) { return 0, boom }); !errors.Is(err, boom) {
		t.Fatalf("the roller's error must come back, got %v", err)
	}
}
