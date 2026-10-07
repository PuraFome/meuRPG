package rules

import (
	"strconv"
	"strings"
	"testing"
)

// The fixtures of the overlay tests: a table class of each casting kind and
// third-caster subclasses of SRD classes, built the way the editors will (the
// numbers of their tables come from the SRD's own class tables), plus the small
// table of races, backgrounds and spells the other tests use.

// tf is a table feature "feature:<slug>@mesa".
func tf(slug, name string, effects ...Effect) TableFeature {
	return TableFeature{Key: "feature:" + slug + tableSuffix, NamePT: name, DescPT: []string{"Texto de teste de " + name + "."}, Effects: effects}
}

// genKind is a casting kind of the generated classes, and the SRD class whose
// table and spell list it copies.
type genKind struct {
	name        string
	casting     TableCasting
	tableFrom   string // the SRD class whose slot columns it copies
	cantrips    bool
	knownSpells bool
}

var genKinds = []genKind{
	{name: "none"},
	{name: "full-prepared", casting: TableCasting{Kind: CastingFull, Ability: WIS, Preparation: PreparationPrepared, ListFrom: "class:cleric"}, tableFrom: "class:cleric", cantrips: true},
	{name: "full-known", casting: TableCasting{Kind: CastingFull, Ability: CHA, Preparation: PreparationKnown, ListFrom: "class:sorcerer"}, tableFrom: "class:sorcerer", cantrips: true, knownSpells: true},
	{name: "half-prepared", casting: TableCasting{Kind: CastingHalf, Ability: WIS, Preparation: PreparationPrepared, ListFrom: "class:paladin"}, tableFrom: "class:paladin"},
	{name: "pact-known", casting: TableCasting{Kind: CastingPact, Ability: CHA, Preparation: PreparationKnown, ListFrom: "class:warlock"}, tableFrom: "class:warlock", cantrips: true, knownSpells: true},
}

// genClass builds a table class with a 20-level table: features with every
// effect of the menu, and the casting columns copied from the SRD class gk says.
func genClass(c *Content, gk genKind) TableClass {
	slug := "gen-" + gk.name
	tc := TableClass{
		Key: "class:" + slug + tableSuffix, NamePT: "Classe " + gk.name,
		HitDie:        8,
		SavingThrows:  []Ability{CON, WIS},
		SkillChoose:   2,
		SkillFrom:     []string{"skill:arcana", "skill:history", "skill:medicine", "skill:nature", "skill:perception", "skill:survival"},
		Proficiencies: []string{"proficiency:light-armor", "proficiency:simple-weapons"},
		Minimums:      map[Ability]int{CON: 11},
		SubclassLevel: 3,
		Casting:       gk.casting,
		Levels:        make([]TableClassLevel, MaxLevel),
	}
	for i := range tc.Levels {
		lvl := i + 1
		row := &tc.Levels[i]
		if gk.tableFrom != "" {
			src := c.c.classLevels[gk.tableFrom][i].Spellcasting
			if gk.cantrips {
				row.CantripsKnown = src.CantripsKnown
			}
			if gk.knownSpells {
				row.SpellsKnown = src.SpellsKnown
			}
			row.Slots = src.Slots
			if lvl < startOf(gk.casting) {
				row.TableLevel = TableLevel{}
			}
		}
		row.Features = genFeatures(slug, lvl)
	}
	return tc
}

func startOf(c TableCasting) int {
	if c.Kind == CastingHalf {
		return 2
	}
	return 1
}

// genFeatures are the features of a level of a generated class: each effect
// type of the menu comes at some level.
func genFeatures(slug string, lvl int) []TableFeature {
	name := func(s string) string { return slug + "-" + s }
	resource := strings.ReplaceAll(slug, "-", "_") + "_surto"
	switch lvl {
	case 1:
		return []TableFeature{
			tf(name("vigor"), "Vigor", Effect{Type: "modifier", Target: "initiative", Mode: "add", Value: "1"}),
			tf(name("treino"), "Treino", Effect{Type: "proficiency", Proficiency: "skill:survival"}),
		}
	case 2:
		return []TableFeature{tf(name("surto"), "Surto",
			Effect{Type: "resource", Resource: resource, Max: "prof()", Recharge: "short_rest"},
			Effect{Type: "grant_action", Economy: "bonus_action"})}
	case 3:
		return []TableFeature{tf(name("olhos"), "Olhos", Effect{Type: "sense", Sense: "darkvision", RangeFt: 60},
			Effect{Type: "roll_mode", Roll: "advantage", Targets: []string{"save.wis"}, Tags: []string{"against:charm"}})}
	case 5:
		return []TableFeature{tf(name("ataque"), "Ataque", Effect{Type: "extra_attack", Count: 2})}
	case 6:
		return []TableFeature{tf(name("estilo"), "Estilo de luta", Effect{
			Type: "choice", Choice: "feature", Count: 1,
			From: []string{"feature:fighter-fighting-style-defense", "feature:fighter-fighting-style-dueling"},
		})}
	case 8:
		return []TableFeature{tf(name("pericia"), "Perícia extra", Effect{Type: "choice", Choice: "skill", Count: 1})}
	case 10:
		return []TableFeature{tf(name("mestre"), "Mestria", Effect{Type: "choice", Choice: "expertise", Count: 1})}
	case 12:
		return []TableFeature{tf(name("marca"), "Marca", Effect{Type: "note", Value: "prof()", TextPT: "Bônus de marca {value}."})}
	case 14:
		return []TableFeature{tf(name("lingua"), "Língua", Effect{Type: "choice", Choice: "language", Count: 1})}
	case 20:
		return []TableFeature{tf(name("auge"), "Auge")} // text only
	}
	return nil
}

