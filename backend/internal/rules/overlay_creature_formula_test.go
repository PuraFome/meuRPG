package rules

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// The formulas of a table creature. The names are invented; the bases are the SRD's
// giant spider (a bite with a DC 11 Constitution save, 1d8+3, +5 to hit) and wolf.

const (
	spiderKey  = "monster:giant-spider"
	swarmLord  = "creature:senhor-das-teias@mesa"
	spiderName = "Senhor das Teias"
)

// owner is a formula environment for a test: the level, the levels by class, the proficiency bonus and the
// ability modifiers (the missing ones are 0).
func owner(level, prof int, classes map[string]int, mods map[string]int) *formula.Env {
	return &formula.Env{
		Level:      func() int { return level },
		ClassLevel: func(c string) int { return classes[c] },
		Mod:        func(a string) int { return mods[a] },
		Score:      func(a string) int { return 10 + 2*mods[a] },
		Prof:       func() int { return prof },
		Armor:      func() string { return "none" },
		Shield:     func() bool { return false },
	}
}

func spiderLord() TableCreature {
	return TableCreature{
		TableEntry: TableEntry{Key: swarmLord, NamePT: spiderName}, BaseKey: spiderKey,
		HitPointsFormula:   `5 * classLevel("druid") + mod("con")`,
		ArmorClassFormula:  `10 + prof() + mod("dex")`,
		AttackBonusFormula: `prof() + mod("wis")`,
		DamageBonusFormula: `floor(level() / 4)`,
	}
}

// baseBiteDC is the DC of the giant spider's bite: no formula changes it.
const baseBiteDC = 11

func TestACreatureHasItsNumbersFromItsOwner(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, creatureOverlay(spiderLord()))
	base, _ := c.MonsterDerived(spiderKey)
	cases := []struct {
		name        string
		env         *formula.Env
		hp, ac, atk int
		damage      string
	}{
		{"level 5 druid", owner(5, 3, map[string]int{"druid": 5}, map[string]int{"con": 1, "dex": 2, "wis": 3}), 26, 15, 6, "1d8+4"},
		{"level 17 druid and wizard", owner(17, 6, map[string]int{"druid": 9, "wizard": 8}, map[string]int{"con": 3, "dex": 4, "wis": 5}), 48, 20, 11, "1d8+7"},
		{"negative modifiers", owner(3, 2, map[string]int{"druid": 1}, map[string]int{"con": -2, "dex": -1, "wis": -3}), 3, 11, -1, "1d8+3"},
		{"a different proficiency bonus alone", owner(5, 4, map[string]int{"druid": 5}, map[string]int{"con": 1, "dex": 2, "wis": 3}), 26, 16, 7, "1d8+4"},
	}
	for _, tc := range cases {
		r, ok := c.ResolveCreature(swarmLord, tc.env, 0)
		if !ok {
			t.Fatalf("%s: no creature", tc.name)
		}
		if r.HitPoints != tc.hp || r.ArmorClass != tc.ac || !r.HasAttackBonus || r.AttackBonus != tc.atk {
			t.Errorf("%s: resolved = %+v, want HP %d AC %d to-hit %d", tc.name, r, tc.hp, tc.ac, tc.atk)
		}
		d, plan, ok := c.ResolvedDerived(r)
		if !ok {
			t.Fatalf("%s: ResolvedDerived refused", tc.name)
		}
		if d.HitPointsMax != tc.hp || d.ArmorClass != tc.ac || d.ArmorClassDescription != "" {
			t.Errorf("%s: HP %d AC %d (%q), want %d and %d without a description", tc.name, d.HitPointsMax, d.ArmorClass, d.ArmorClassDescription, tc.hp, tc.ac)
		}
		if len(d.Attacks) != 2 || d.Attacks[0].AttackBonus != tc.atk || d.Attacks[1].AttackBonus != tc.atk {
			t.Errorf("%s: attacks = %+v, want every to-hit %d", tc.name, d.Attacks, tc.atk)
		}
		if got := d.Attacks[0].Damage; got != tc.damage {
			t.Errorf("%s: bite damage %s, want %s", tc.name, got, tc.damage)
		}
		if len(d.SaveActions) != 1 || d.SaveActions[0].DC != baseBiteDC || d.SaveActions[0].Key != swarmLord+"#bite" {
			t.Errorf("%s: save actions = %+v, want the bite at the base DC %d", tc.name, d.SaveActions, baseBiteDC)
		}
		ap, ok := plan.Action(swarmLord + "#bite")
		if !ok || ap.Attack == nil || ap.Attack.Bonus != tc.atk || ap.Save == nil || ap.Save.DC != baseBiteDC {
			t.Errorf("%s: plan = %+v", tc.name, ap)
		}
		if !strings.Contains(ap.Text, "DC "+strconv.Itoa(baseBiteDC)+" Constitution saving throw") {
			t.Errorf("%s: the text does not state DC %d: %s", tc.name, baseBiteDC, ap.Text)
		}
	}
	if base.HitPointsMax != 26 || base.Attacks[0].Damage != "1d8+3" {
		t.Errorf("the creature's own base changed: %+v", base.Attacks[0])
	}
}

