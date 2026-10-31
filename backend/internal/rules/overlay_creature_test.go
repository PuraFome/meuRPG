package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// The table's creatures. The names are invented; the base is the SRD's wolf.

const (
	packLeader = "creature:lobo-alfa@mesa"
	wolfKey    = "monster:wolf"
)

func packLeaderCreature() TableCreature {
	return TableCreature{
		TableEntry: TableEntry{Key: packLeader, NamePT: "Lobo Alfa"}, BaseKey: wolfKey,
		HitPoints: 30, ArmorClass: 15, DamageBonus: 3,
		Traits: []TableCreatureTrait{{NamePT: "Uivo de Comando", TextPT: "Aliados a 30 pés ouvem o uivo."}},
	}
}

func creatureOverlay(cs ...TableCreature) Overlay { return Overlay{Revision: 1, Creatures: cs} }

func TestACreatureHasTheAdjustmentsInItsDerived(t *testing.T) {
	srd := loadForTest(t)
	c := withOverlayOn(t, srd, creatureOverlay(packLeaderCreature()))
	base, _ := c.MonsterDerived(wolfKey)
	d, ok := c.MonsterDerived(packLeader)
	if !ok {
		t.Fatal("MonsterDerived does not know the table creature")
	}
	if d.HitPointsMax != 30 || d.ArmorClass != 15 {
		t.Errorf("HP %d AC %d, want 30 and 15", d.HitPointsMax, d.ArmorClass)
	}
	if base.HitPointsMax != 11 || base.ArmorClass != 13 {
		t.Errorf("the base wolf changed: HP %d AC %d", base.HitPointsMax, base.ArmorClass)
	}
	if len(d.Attacks) != 1 || d.Attacks[0].Damage != "2d4+5" || d.Attacks[0].Key != packLeader+"#bite" {
		t.Errorf("attacks = %+v, want the bite with 2d4+5", d.Attacks)
	}
	if got := d.Attacks[0].DamageDice; got.Count != 2 || got.Sides != 4 || got.Bonus != 5 {
		t.Errorf("DamageDice = %+v, want 2d4+5", got)
	}
	if d.Attacks[0].AttackBonus != base.Attacks[0].AttackBonus {
		t.Error("the attack bonus changed")
	}
	if d.SpeedWalkFt != base.SpeedWalkFt || len(d.Abilities) != len(base.Abilities) {
		t.Error("what is not adjusted differs from the base")
	}
	// The traits come after the base's, as text.
	var names []string
	for _, f := range d.Features {
		names = append(names, f.Name)
	}
	if want := append(featureNames(base), "Uivo de Comando"); !slices.Equal(names, want) {
		t.Errorf("features = %v, want %v", names, want)
	}
	last := d.Features[len(d.Features)-1]
	if last.Key != packLeader+"#uivo-de-comando" || last.Description[0] != "Aliados a 30 pés ouvem o uivo." || last.SourcePT != "Lobo Alfa" {
		t.Errorf("trait = %+v", last)
	}
	// The plan reads the adjusted damage too.
	ap, ok := c.MonsterActionPlan(packLeader + "#bite")
	if !ok || len(ap.Damage) != 1 || ap.Damage[0].DiceText != "2d4+5" {
		t.Errorf("plan = %+v, %v, want the bite with 2d4+5", ap, ok)
	}
	// The extra attacks of a Multiattack and the per-action count are the base's.
	if d.AttacksPerAction != base.AttacksPerAction {
		t.Errorf("AttacksPerAction = %d, want %d", d.AttacksPerAction, base.AttacksPerAction)
	}
}

func featureNames(d Derived) []string {
	var out []string
	for _, f := range d.Features {
		out = append(out, f.Name)
	}
	return out
}