// genSubclasses are two subclasses of a generated class: one with features,
// and, when the class casts, one with always-prepared spells.
func genSubclasses(tc TableClass) []TableSubclass {
	slug := slugOfKey(tc.Key)
	plain := TableSubclass{
		Key: "subclass:" + slug + "-a" + tableSuffix, NamePT: "Caminho A",
		Class: tc.Key,
		Levels: []TableSubclassLevel{
			{Level: 3, Features: []TableFeature{tf(slug+"-a3", "A3", Effect{Type: "modifier", Target: "speed.walk", Mode: "add", Value: "5"})}},
			{Level: 9, Features: []TableFeature{tf(slug+"-a9", "A9", Effect{Type: "note", TextPT: "Nota do caminho."})}},
		},
	}
	out := []TableSubclass{plain}
	if tc.Casting.Kind != CastingNone {
		out = append(out, TableSubclass{
			Key: "subclass:" + slug + "-b" + tableSuffix, NamePT: "Caminho B",
			Class:          tc.Key,
			Levels:         []TableSubclassLevel{{Level: 3, Features: []TableFeature{tf(slug+"-b3", "B3")}}},
			AlwaysPrepared: []TableAlwaysPrepared{{ClassLevel: 3, Spell: "spell:bless"}, {ClassLevel: 5, Spell: "spell:fireball"}},
		})
	}
	return out
}

// thirdCaster is a table subclass of an SRD class that casts as a third caster,
// its table following the 2014 third casters' (slots of the full table at a
// third of the level, rounded up).
func thirdCaster(c *Content, slug, class, list, preparation string) TableSubclass {
	ts := TableSubclass{
		Key: "subclass:" + slug + tableSuffix, NamePT: "Terço " + slug,
		Class:   class,
		Casting: &TableCasting{Kind: CastingThird, Ability: INT, Preparation: preparation, ListFrom: list},
	}
	for lvl := 3; lvl <= MaxLevel; lvl++ {
		src := c.c.classLevels["class:wizard"][(lvl+2)/3-1].Spellcasting
		cantrips := 2
		if lvl >= 10 {
			cantrips = 3
		}
		row := TableSubclassLevel{Level: lvl, CantripsKnown: cantrips, SpellsKnown: min(3+(lvl-3)/2, 13), Slots: src.Slots}
		if lvl == 3 || lvl == 7 {
			row.Features = []TableFeature{tf(slug+"-l"+strconv.Itoa(lvl), "Passo "+strconv.Itoa(lvl))}
		}
		ts.Levels = append(ts.Levels, row)
	}
	return ts
}

// genOverlay is the overlay of the sweeps: a class of each casting kind with two
// subclasses, and third-caster subclasses of the SRD Fighter (known spells) and
// Rogue (prepared).
func genOverlay(t testing.TB, c *Content) Overlay {
	t.Helper()
	o := Overlay{Revision: 7}
	for _, gk := range genKinds {
		tc := genClass(c, gk)
		o.Classes = append(o.Classes, tc)
		o.Subclasses = append(o.Subclasses, genSubclasses(tc)...)
	}
	o.Subclasses = append(o.Subclasses,
		thirdCaster(c, "cavaleiro-runico", "class:fighter", "class:wizard", PreparationKnown),
		thirdCaster(c, "trapaceiro-mistico", "class:rogue", "class:wizard", PreparationPrepared),
	)
	return o
}

// withOverlay applies o to a fresh load of the SRD, failing the test on an error.
func withOverlay(t testing.TB, o Overlay) *Content {
	t.Helper()
	return withOverlayOn(t, loadForTest(t), o)
}

// withOverlayOn applies o to the given base.
func withOverlayOn(t testing.TB, base *Content, o Overlay) *Content {
	t.Helper()
	c, err := base.With(o)
	if err != nil {
		t.Fatalf("With: %v", err)
	}
	return c
}