func TestResolvedNumbersAreClamped(t *testing.T) {
	t.Parallel()
	mk := func(src string, edit func(*TableCreature, string)) ResolvedCreature {
		cr := TableCreature{TableEntry: TableEntry{Key: swarmLord, NamePT: spiderName}, BaseKey: spiderKey}
		edit(&cr, src)
		c := withOverlay(t, creatureOverlay(cr))
		r, ok := c.ResolveCreature(swarmLord, owner(1, 2, nil, nil), 0)
		if !ok {
			t.Fatal("no creature")
		}
		return r
	}
	hp := func(c *TableCreature, s string) { c.HitPointsFormula = s }
	ac := func(c *TableCreature, s string) { c.ArmorClassFormula = s }
	dmg := func(c *TableCreature, s string) { c.DamageBonusFormula = s }
	atk := func(c *TableCreature, s string) { c.AttackBonusFormula = s }
	if r := mk("0 - 50", hp); r.HitPoints != 1 {
		t.Errorf("HP low = %d, want 1", r.HitPoints)
	}
	if r := mk("1000 * 10", hp); r.HitPoints != MaxCreatureHitPoints {
		t.Errorf("HP high = %d, want %d", r.HitPoints, MaxCreatureHitPoints)
	}
	if r := mk("0", ac); r.ArmorClass != 1 {
		t.Errorf("AC low = %d, want 1", r.ArmorClass)
	}
	if r := mk("100", ac); r.ArmorClass != MaxCreatureArmorClass {
		t.Errorf("AC high = %d, want %d", r.ArmorClass, MaxCreatureArmorClass)
	}
	if r := mk("0 - 50", dmg); r.DamageBonus != MinCreatureDamageBonus {
		t.Errorf("damage low = %d", r.DamageBonus)
	}
	if r := mk("50", dmg); r.DamageBonus != MaxCreatureDamageBonus {
		t.Errorf("damage high = %d", r.DamageBonus)
	}
	if r := mk("0 - 50", atk); !r.HasAttackBonus || r.AttackBonus != MinCreatureAttackBonus {
		t.Errorf("to-hit low = %+v", r)
	}
	if r := mk("50", atk); r.AttackBonus != MaxCreatureAttackBonus {
		t.Errorf("to-hit high = %d", r.AttackBonus)
	}
	// A zero attack bonus is a bonus (+0), not "no formula".
	if r := mk("0", atk); !r.HasAttackBonus || r.AttackBonus != 0 {
		t.Errorf("a +0 to-hit = %+v, want HasAttackBonus", r)
	}
}

func TestACreatureWithNoFormulaIsItsStaticSelf(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, creatureOverlay(packLeaderCreature()))
	want, _ := c.MonsterDerived(packLeader)
	wantPlan, _ := c.MonsterPlan(packLeader)
	// Any owner, any slot: with no formula the result is the static stat block, byte for byte.
	for _, env := range []*formula.Env{owner(1, 2, nil, nil), owner(20, 6, map[string]int{"druid": 20}, map[string]int{"wis": 5})} {
		r, ok := c.ResolveCreature(packLeader, env, 3)
		if !ok {
			t.Fatal("no creature")
		}
		if r.HasAttackBonus || len(r.Failed) != 0 || r.HitPoints != 30 || r.ArmorClass != 15 || r.DamageBonus != 3 {
			t.Errorf("resolved = %+v, want the static values and no override", r)
		}
		got, plan, ok := c.ResolvedDerived(r)
		if !ok || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(plan, wantPlan) {
			t.Errorf("the resolved stat block differs from the static one (ok %v)", ok)
		}
	}
	// And the static block is the content's own: the wolf with 30 HP, AC 15 and +3 on the bite.
	if want.HitPointsMax != 30 || want.ArmorClass != 15 || want.Attacks[0].Damage != "2d4+5" {
		t.Errorf("static stat block = HP %d AC %d damage %s", want.HitPointsMax, want.ArmorClass, want.Attacks[0].Damage)
	}
	// A key that is not a table creature, and a missing owner, are not resolved.
	if _, ok := c.ResolveCreature(wolfKey, owner(1, 2, nil, nil), 0); ok {
		t.Error("an SRD creature resolved")
	}
	if _, ok := c.ResolveCreature(packLeader, nil, 0); ok {
		t.Error("a nil owner resolved")
	}
}

func TestAStaticValueAndAFormulaMix(t *testing.T) {
	t.Parallel()
	cr := packLeaderCreature() // HP 30, AC 15, damage +3, all static
	cr.ArmorClassFormula = `10 + prof() + mod("dex")`
	cr.DamageBonusFormula = `mod("str")`
	c := withOverlay(t, creatureOverlay(cr))
	r, _ := c.ResolveCreature(packLeader, owner(5, 3, nil, map[string]int{"dex": 2, "str": -2}), 0)
	if r.HitPoints != 30 || r.ArmorClass != 15 || r.DamageBonus != -2 || r.HasAttackBonus {
		t.Errorf("resolved = %+v, want static HP 30, AC formula 15, damage -2", r)
	}
	d, _, _ := c.ResolvedDerived(r)
	// The wolf's bite is 2d4+2: with -2 it is plain 2d4, the base's dice and not the static +3 on top.
	if d.Attacks[0].Damage != "2d4" {
		t.Errorf("bite damage = %s, want 2d4 (the formula replaces the static +3)", d.Attacks[0].Damage)
	}
	// A formula that fails when it runs leaves the static value and is named.
	cr.HitPointsFormula = `10 % (level() - 1)`
	c = withOverlay(t, creatureOverlay(cr))
	r, _ = c.ResolveCreature(packLeader, owner(1, 2, nil, nil), 0)
	if r.HitPoints != 30 || len(r.Failed) != 1 || r.Failed[0] != ".hit_points_formula" {
		t.Errorf("failed formula: HP %d, failed %v, want 30 and .hit_points_formula", r.HitPoints, r.Failed)
	}
}