func TestEachAdjustmentWorksAlone(t *testing.T) {
	srd := loadForTest(t)
	base, _ := srd.MonsterDerived(wolfKey)
	mk := func(edit func(*TableCreature)) Derived {
		cr := TableCreature{TableEntry: TableEntry{Key: packLeader, NamePT: "Lobo Alfa"}, BaseKey: wolfKey}
		edit(&cr)
		d, ok := withOverlayOn(t, srd, creatureOverlay(cr)).MonsterDerived(packLeader)
		if !ok {
			t.Fatal("no creature")
		}
		return d
	}
	if d := mk(func(*TableCreature) {}); d.HitPointsMax != base.HitPointsMax || d.ArmorClass != base.ArmorClass || d.Attacks[0].Damage != "2d4+2" || len(d.Features) != len(base.Features) {
		t.Errorf("no adjustment: HP %d AC %d damage %s features %d, want the base's", d.HitPointsMax, d.ArmorClass, d.Attacks[0].Damage, len(d.Features))
	}
	if d := mk(func(c *TableCreature) { c.HitPoints = 1 }); d.HitPointsMax != 1 || d.ArmorClass != base.ArmorClass {
		t.Errorf("hit points only: HP %d AC %d", d.HitPointsMax, d.ArmorClass)
	}
	if d := mk(func(c *TableCreature) { c.ArmorClass = 40 }); d.ArmorClass != 40 || d.HitPointsMax != base.HitPointsMax {
		t.Errorf("armor class only: HP %d AC %d", d.HitPointsMax, d.ArmorClass)
	}
	if d := mk(func(c *TableCreature) { c.DamageBonus = -10 }); d.Attacks[0].Damage != "2d4-8" {
		t.Errorf("negative bonus: damage %s, want 2d4-8", d.Attacks[0].Damage)
	}
	// Flat damage (the raven's beak is 1) stays a number.
	cr := TableCreature{TableEntry: TableEntry{Key: packLeader, NamePT: "Corvo Grande"}, BaseKey: "monster:raven", DamageBonus: 2}
	if d, _ := withOverlayOn(t, srd, creatureOverlay(cr)).MonsterDerived(packLeader); d.Attacks[0].Damage != "3" {
		t.Errorf("flat damage = %s, want 3", d.Attacks[0].Damage)
	}
}

func TestACreatureResolvesWhereAnSRDCreatureDoes(t *testing.T) {
	srd := loadForTest(t)
	c := withOverlayOn(t, srd, creatureOverlay(packLeaderCreature()))
	cr, ok := c.CreatureByKey(packLeader)
	if !ok || cr.NamePT != "Lobo Alfa" || cr.HitPoints != 30 || cr.ArmorClass != 15 || cr.Size != "Medium" || cr.Type != "beast" {
		t.Errorf("CreatureByKey = %+v, %v", cr.CreatureEntry, ok)
	}
	if len(cr.Traits) == 0 || cr.Traits[len(cr.Traits)-1].Name != "Uivo de Comando" {
		t.Errorf("traits = %+v, want the table's last", cr.Traits)
	}
	if len(cr.Actions) == 0 || cr.Actions[0].Plan == nil {
		t.Error("the creature's action has no plan")
	}
	if c.c.namePT(packLeader) != "Lobo Alfa" {
		t.Errorf("namePT = %q", c.c.namePT(packLeader))
	}
	list, err := c.ListCreatures(CreatureFilter{Query: "lobo alfa"})
	if err != nil || len(list) != 1 || list[0].Key != packLeader || list[0].HitPoints != 30 {
		t.Errorf("ListCreatures = %+v, %v, want the table creature", list, err)
	}
	all, _ := c.ListCreatures(CreatureFilter{})
	plain, _ := srd.ListCreatures(CreatureFilter{})
	if len(all) != len(plain)+1 {
		t.Errorf("list has %d creatures, want %d", len(all), len(plain)+1)
	}
	if !slices.IsSortedFunc(all, func(a, b CreatureEntry) int { return comparePT(a.NamePT, b.NamePT) }) {
		t.Error("the list is not sorted by Portuguese name")
	}
	if !c.HasCreature(packLeader) {
		t.Error("HasCreature = false")
	}
	if err := c.c.checkSummonCreatures([]string{packLeader}, map[string]bool{}, func(f string, a ...any) error { return errors.New(f) }); err != nil {
		t.Errorf("a summon refuses the table creature: %v", err)
	}
	if got := c.Creatures(); len(got) != 1 || got[0].Key != packLeader || got[0].BaseKey != wolfKey || got[0].NamePT != "Lobo Alfa" {
		t.Errorf("Creatures() = %+v", got)
	}
	// Wild Shape reads the creature as a beast form.
	d, err := c.WildShapeDerived(Derived{}, packLeader)
	if err != nil || d.HitPointsMax != 30 || d.ArmorClass != 15 {
		t.Errorf("WildShapeDerived = HP %d AC %d, %v", d.HitPointsMax, d.ArmorClass, err)
	}
}