// tableTestSpells are table spells of every shape the brief lists.
func tableTestSpells() []TableSpell {
	base := func(slug, name string, level int) TableSpell {
		return TableSpell{
			Key: "spell:" + slug + tableSuffix, NamePT: name, Level: level, School: "school:evocation",
			CastingTime: TableCastingTime{Amount: 1, Unit: CastAction}, Range: TableRange{Kind: RangeRanged, DistanceFt: 60},
			Duration: TableDuration{Kind: DurationInstantaneous}, Components: TableComponents{Verbal: true, Somatic: true},
			Classes: []string{"class:wizard"}, DescPT: []string{"Texto de teste."}, Target: SpellTarget{Kind: TargetCreature},
		}
	}
	bolt := base("raio-de-teste", "Raio de teste", 1)
	bolt.Attack = "ranged"
	bolt.Damage = []TableSpellDamage{{Type: "damage-type:force", Dice: "3d6", PerSlotLevel: "1d6"}}

	cone := base("cone-de-teste", "Cone de teste", 3)
	cone.Range = TableRange{Kind: RangeSelf}
	cone.Target = SpellTarget{Kind: TargetArea, Shape: ShapeCone, SizeFt: 15}
	cone.Save = &SpellSave{Ability: DEX, OnSuccess: "half"}
	cone.Damage = []TableSpellDamage{{Type: "damage-type:cold", Dice: "6d6", PerSlotLevel: "1d6"}}
	cone.Classes = []string{"class:wizard", "class:gen-full-prepared" + tableSuffix}

	twin := base("par-de-teste", "Par de teste", 2)
	twin.Target = SpellTarget{Kind: TargetCreatures, Count: 2, PerSlotLevel: 1}
	twin.Save = &SpellSave{Ability: WIS, OnSuccess: "none"}
	twin.Concentration = true
	twin.Duration = TableDuration{Kind: DurationTimed, Amount: 1, Unit: DurationMinute}
	twin.Classes = []string{"class:wizard", "class:cleric"}

	heal := base("cura-de-teste", "Cura de teste", 1)
	heal.Range = TableRange{Kind: RangeTouch}
	heal.Classes = []string{"class:cleric"}
	heal.Heal = &TableSpellHeal{Dice: "2d8", PerSlotLevel: "1d8", AddsModifier: true}

	cantrip := base("faisca-de-teste", "Faísca de teste", 0)
	cantrip.Attack = "ranged"
	cantrip.Damage = []TableSpellDamage{{Type: "damage-type:lightning", Dice: "1d8", PerTier: "1d8"}}

	return []TableSpell{bolt, cone, twin, heal, cantrip}
}

// tableMisc are a race with a subrace, and a background, of the table.
func tableMisc() (TableRace, TableSubrace, TableBackground) {
	race := TableRace{
		Key: "race:anao-das-brumas" + tableSuffix, NamePT: "Anão das Brumas",
		Size: "Medium", SpeedFt: 25, AbilityBonuses: map[Ability]int{CON: 1}, ChoiceBonuses: []int{1, 2},
		DarkvisionFt: 60, Languages: []string{"language:common"}, LanguageChoices: 1,
		Traits: []TableFeature{{
			Key: "trait:resistente" + tableSuffix, NamePT: "Resistente", DescPT: []string{"Texto de teste."},
			Effects: []Effect{{Type: "roll_mode", Roll: "advantage", Targets: []string{"save.con"}, Tags: []string{"against:poison"}}},
		}},
	}
	sub := TableSubrace{
		Key: "subrace:da-colina-nevoenta" + tableSuffix, NamePT: "Da Colina Nevoenta",
		Race: race.Key, AbilityBonuses: map[Ability]int{WIS: 1},
		Traits: []TableFeature{{Key: "trait:olhar-firme" + tableSuffix, NamePT: "Olhar firme", Effects: []Effect{{Type: "proficiency", Proficiency: "skill:perception"}}}},
	}
	bg := TableBackground{
		Key: "background:guarda-de-farol" + tableSuffix, NamePT: "Guarda de farol",
		Skills: []string{"skill:insight", "skill:religion"}, Tools: []string{"proficiency:thieves-tools"}, LanguageChoices: 1,
		EquipmentPT: "Uma lanterna, um apito e roupas de viagem.",
		Feature:     TableFeature{Key: "background-feature:luz-guia" + tableSuffix, NamePT: "Luz-guia", Effects: []Effect{{Type: "note", TextPT: "Luz-guia: conhece a rota dos navios."}}},
	}
	return race, sub, bg
}

// fullOverlay is genOverlay plus the table spells, the race, the subrace and the
// background.
func fullOverlay(t testing.TB, c *Content) Overlay {
	t.Helper()
	o := genOverlay(t, c)
	o.Spells = tableTestSpells()
	race, sub, bg := tableMisc()
	o.Races, o.Subraces, o.Backgrounds = []TableRace{race}, []TableSubrace{sub}, []TableBackground{bg}
	return o
}