func TestACreatureOwnAttacksAreActions(t *testing.T) {
	t.Parallel()
	cr := TableCreature{
		TableEntry: TableEntry{Key: swarmLord, NamePT: spiderName}, BaseKey: spiderKey,
		Attacks: []TableCreatureAttack{
			{NamePT: "Garra Venenosa", Melee: true, ReachFt: 10, DiceCount: 2, DiceSides: 6, DamageType: "damage-type:slashing", ToHit: `prof() + mod("str")`, DamageBonus: `mod("str")`},
			{NamePT: "Espinho", RangeFt: 30, LongRangeFt: 90, DiceCount: 1, DiceSides: 8, DamageType: "damage-type:piercing", ToHit: `prof() + slot()`},
			{NamePT: "Pancada", Melee: true, DiceCount: 1, DiceSides: 4, DamageType: "damage-type:bludgeoning"},
		},
	}
	c := withOverlay(t, creatureOverlay(cr))
	env := owner(5, 3, nil, map[string]int{"str": 4})
	r, ok := c.ResolveCreature(swarmLord, env, 2)
	if !ok || len(r.Attacks) != 3 || r.Attacks[0].AttackBonus != 7 || r.Attacks[0].DamageBonusValue != 4 ||
		r.Attacks[1].AttackBonus != 5 || r.Attacks[1].DamageBonusValue != 0 || r.Attacks[2].AttackBonus != 0 || r.Attacks[2].DamageBonusValue != 0 {
		t.Fatalf("resolved attacks = %+v", r.Attacks)
	}
	d, plan, _ := c.ResolvedDerived(r)
	byKey := map[string]Attack{}
	for _, a := range d.Attacks {
		byKey[a.Key] = a
	}
	claw, spike := byKey[swarmLord+"#garra-venenosa"], byKey[swarmLord+"#espinho"]
	if claw.Key == "" || spike.Key == "" || len(d.Attacks) != 5 {
		t.Fatalf("attacks = %+v, want the spider's two and the three own", d.Attacks)
	}
	if !claw.Melee || claw.AttackBonus != 7 || claw.Damage != "2d6+4" || claw.DamageType != "damage-type:slashing" || claw.ReachFt != 10 {
		t.Errorf("claw = %+v", claw)
	}
	if spike.Melee || spike.AttackBonus != 5 || spike.Damage != "1d8" || spike.RangeFt != 30 || spike.LongRangeFt != 90 {
		t.Errorf("spike = %+v", spike)
	}
	ap, ok := plan.Action(swarmLord + "#espinho")
	if !ok || ap.Kind != ActionKindAttack || ap.Attack == nil || ap.Attack.Bonus != 5 || ap.Attack.RangeFt != 30 || len(ap.Damage) != 1 {
		t.Errorf("plan of the spike = %+v", ap)
	}

	// The specific beats the general: with creature-level formulas the attacks that have their own keep them,
	// and the ones that do not (and the base's) get the creature's.
	cr.AttackBonusFormula, cr.DamageBonusFormula = "9", "2"
	c = withOverlay(t, creatureOverlay(cr))
	r, _ = c.ResolveCreature(swarmLord, env, 2)
	got := [][2]int{}
	for _, a := range r.Attacks {
		got = append(got, [2]int{a.AttackBonus, a.DamageBonusValue})
	}
	if want := [][2]int{{7, 4}, {5, 2}, {9, 2}}; !reflect.DeepEqual(got, want) {
		t.Errorf("own attacks (to-hit, damage) = %v, want %v", got, want)
	}
	d, _, _ = c.ResolvedDerived(r)
	if d.Attacks[0].AttackBonus != 9 || d.Attacks[0].Damage != "1d8+5" || d.Attacks[1].AttackBonus != 9 {
		t.Errorf("the base's attacks = %+v, want +9 and the bite 1d8+5", d.Attacks[:2])
	}

	// Without an owner no formula runs: the own attacks are +0 with the static damage bonus, the base's are the base's.
	cr.DamageBonus = 1
	c = withOverlay(t, creatureOverlay(cr))
	st, _ := c.MonsterDerived(swarmLord)
	if len(st.Attacks) != 5 || st.Attacks[0].AttackBonus != 5 || st.Attacks[0].Damage != "1d8+4" {
		t.Fatalf("static base attacks = %+v", st.Attacks[:2])
	}
	for i, dmg := range []string{"2d6+1", "1d8+1", "1d4+1"} {
		a := st.Attacks[2+i]
		if a.AttackBonus != 0 || a.Damage != dmg {
			t.Errorf("static own attack %d = to-hit %d damage %s, want +0 and %s", i, a.AttackBonus, a.Damage, dmg)
		}
	}
}