func TestACreatureDoesNotChangeTheContentItCameFrom(t *testing.T) {
	srd := loadForTest(t)
	before, _ := srd.MonsterDerived(wolfKey)
	nBefore := len(srd.c.monsterEntries)
	c := withOverlayOn(t, srd, creatureOverlay(packLeaderCreature()))
	if _, ok := srd.MonsterDerived(packLeader); ok {
		t.Error("the original content knows the table creature")
	}
	if _, ok := srd.CreatureByKey(packLeader); ok || srd.HasCreature(packLeader) || len(srd.c.monsterEntries) != nBefore || len(srd.Creatures()) != 0 {
		t.Error("the original content changed")
	}
	if _, ok := srd.MonsterActionPlan(packLeader + "#bite"); ok {
		t.Error("the original content has the plan")
	}
	after, _ := srd.MonsterDerived(wolfKey)
	if after.HitPointsMax != before.HitPointsMax || after.ArmorClass != before.ArmorClass || after.Attacks[0].Damage != "2d4+2" || len(after.Features) != len(before.Features) {
		t.Errorf("the SRD wolf changed: %+v", after.Attacks)
	}
	viaClone, _ := c.MonsterDerived(wolfKey)
	if viaClone.Attacks[0].Damage != "2d4+2" || viaClone.HitPointsMax != 11 || len(viaClone.Features) != len(before.Features) {
		t.Error("the wolf seen through the new content changed")
	}
	if m := srd.c.monsters[wolfKey]; len(m.SpecialAbilities) != len(before.Features) || m.Actions[0].Damage[0].Dice != "2d4+2" {
		t.Error("the shared SRD stat block was written")
	}
}

func TestAnArchivedOrSwitchedOffCreatureStillResolves(t *testing.T) {
	cr := packLeaderCreature()
	cr.Archived = true
	o := creatureOverlay(cr)
	o.Off = []string{packLeader}
	c := withOverlay(t, o)
	if d, ok := c.MonsterDerived(packLeader); !ok || d.HitPointsMax != 30 {
		t.Errorf("MonsterDerived = %d, %v", d.HitPointsMax, ok)
	}
	if got := c.Creatures(); len(got) != 1 || !got[0].Archived || !got[0].Off {
		t.Errorf("Creatures() = %+v, want it archived and off", got)
	}
	if !c.Switchable(packLeader) {
		t.Error("the table creature cannot be switched off")
	}
}

func TestACreatureKeyIsChecked(t *testing.T) {
	srd := loadForTest(t)
	cases := []struct {
		name   string
		cs     func() []TableCreature
		reason string
	}{
		{"a wrong prefix", func() []TableCreature {
			c := packLeaderCreature()
			c.Key = "monster:lobo@mesa"
			return []TableCreature{c}
		}, ""},
		{"no suffix", func() []TableCreature {
			c := packLeaderCreature()
			c.Key = "creature:lobo"
			return []TableCreature{c}
		}, ReasonKey},
		{"a bad slug", func() []TableCreature {
			c := packLeaderCreature()
			c.Key = "creature:Lobo Alfa@mesa"
			return []TableCreature{c}
		}, ReasonKey},
		{"a reserved word", func() []TableCreature {
			c := packLeaderCreature()
			c.Key = "creature:wild-shape-lobo@mesa"
			return []TableCreature{c}
		}, ReasonReservedKey},
		{"a repeated key", func() []TableCreature {
			a, b := packLeaderCreature(), packLeaderCreature()
			b.NamePT = "Outro"
			return []TableCreature{a, b}
		}, ReasonDuplicateKey},
		{"an empty name", func() []TableCreature {
			c := packLeaderCreature()
			c.NamePT = ""
			return []TableCreature{c}
		}, ReasonName},
	}
	for _, tc := range cases {
		_, err := srd.With(creatureOverlay(tc.cs()...))
		oe, ok := errors.AsType[*OverlayError](err)
		if !ok {
			t.Errorf("%s: err = %v, want an overlay error", tc.name, err)
			continue
		}
		if tc.reason != "" && oe.Reason != tc.reason {
			t.Errorf("%s: reason %q, want %q (%v)", tc.name, oe.Reason, tc.reason, err)
		}
	}
}

func TestACreatureLimitIsTheTableLimit(t *testing.T) {
	srd := loadForTest(t)
	mk := func(n int) []TableCreature {
		var out []TableCreature
		for i := range n {
			c := packLeaderCreature()
			c.Key, c.NamePT = "creature:lobo-"+itoa(i)+tableSuffix, "Lobo "+itoa(i)
			c.Traits = nil
			out = append(out, c)
		}
		return out
	}
	c, err := srd.With(creatureOverlay(mk(MaxOverlayEntries)...))
	if err != nil {
		t.Fatalf("%d creatures: %v", MaxOverlayEntries, err)
	}
	if got := len(c.Creatures()); got != MaxOverlayEntries {
		t.Errorf("Creatures() = %d", got)
	}
	_, err = srd.With(creatureOverlay(mk(MaxOverlayEntries + 1)...))
	if oe, ok := errors.AsType[*OverlayError](err); !ok || oe.Reason != ReasonLimit {
		t.Errorf("1001 creatures: err = %v, want the limit", err)
	}
}

func TestACreatureRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	trait := func(n, x string) TableCreatureTrait { return TableCreatureTrait{NamePT: n, TextPT: x} }
	many := make([]TableCreatureTrait, MaxCreatureTraits+1)
	for i := range many {
		many[i] = trait("Traço "+string(rune('a'+i)), "x")
	}
	cases := []struct {
		name          string
		edit          func(c *TableCreature)
		field, reason string
	}{
		{"a base that does not exist", func(c *TableCreature) { c.BaseKey = "monster:nao-existe" }, ".base_key", ReasonReference},
		{"no base", func(c *TableCreature) { c.BaseKey = "" }, ".base_key", ReasonReference},
		{"a table creature as base", func(c *TableCreature) { c.BaseKey = "creature:outro@mesa" }, ".base_key", ReasonReference},
		{"a spell as base", func(c *TableCreature) { c.BaseKey = "spell:light" }, ".base_key", ReasonReference},
		{"negative hit points", func(c *TableCreature) { c.HitPoints = -1 }, ".hit_points", ReasonValue},
		{"too many hit points", func(c *TableCreature) { c.HitPoints = 10000 }, ".hit_points", ReasonValue},
		{"negative armor class", func(c *TableCreature) { c.ArmorClass = -1 }, ".armor_class", ReasonValue},
		{"armor class 41", func(c *TableCreature) { c.ArmorClass = 41 }, ".armor_class", ReasonValue},
		{"damage bonus -11", func(c *TableCreature) { c.DamageBonus = -11 }, ".damage_bonus", ReasonValue},
		{"damage bonus 31", func(c *TableCreature) { c.DamageBonus = 31 }, ".damage_bonus", ReasonValue},
		{"11 traits", func(c *TableCreature) { c.Traits = many }, ".traits", ReasonLimit},
		{"an empty trait name", func(c *TableCreature) { c.Traits = []TableCreatureTrait{trait("", "x")} }, ".traits[0].name_pt", ReasonName},
		{"a long trait name", func(c *TableCreature) { c.Traits = []TableCreatureTrait{trait(strings.Repeat("a", 81), "x")} }, ".traits[0].name_pt", ReasonName},
		{"a trait name on two lines", func(c *TableCreature) { c.Traits = []TableCreatureTrait{trait("a\nb", "x")} }, ".traits[0].name_pt", ReasonName},
		{"a trait name with no letter", func(c *TableCreature) { c.Traits = []TableCreatureTrait{trait("!!!", "x")} }, ".traits[0].name_pt", ReasonName},
		{"a trait named as a base trait", func(c *TableCreature) { c.Traits = []TableCreatureTrait{trait("Pack Tactics", "x")} }, ".traits[0].name_pt", ReasonName},
		{"two traits with a name", func(c *TableCreature) { c.Traits = []TableCreatureTrait{trait("Uivo", "x"), trait("uivo", "y")} }, ".traits[1].name_pt", ReasonName},
		{"an empty trait text", func(c *TableCreature) { c.Traits = []TableCreatureTrait{trait("Uivo", "")} }, ".traits[0].text_pt", ReasonText},
		{"a long trait text", func(c *TableCreature) { c.Traits = []TableCreatureTrait{trait("Uivo", strings.Repeat("a", 4001))} }, ".traits[0].text_pt", ReasonText},
	}
	for _, tc := range cases {
		cr := packLeaderCreature()
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
	// The edges are accepted.
	edge := packLeaderCreature()
	edge.HitPoints, edge.ArmorClass, edge.DamageBonus = MaxCreatureHitPoints, MaxCreatureArmorClass, MaxCreatureDamageBonus
	edge.Traits = make([]TableCreatureTrait, MaxCreatureTraits)
	for i := range edge.Traits {
		edge.Traits[i] = trait("Traço "+string(rune('a'+i)), strings.Repeat("ã", maxTextRunes))
	}
	if _, err := srd.With(creatureOverlay(edge)); err != nil {
		t.Errorf("the limits themselves were refused: %v", err)
	}
}