func TestSlotIsLegalOnlyInACreature(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	cr := TableCreature{TableEntry: TableEntry{Key: swarmLord, NamePT: spiderName}, BaseKey: spiderKey, HitPointsFormula: `10 + 5 * slot()`, DamageBonusFormula: `slot()`}
	c := withOverlayOn(t, srd, creatureOverlay(cr))
	for slot, want := range map[int]int{0: 10, 1: 15, 3: 25, 9: 55, -1: 10, 10: 10, 99: 10} {
		r, _ := c.ResolveCreature(swarmLord, owner(5, 3, nil, nil), slot)
		if r.HitPoints != want {
			t.Errorf("slot %d: HP %d, want %d", slot, r.HitPoints, want)
		}
	}
	// The damage bonus formula reads the slot too, clamped to its range.
	for slot, want := range map[int]int{0: 0, 4: 4, 9: 9} {
		r, _ := c.ResolveCreature(swarmLord, owner(5, 3, nil, nil), slot)
		if r.DamageBonus != want {
			t.Errorf("slot %d: damage bonus %d, want %d", slot, r.DamageBonus, want)
		}
	}
	// An effect (a number) and a condition refuse it; so does the plain compiler.
	for _, e := range []Effect{
		{Type: "modifier", Target: "ac", Mode: "add", Value: "slot()"},
		{Type: "modifier", Target: "ac", Mode: "add", Value: "1", When: "slot() > 1"},
	} {
		_, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{classWithEffects([]Effect{e})}})
		if err == nil || !strings.Contains(err.Error(), "slot()") {
			t.Errorf("effect %+v: err = %v, want slot() refused", e, err)
		}
	}
	// The caster's prepared-spells formula refuses it, and accepts a plain one.
	prepared := func(src string) error {
		var tc TableClass
		for _, tab := range srd.TableDefaults().Tables {
			if tab.Kind == CastingFull && tab.Preparation == PreparationPrepared {
				tc = classFromDefaults(tab, "preparador")
			}
		}
		tc.Casting.PreparedMax = src
		_, err := srd.With(Overlay{Revision: 1, Classes: []TableClass{tc}})
		return err
	}
	if err := prepared("level()"); err != nil {
		t.Errorf("a prepared-spells formula of level(): %v", err)
	}
	if err := prepared("slot()"); err == nil || !strings.Contains(err.Error(), "slot()") {
		t.Errorf("slot() in a prepared-spells formula: err = %v, want it refused naming slot()", err)
	}
}

func TestACreatureFormulaRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	atk := func(n string) TableCreatureAttack {
		return TableCreatureAttack{NamePT: n, Melee: true, DiceCount: 1, DiceSides: 6, DamageType: "damage-type:slashing"}
	}
	three := func(edit func(*TableCreatureAttack)) []TableCreatureAttack {
		a := []TableCreatureAttack{atk("Um"), atk("Dois"), atk("Tres")}
		edit(&a[2])
		return a
	}
	tooMany := make([]TableCreatureAttack, MaxCreatureAttacks+1)
	for i := range tooMany {
		tooMany[i] = atk("Golpe " + string(rune('a'+i)))
	}
	cases := []struct {
		name          string
		edit          func(c *TableCreature)
		field, reason string
	}{
		{"a formula that does not parse", func(c *TableCreature) { c.HitPointsFormula = "1 +" }, ".hit_points_formula", ReasonFormula},
		{"an unknown function", func(c *TableCreature) { c.ArmorClassFormula = "wisdom()" }, ".armor_class_formula", ReasonFormula},
		{"an illegal argument", func(c *TableCreature) { c.DamageBonusFormula = `mod("xyz")` }, ".damage_bonus_formula", ReasonFormula},
		{"a state in a number", func(c *TableCreature) { c.AttackBonusFormula = `active("x")` }, ".attack_bonus_formula", ReasonFormula},
		{"a condition as a number", func(c *TableCreature) { c.ArmorClassFormula = `armor() == "none"` }, ".armor_class_formula", ReasonFormula},
		{"a formula of 1201 bytes", func(c *TableCreature) { c.DamageBonusFormula = strings.Repeat("1+", 600) + "1" }, ".damage_bonus_formula", ReasonFormula},
		{"a bad to_hit", func(c *TableCreature) { c.Attacks = three(func(a *TableCreatureAttack) { a.ToHit = "nope(" }) }, ".attacks[2].to_hit", ReasonFormula},
		{"a bad damage_bonus", func(c *TableCreature) { c.Attacks = three(func(a *TableCreatureAttack) { a.DamageBonus = "level" }) }, ".attacks[2].damage_bonus", ReasonFormula},
		{"nine attacks", func(c *TableCreature) { c.Attacks = tooMany }, ".attacks", ReasonLimit},
		{"an attack name", func(c *TableCreature) { c.Attacks = three(func(a *TableCreatureAttack) { a.NamePT = "" }) }, ".attacks[2].name_pt", ReasonName},
		{"an attack named as a base action", func(c *TableCreature) { c.Attacks = three(func(a *TableCreatureAttack) { a.NamePT = "Bite" }) }, ".attacks[2].name_pt", ReasonName},
		{"an attack named as a trait", func(c *TableCreature) {
			c.Traits = []TableCreatureTrait{{NamePT: "Dois", TextPT: "x"}}
			c.Attacks = []TableCreatureAttack{atk("dois")}
		}, ".attacks[0].name_pt", ReasonName},
		{"a reach too long", func(c *TableCreature) { c.Attacks = three(func(a *TableCreatureAttack) { a.ReachFt = 31 }) }, ".attacks[2].reach_ft", ReasonValue},
		{"a range on a melee attack", func(c *TableCreature) { c.Attacks = three(func(a *TableCreatureAttack) { a.RangeFt = 30 }) }, ".attacks[2].range_ft", ReasonValue},
		{"a ranged attack with no range", func(c *TableCreature) { c.Attacks = three(func(a *TableCreatureAttack) { a.Melee = false }) }, ".attacks[2].range_ft", ReasonValue},
		{"a long range under the range", func(c *TableCreature) {
			c.Attacks = three(func(a *TableCreatureAttack) { a.Melee, a.RangeFt, a.LongRangeFt = false, 60, 30 })
		}, ".attacks[2].long_range_ft", ReasonValue},
		{"a name repeating another attack's", func(c *TableCreature) { c.Attacks = three(func(a *TableCreatureAttack) { a.NamePT = "um" }) }, ".attacks[2].name_pt", ReasonName},
		{"an SRD damage type key that does not exist", func(c *TableCreature) {
			c.Attacks = three(func(a *TableCreatureAttack) { a.DamageType = "damage-type:nope" })
		}, ".attacks[2].damage_type", ReasonReference},
		{"a long range of 1201", func(c *TableCreature) {
			c.Attacks = three(func(a *TableCreatureAttack) { a.Melee, a.RangeFt, a.LongRangeFt = false, 60, 1201 })
		}, ".attacks[2].long_range_ft", ReasonValue},
		{"a reach on a ranged attack", func(c *TableCreature) {
			c.Attacks = three(func(a *TableCreatureAttack) { a.Melee, a.RangeFt, a.ReachFt = false, 30, 5 })
		}, ".attacks[2].reach_ft", ReasonValue},
		{"no dice", func(c *TableCreature) { c.Attacks = three(func(a *TableCreatureAttack) { a.DiceCount = 0 }) }, ".attacks[2].dice_count", ReasonValue},
		{"a d7", func(c *TableCreature) { c.Attacks = three(func(a *TableCreatureAttack) { a.DiceSides = 7 }) }, ".attacks[2].dice_sides", ReasonValue},
		{"a damage type that is not one", func(c *TableCreature) { c.Attacks = three(func(a *TableCreatureAttack) { a.DamageType = "slashing" }) }, ".attacks[2].damage_type", ReasonReference},
	}
	for _, tc := range cases {
		cr := TableCreature{TableEntry: TableEntry{Key: swarmLord, NamePT: spiderName}, BaseKey: spiderKey}
		tc.edit(&cr)
		_, err := srd.With(creatureOverlay(cr))
		oe, ok := errors.AsType[*OverlayError](err)
		if !ok {
			t.Errorf("%s: err = %v, want an overlay error", tc.name, err)
			continue
		}
		found := false
		for _, v := range oe.Violations() {
			found = found || strings.HasSuffix(v.Field, tc.field) && v.Reason == tc.reason
		}
		if !found {
			t.Errorf("%s: refused at %+v, want %s (%s)", tc.name, oe.Violations(), tc.field, tc.reason)
		}
	}
	// Eight attacks are fine.
	cr := TableCreature{TableEntry: TableEntry{Key: swarmLord, NamePT: spiderName}, BaseKey: spiderKey, Attacks: tooMany[:MaxCreatureAttacks]}
	if _, err := srd.With(creatureOverlay(cr)); err != nil {
		t.Errorf("8 attacks: %v", err)
	}
}

func TestCreatureFormulasShareTheOverlayFormulaBudget(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	var list []TableCreature
	for i := range MaxOverlayFormulas + 1 {
		list = append(list, TableCreature{
			TableEntry: TableEntry{Key: "creature:c" + itoa(i) + tableSuffix, NamePT: "Criatura " + itoa(i)}, BaseKey: wolfKey,
			HitPointsFormula: "10 + " + itoa(i),
		})
	}
	_, err := srd.With(creatureOverlay(list...))
	if oe, ok := errors.AsType[*OverlayError](err); !ok || oe.Reason != ReasonLimit {
		t.Errorf("%d distinct formulas: err = %v, want the limit", len(list), err)
	}
	// The same formula in every creature is compiled once and counts once.
	for i := range list {
		list[i].HitPointsFormula = "10 + prof()"
	}
	if _, err := srd.With(creatureOverlay(list...)); err != nil {
		t.Errorf("one formula in %d creatures: %v", len(list), err)
	}
}

func TestOwnerEnvReadsADerivedSheet(t *testing.T) {
	t.Parallel()
	d := Derived{
		TotalLevel: 7, ProficiencyBonus: 3, SpeedWalkFt: 30, ArmorCategory: "light",
		Classes: []DerivedClass{{ClassKey: "class:druid", Level: 4}, {ClassKey: "class:wizard", Level: 3}},
		Abilities: []AbilityScore{
			{Ability: WIS, Score: 8, Modifier: -1}, {Ability: CON, Score: 14, Modifier: 2},
		},
	}
	c := withOverlay(t, creatureOverlay(TableCreature{
		TableEntry: TableEntry{Key: swarmLord, NamePT: spiderName}, BaseKey: spiderKey,
		HitPointsFormula:  `level() + classLevel("druid") * 2 + classLevel("wizard") + mod("con") + mod("wis")`,
		ArmorClassFormula: `10 + prof() + floor(score("wis") / 4)`,
	}))
	r, ok := c.ResolveCreature(swarmLord, OwnerEnv(d, false), 0)
	// 7 + 8 + 3 + 2 - 1 = 19 and 10 + 3 + 2 = 15.
	if !ok || r.HitPoints != 19 || r.ArmorClass != 15 {
		t.Errorf("resolved = %+v, want HP 19 AC 15", r)
	}
}

func TestSlotInTheFormulaCompiler(t *testing.T) {
	t.Parallel()
	plain := formula.NewCompiler([]string{"wizard"})
	if _, err := plain.Compile("slot()", formula.Int); err == nil {
		t.Error("the plain compiler accepts slot()")
	}
	p, err := plain.WithSlot().Compile("slot() * 2 + 1", formula.Int)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Int(&formula.Env{Slot: func() int { return 4 }})
	if err != nil || got != 9 {
		t.Errorf("slot() * 2 + 1 at 4 = %d, %v", got, err)
	}
	if _, err := plain.WithSlot().WithStates().Compile(`active("x") and slot() > 1`, formula.Bool); err != nil {
		t.Errorf("both gates: %v", err)
	}
	if _, err := plain.WithSlot().Compile("slot(1)", formula.Int); err == nil {
		t.Error("slot(1) accepted")
	}
}

// TestClampEdges: one creature whose four numbers are level(), run for owners whose level is each edge of the
// ranges, one past it and far past it.
func TestClampEdges(t *testing.T) {
	t.Parallel()
	cr := TableCreature{
		TableEntry: TableEntry{Key: swarmLord, NamePT: spiderName}, BaseKey: spiderKey,
		HitPointsFormula: "level()", ArmorClassFormula: "level()", DamageBonusFormula: "level()",
		AttackBonusFormula: "level()",
	}
	c := withOverlay(t, creatureOverlay(cr))
	limits := []struct {
		name   string
		get    func(ResolvedCreature) int
		lo, hi int
	}{
		{"hit points", func(r ResolvedCreature) int { return r.HitPoints }, 1, 9999},
		{"armor class", func(r ResolvedCreature) int { return r.ArmorClass }, 1, 40},
		{"damage bonus", func(r ResolvedCreature) int { return r.DamageBonus }, -10, 30},
		{"attack bonus", func(r ResolvedCreature) int { return r.AttackBonus }, -10, 30},
	}
	levels := []int{-10000, -11, -10, -1, 0, 1, 2, 29, 30, 31, 39, 40, 41, 9998, 9999, 10000}
	for _, level := range levels {
		r, ok := c.ResolveCreature(swarmLord, owner(level, 2, nil, nil), 0)
		if !ok || len(r.Failed) != 0 {
			t.Fatalf("level %d: ok %v, failed %v", level, ok, r.Failed)
		}
		for _, l := range limits {
			if got, want := l.get(r), min(max(level, l.lo), l.hi); got != want {
				t.Errorf("%s at %d = %d, want %d", l.name, level, got, want)
			}
		}
	}
	// The ranges themselves, written out.
	for level, want := range map[int][4]int{
		-11: {1, 1, -10, -10}, 0: {1, 1, 0, 0}, 1: {1, 1, 1, 1}, 31: {31, 31, 30, 30},
		41: {41, 40, 30, 30}, 9999: {9999, 40, 30, 30}, 10000: {9999, 40, 30, 30},
	} {
		r, _ := c.ResolveCreature(swarmLord, owner(level, 2, nil, nil), 0)
		if got := [4]int{r.HitPoints, r.ArmorClass, r.DamageBonus, r.AttackBonus}; got != want {
			t.Errorf("level %d: %v, want %v", level, got, want)
		}
	}
}

// TestANoFormulaCreatureWithASaveKeepsItsBaseDC: a creature with only static adjustments, on a base whose action
// has a saving throw, has the base's own stat block with the adjustments. The expectation is built here by hand from the SRD
// monster, not by ApplyResolved.
func TestANoFormulaCreatureWithASaveKeepsItsBaseDC(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	cr := TableCreature{
		TableEntry: TableEntry{Key: swarmLord, NamePT: spiderName}, BaseKey: spiderKey,
		HitPoints: 40, ArmorClass: 16, DamageBonus: 2,
		Traits: []TableCreatureTrait{{NamePT: "Teia Densa", TextPT: "Um texto."}},
	}
	c := withOverlayOn(t, srd, creatureOverlay(cr))
	m := *srd.c.monsters[spiderKey]
	m.Key, m.Name = swarmLord, spiderName
	m.HitPoints, m.HitPointsRoll = 40, ""
	m.ArmorClass, m.ArmorClassType, m.ArmorClassDesc, m.ArmorClassItems, m.ArmorClassAlts = 16, "", "", nil, nil
	m.Actions = append([]srd51.MonsterAction(nil), m.Actions...)
	m.Actions[0].Damage = append([]srd51.MonsterDamage(nil), m.Actions[0].Damage...)
	m.Actions[0].Damage[0].Dice = "1d8+5"
	m.SpecialAbilities = append(append([]srd51.MonsterAbility(nil), m.SpecialAbilities...), srd51.MonsterAbility{Name: "Teia Densa", Desc: "Um texto."})
	want := c.c.monsterDerived(&m)
	wantPlan, err := c.c.planOf(&m)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := c.MonsterDerived(swarmLord)
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("the derived block differs from the one built independently (ok %v)", ok)
	}
	if plan, _ := c.MonsterPlan(swarmLord); !reflect.DeepEqual(plan, wantPlan) {
		t.Error("the plan differs from the one built independently")
	}
	if len(got.SaveActions) != 1 || got.SaveActions[0].DC != 11 {
		t.Errorf("save actions = %+v, want the bite at DC 11", got.SaveActions)
	}
	// And any owner resolves to the same.
	r, _ := c.ResolveCreature(swarmLord, owner(9, 4, nil, nil), 5)
	if d, _, ok := c.ResolvedDerived(r); !ok || !reflect.DeepEqual(d, want) {
		t.Errorf("a resolved block differs (ok %v)", ok)
	}
}

func TestResolvedDerivedTrustsTheContentNotTheValue(t *testing.T) {
	t.Parallel()
	cr := spiderLord()
	cr.Traits = []TableCreatureTrait{{NamePT: "Teia Densa", TextPT: "Um texto."}}
	cr.Attacks = []TableCreatureAttack{{NamePT: "Pancada", Melee: true, DiceCount: 1, DiceSides: 4, DamageType: "damage-type:bludgeoning"}}
	c := withOverlay(t, creatureOverlay(cr))
	env := owner(5, 3, map[string]int{"druid": 5}, nil)
	good, _ := c.ResolveCreature(swarmLord, env, 0)
	want, wantPlan, _ := c.ResolvedDerived(good)

	forged := good
	forged.BaseKey, forged.NamePT, forged.Traits = wolfKey, "Outro", nil
	forged.Attacks = []ResolvedAttack{good.Attacks[0]}
	forged.Attacks[0].DiceSides, forged.Attacks[0].DamageType, forged.Attacks[0].NamePT = 20, "damage-type:fire", "Falso"
	got, plan, ok := c.ResolvedDerived(forged)
	if !ok || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(plan, wantPlan) {
		t.Errorf("a forged identity changed the stat block (ok %v)", ok)
	}
	// Forged numbers are clamped.
	wild := good
	wild.HitPoints, wild.ArmorClass, wild.DamageBonus = 1<<30, -5, 1000
	wild.Attacks = []ResolvedAttack{good.Attacks[0]}
	wild.Attacks[0].AttackBonus = 99
	if d, _, ok := c.ResolvedDerived(wild); !ok || d.HitPointsMax != MaxCreatureHitPoints || d.ArmorClass != 1 || d.Attacks[2].AttackBonus != MaxCreatureAttackBonus {
		t.Errorf("forged numbers: HP %d AC %d (ok %v)", d.HitPointsMax, d.ArmorClass, ok)
	}
	// A creature the content does not have is refused.
	other := good
	other.Key = wolfKey
	if _, _, ok := c.ResolvedDerived(other); ok {
		t.Error("an SRD creature resolved")
	}
	// The traits of a resolved value are a copy.
	good.Traits[0].NamePT = "Mexido"
	again, _ := c.ResolveCreature(swarmLord, env, 0)
	if again.Traits[0].NamePT != "Teia Densa" {
		t.Errorf("a trait written through a resolved value changed the content: %q", again.Traits[0].NamePT)
	}
}

func twoAttacks(names ...string) TableCreature {
	cr := spiderLord()
	for i, n := range names {
		cr.Attacks = append(cr.Attacks, TableCreatureAttack{
			NamePT: n, Melee: true, DiceCount: 1, DiceSides: 4, DamageType: "damage-type:piercing",
			ToHit: fmt.Sprintf("%d + level()", i*10),
		})
	}
	return cr
}

func attackNumbers(t *testing.T, d Derived) map[string]int {
	t.Helper()
	got := map[string]int{}
	for _, a := range d.Attacks {
		got[a.Name] = a.AttackBonus
	}
	return got
}

func TestASnapshotSurvivesAnEditOfTheOwnAttacks(t *testing.T) {
	t.Parallel()
	env := owner(5, 3, map[string]int{"druid": 5}, map[string]int{"wis": 2})
	snap, ok := withOverlay(t, creatureOverlay(twoAttacks("Mordida", "Ferrão"))).ResolveCreature(swarmLord, env, 0)
	if !ok || len(snap.Attacks) != 2 || snap.Attacks[0].Key != swarmLord+"#mordida" {
		t.Fatalf("snapshot = %+v (ok %v)", snap.Attacks, ok)
	}
	// "Mordida" is +5 and "Ferrão" is +15 for this owner; the wolf-like base has attacks of its own.
	cases := []struct {
		name    string
		edited  []string
		want    map[string]int
		wantLen int
	}{
		{"unchanged", []string{"Mordida", "Ferrão"}, map[string]int{"Mordida": 5, "Ferrão": 15}, 2},
		{"reordered", []string{"Ferrão", "Mordida"}, map[string]int{"Mordida": 5, "Ferrão": 15}, 2},
		{"one added", []string{"Mordida", "Ferrão", "Garra"}, map[string]int{"Mordida": 5, "Ferrão": 15, "Garra": 0}, 3},
		{"one added first", []string{"Garra", "Mordida", "Ferrão"}, map[string]int{"Mordida": 5, "Ferrão": 15, "Garra": 0}, 3},
		{"one removed", []string{"Ferrão"}, map[string]int{"Ferrão": 15}, 1},
		{"all removed", nil, map[string]int{}, 0},
		{"one renamed", []string{"Mordida", "Picada"}, map[string]int{"Mordida": 5, "Picada": 0}, 2},
	}
	for _, tc := range cases {
		c := withOverlay(t, creatureOverlay(twoAttacks(tc.edited...)))
		d, _, ok := c.ResolvedDerived(snap)
		if !ok {
			t.Errorf("%s: not resolved", tc.name)
			continue
		}
		own := map[string]int{}
		for name, bonus := range attackNumbers(t, d) {
			if slices.ContainsFunc(tc.edited, func(n string) bool { return n == name }) {
				own[name] = bonus
			}
		}
		if !reflect.DeepEqual(own, tc.want) {
			t.Errorf("%s: own attacks = %v, want %v", tc.name, own, tc.want)
		}
		// The creature's own numbers never move with the attack list.
		if d.HitPointsMax != snap.HitPoints || d.ArmorClass != snap.ArmorClass {
			t.Errorf("%s: HP %d AC %d, want %d %d", tc.name, d.HitPointsMax, d.ArmorClass, snap.HitPoints, snap.ArmorClass)
		}
	}
}

func TestAMalformedSnapshotOfAttacksIsIgnoredOrClamped(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, creatureOverlay(twoAttacks("Mordida", "Ferrão")))
	good, _ := c.ResolveCreature(swarmLord, owner(5, 3, map[string]int{"druid": 5}, nil), 0)
	bonus := func(r ResolvedCreature) map[string]int {
		d, _, ok := c.ResolvedDerived(r)
		if !ok {
			t.Fatalf("not resolved: %+v", r.Attacks)
		}
		return attackNumbers(t, d)
	}
	// Keys that are not the entry's are ignored: both attacks have the static numbers.
	forged := good
	forged.Attacks = []ResolvedAttack{
		{Key: "creature:other#mordida", AttackBonus: 25}, {Key: "", AttackBonus: 25}, {Key: swarmLord + "#../x", AttackBonus: 25},
	}
	if got := bonus(forged); got["Mordida"] != 0 || got["Ferrão"] != 0 {
		t.Errorf("forged keys were used: %v", got)
	}
	// Out-of-range numbers are clamped; of two attacks with one key the first counts.
	wild := good
	wild.Attacks = []ResolvedAttack{
		{Key: swarmLord + "#mordida", AttackBonus: 1 << 40, DamageBonusValue: -(1 << 40)},
		{Key: swarmLord + "#mordida", AttackBonus: -7},
		{Key: swarmLord + "#ferr-o", AttackBonus: -1 << 40, DamageBonusValue: 1 << 40},
	}
	d, _, ok := c.ResolvedDerived(wild)
	if !ok {
		t.Fatal("a wild snapshot did not resolve")
	}
	got := attackNumbers(t, d)
	if got["Mordida"] != MaxCreatureAttackBonus || got["Ferrão"] != MinCreatureAttackBonus {
		t.Errorf("wild numbers = %v", got)
	}
	// Many attacks, more than the entry has, do not crash.
	many := good
	for range 100 {
		many.Attacks = append(many.Attacks, good.Attacks...)
	}
	if _, _, ok := c.ResolvedDerived(many); !ok {
		t.Error("a snapshot with many attacks did not resolve")
	}
}

func TestADamageTypeInAnErrorIsShort(t *testing.T) {
	t.Parallel()
	cr := TableCreature{
		TableEntry: TableEntry{Key: swarmLord, NamePT: spiderName}, BaseKey: spiderKey,
		Attacks: []TableCreatureAttack{{NamePT: "Um", Melee: true, DiceCount: 1, DiceSides: 6, DamageType: strings.Repeat("x", 500)}},
	}
	_, err := loadForTest(t).With(creatureOverlay(cr))
	if err == nil || strings.Contains(err.Error(), strings.Repeat("x", 100)) {
		t.Errorf("err = %v, want one without the 500 characters", err)
	}
}

// TestAFormulaBeyondTheEngineLimitFailsAndKeepsTheStaticValue: a result past formula.MaxResult is a failed
// formula (the field keeps its static value or the base's); within the limit it is clamped to the field's range.
func TestAFormulaBeyondTheEngineLimitFailsAndKeepsTheStaticValue(t *testing.T) {
	t.Parallel()
	cr := TableCreature{
		TableEntry: TableEntry{Key: swarmLord, NamePT: spiderName}, BaseKey: spiderKey, HitPoints: 40, ArmorClass: 14, DamageBonus: 2,
		HitPointsFormula: "level() * 1000 * 2", ArmorClassFormula: "level() * 1000 * 2", DamageBonusFormula: "level() * 1000 * 2", AttackBonusFormula: "level() * 1000 * 2",
	}
	c := withOverlay(t, creatureOverlay(cr))
	r, _ := c.ResolveCreature(swarmLord, owner(6, 2, nil, nil), 0)
	if r.HitPoints != 40 || r.ArmorClass != 14 || r.DamageBonus != 2 || r.HasAttackBonus || len(r.Failed) != 4 {
		t.Errorf("12000 resolved to %+v, want the static values and 4 failed fields", r)
	}
	r, _ = c.ResolveCreature(swarmLord, owner(5, 2, nil, nil), 0)
	if r.HitPoints != MaxCreatureHitPoints || r.ArmorClass != MaxCreatureArmorClass || r.DamageBonus != MaxCreatureDamageBonus || r.AttackBonus != MaxCreatureAttackBonus || len(r.Failed) != 0 {
		t.Errorf("10000 resolved to %+v, want the field maxima", r)
	}
}
