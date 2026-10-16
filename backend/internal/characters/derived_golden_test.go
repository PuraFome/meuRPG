package characters

import (
	"bytes"
	"encoding/json"
	goflag "flag"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The conversions of derived.go (derivedToProto, catalogToProto) and the reads of
// choices.go and spelllist.go, frozen as golden files (testdata/choices_*.golden.json,
// testdata/derived_*.golden.json, testdata/spells_*.golden.json). Run with
// -update-choices-goldens to write them again after a change of behavior that was meant.

var updateChoicesGoldens = goflag.Bool("update-choices-goldens", false, "write the golden files of the choices, the derived sheet, the catalog and the spells page again")

// choicesGolden compares v (a proto message, or any JSON value) with the golden file
// testdata/<name>.golden.json, after the replacements old -> new (ids replaced by names).
func choicesGolden(t *testing.T, name string, v any, replace ...string) {
	t.Helper()
	var raw []byte
	var err error
	if m, ok := v.(proto.Message); ok {
		raw, err = protojson.Marshal(m)
	} else {
		raw, err = json.Marshal(v)
	}
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(replace); i += 2 {
		raw = bytes.ReplaceAll(raw, []byte(replace[i]), []byte(replace[i+1]))
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	got, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", name+".golden.json")
	if *updateChoicesGoldens {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the golden file: %v (write it with -update-choices-goldens)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from %s:\n--- got\n%s\n--- want\n%s", name, path, got, want)
	}
}

// derivedFixture is a derived sheet with every field of every list set to a value of its
// own, so the golden shows where each one lands; the edges (a spell slot of 0, a
// proficiency kind the sheet does not list, 0 hit points) are in it too.
func derivedFixture() rules.Derived {
	return rules.Derived{
		ContentVersion: "srd-5.1+mesa.7", RaceNamePT: "Draconato", SubraceNamePT: "Sub", BackgroundNamePT: "Acólito",
		BackgroundEquipmentPT: "Um símbolo sagrado", TotalLevel: 7, ProficiencyBonus: 3, PassivePerception: 13,
		PassiveInvestigation: 11, PassiveInsight: 12, Initiative: 2, NextLevelXP: 34000, ArmorClass: 16,
		ArmorClassDescription: "Cota de malha", HitPointsMax: 0, HitPointsFromEffects: -2,
		SpeedWalkFt: 30, SpeedFlyFt: 40, SpeedSwimFt: 15, SpeedClimbFt: 20, SpeedBurrowFt: 5, Hover: true,
		Classes: []rules.DerivedClass{
			{ClassKey: "class:warlock", NamePT: "Bruxo", Level: 5, SubclassNamePT: "Corruptor"},
			{ClassKey: "class:wizard", NamePT: "Mago", Level: 2},
		},
		Abilities: []rules.AbilityScore{
			{Ability: rules.STR, NamePT: "Força", Base: 15, RaceBonus: 2, ManualBonus: 1, Bonus: 3, Score: 18, Modifier: 4},
			{Ability: rules.CHA, NamePT: "Carisma", Base: 8, Score: 8, Modifier: -1},
		},
		SavingThrows: []rules.SavingThrow{
			{Ability: rules.DEX, NamePT: "Destreza", Proficient: true, Bonus: 5},
			{Ability: rules.INT, NamePT: "Inteligência", Bonus: -1},
		},
		Skills: []rules.Skill{
			{Key: "skill:arcana", NamePT: "Arcanismo", Ability: rules.INT, Proficiency: rules.ProficiencyExpertise, Bonus: 7},
			{Key: "skill:stealth", NamePT: "Furtividade", Ability: rules.DEX, Proficiency: rules.ProficiencyHalf, Bonus: 3},
		},
		HitDice: []rules.HitDice{{Die: 8, Count: 5}, {Die: 6, Count: 2}},
		Senses:  []rules.Sense{{Key: "sense:darkvision", NamePT: "Visão no escuro", RangeFt: 60, Source: "race:dragonborn"}},
		Spellcasting: []rules.Spellcasting{
			{Class: "class:warlock", ClassNamePT: "Bruxo", Ability: rules.CHA, SaveDC: 13, AttackBonus: 5, CantripsKnown: 3, SpellsKnownMax: 6, PreparedMax: 99, MaxSpellLevel: 3, SpellList: "class:warlock"},
			{Class: "class:wizard", ClassNamePT: "Mago", Ability: rules.INT, SaveDC: 12, AttackBonus: 4, CantripsKnown: 3, PreparesSpells: true, PreparedMax: 4, SpellsKnownMax: 77, Spellbook: true},
		},
		SpellSlots: []int{3, 0, 1, 0, 0, 0, 0, 0, 0},
		PactMagic:  &rules.PactMagic{SlotLevel: 3, Slots: 2},
		Spells: []rules.CharacterSpell{
			{Spell: rules.SpellEntry{Key: "spell:shield", Name: "Shield", NamePT: "Escudo Arcano", Level: 1, School: "school:abjuration", SchoolNamePT: "Abjuração", Classes: []string{"class:wizard"}, CastingTime: rules.CastingTime{Amount: 1, Unit: rules.CastReaction}}, Prepared: true},
			{Spell: rules.SpellEntry{Key: "spell:detect-magic", NamePT: "Detectar Magia", Level: 1, Ritual: true, Concentration: true, Archived: true, Off: true}},
		},
		Attacks: []rules.Attack{{
			Key: "equipment:quarterstaff", Name: "Quarterstaff", NamePT: "Bordão", Kind: "weapon", AttackBonus: 6, SaveDC: 14, SaveAbility: rules.WIS,
			Damage: "1d6+4", DamageType: "damage-type:bludgeoning", DamageTypeNamePT: "concussão", VersatileDamage: "1d8+4", DamageNotePT: "nota",
			DamageDice: rules.DiceFormula{Count: 1, Sides: 6, Bonus: 4, AddsModifier: true}, VersatileDice: rules.DiceFormula{Count: 1, Sides: 8},
			RangeFt: 5, LongRangeFt: 10, Melee: true, Notes: "duas mãos", Beams: 2, SpellDice: "2d10",
		}, {Key: "spell:eldritch-blast", Kind: "spell"}, {Key: "x", Kind: "neither"}},
		Resources: []rules.Resource{
			{Key: "pact_slots", NamePT: "Espaços de pacto", Max: 2, Recharge: rules.RechargeShortRest, Source: "class:warlock"},
			{Key: "odd", Recharge: "never-heard-of"},
		},
		SaveActions: []rules.SaveAction{{Key: "breath", Name: "Breath", Ability: rules.DEX, DC: 13, OnSuccess: "half", Usage: "1/descanso", Text: "texto"}},
		Actions:     []rules.Action{{Key: "feature:second-wind", NamePT: "Retomar o Fôlego", Economy: "bonus_action", Resource: "second_wind", Source: "class:fighter"}},
		StandardActions: []rules.Action{
			{Key: "action:dash", NamePT: "Disparada", Economy: "action"},
			{Key: "action:odd", Economy: "never-heard-of"},
		},
		Features: []rules.Feature{{
			Key: "feature:pact-boon", Name: "Pact Boon", NamePT: "Dádiva do Pacto", SourcePT: "Bruxo 3", Description: []string{"um", "dois"},
			SummaryPT: "resumo", DescriptionPT: []string{"primeiro", "segundo", "terceiro"}, DescriptionPTMissing: true, DescriptionPTOnly: true,
		}},
		BreathWeapon: &rules.BreathWeapon{
			AncestryKey: "trait:draconic-ancestry-red", DamageType: "damage-type:fire", DamageTypePT: "fogo", Dice: "2d6",
			Shape: rules.BreathLine, SizeFt: 30, WidthFt: 5, SaveAbility: rules.DEX, DC: 13, Text: "sopro",
		},
		Resistances: []rules.ChoiceResistance{{DamageType: "damage-type:fire", DamageTypePT: "fogo", SourceKey: "trait:draconic-ancestry-red"}},
		Languages:   []rules.NamedKey{{Key: "language:common", NamePT: "Comum"}, {Key: "language:draconic", NamePT: "Dracônico"}},
		Proficiencies: []rules.Proficiency{
			{Kind: "armor", Key: "a", NamePT: "Armaduras leves"},
			{Kind: "weapon", Key: "w", NamePT: "Armas simples"},
			{Kind: "tool", Key: "t", NamePT: "Ferramentas de ladrão"},
			{Kind: "other", Key: "o", NamePT: "Kit de herbalismo"},
			{Kind: "skill", Key: "s", NamePT: "never shown"},
			{Kind: "saving-throw", Key: "st", NamePT: "never shown either"},
		},
		Hints:  []rules.Hint{{Source: "race:dwarf", TextPT: "vantagem contra veneno"}},
		Issues: []rules.Issue{{Code: "unknown_key", Field: "full.race_key", Message: "a key nobody knows"}},
	}
}

// TestDerivedToProtoGolden freezes derivedToProto: the hand-made sheet with every field,
// an empty one, a breath in a cone with no pact magic, and a real wizard derived from
// the SRD.
func TestDerivedToProtoGolden(t *testing.T) {
	t.Parallel()
	choicesGolden(t, "derived_fixture", derivedToProto(derivedFixture()))
	choicesGolden(t, "derived_empty", derivedToProto(rules.Derived{}))
	cone := derivedFixture()
	cone.PactMagic = nil
	cone.HitPointsMax = 9
	cone.BreathWeapon.Shape, cone.BreathWeapon.WidthFt = rules.BreathCone, 0
	choicesGolden(t, "derived_cone", derivedToProto(cone))
	// The real wizard's feature texts are left out: the golden keeps no rules text.
	pensantus := rules.Derive(buildOf(pensantusSheet().GetFull()), loadRules(t))
	for i := range pensantus.Features {
		pensantus.Features[i].Description, pensantus.Features[i].DescriptionPT = nil, nil
	}
	choicesGolden(t, "derived_pensantus", derivedToProto(pensantus))
}

// catalogFixture is a catalog with every field of every list set, and the edges: a class
// without spellcasting, a subclass without casting, a melee and a ranged weapon,
// categories and kinds the API does not name.
func catalogFixture() rules.Catalog {
	return rules.Catalog{
		ContentVersion: "srd-5.1+mesa.3", Attribution: "CC-BY-4.0",
		Abilities: []rules.AbilityEntry{{Ability: rules.WIS, Name: "Wisdom", NamePT: "Sabedoria", AbbreviationPT: "SAB"}},
		Races: []rules.RaceEntry{{
			Key: "race:half-elf", Name: "Half-Elf", NamePT: "Meio-Elfo", SpeedFt: 30, AbilityBonuses: map[rules.Ability]int{rules.CHA: 2},
			ChoiceBonuses: []int{1, 1}, SkillProficiencies: []string{"skill:insight"}, Archived: true, Off: true,
		}},
		Subraces: []rules.SubraceEntry{{
			Key: "subrace:hill-dwarf", Name: "Hill Dwarf", NamePT: "Anão da Colina", Race: "race:dwarf",
			AbilityBonuses: map[rules.Ability]int{rules.WIS: 1}, SkillProficiencies: []string{"skill:x"}, Archived: true, Off: true,
		}},
		Classes: []rules.ClassEntry{{
			Key: "class:wizard", Name: "Wizard", NamePT: "Mago", HitDie: 6, SavingThrows: []rules.Ability{rules.INT, rules.WIS},
			SkillChoices: 2, SkillOptions: []string{"skill:arcana", "skill:history"}, SpellcastingAbility: rules.INT, SpellcastingLevel: 1,
			SpellPreparation: rules.PreparationSpellbook, SubclassLevel: 2, MaxSpellLevelByLevel: []int{1, 1, 2}, Archived: true, Off: true,
			SpellListFrom: "class:sorcerer",
		}, {Key: "class:fighter", NamePT: "Guerreiro", HitDie: 10, SpellPreparation: "ignored without an ability"}},
		Subclasses: []rules.SubclassEntry{{
			Key: "subclass:eldritch-knight", Name: "Eldritch Knight", NamePT: "Cavaleiro Arcano", Class: "class:fighter", Archived: true, Off: true,
			AlwaysPrepared: []rules.SubclassSpellRef{{Spell: "spell:shield", ClassLevel: 3}},
			Casting:        &rules.SubclassCasting{Kind: "third", Ability: rules.INT, Preparation: rules.PreparationKnown, SpellList: "class:wizard", StartLevel: 3, MaxSpellLevelByLevel: []int{0, 0, 1}},
		}, {Key: "subclass:champion", NamePT: "Campeão", Class: "class:fighter"}},
		Backgrounds: []rules.BackgroundEntry{{Key: "background:acolyte", Name: "Acolyte", NamePT: "Acólito", SkillProficiencies: []string{"skill:religion"}, EquipmentPT: "Incenso", Archived: true, Off: true}},
		Skills:      []rules.SkillEntry{{Key: "skill:arcana", Name: "Arcana", NamePT: "Arcanismo", Ability: rules.INT}},
		Armor: []rules.ArmorEntry{
			{Key: "equipment:leather", Name: "Leather", NamePT: "Couro", Category: "light"},
			{Key: "equipment:plate", NamePT: "Placas", Category: "heavy"},
			{Key: "equipment:shield", NamePT: "Escudo", Category: "shield"},
		},
		Weapons: []rules.WeaponEntry{
			{Key: "equipment:longbow", Name: "Longbow", NamePT: "Arco longo", Category: "martial", Range: "ranged"},
			{Key: "equipment:club", NamePT: "Clava", Category: "simple", Range: "melee"},
			{Key: "equipment:odd", Category: "exotic"},
		},
		Spells:           []rules.SpellEntry{{Key: "spell:counterspell", NamePT: "Contramágica", Level: 3, CastingTime: rules.CastingTime{Unit: rules.CastReaction}}},
		ChallengeRatings: []rules.ChallengeRating{{Rating: "1/2", XP: 100}},
		LevelXP:          []int{0, 300, 900},
		Languages:        []rules.NamedEntry{{Key: "language:elvish", NamePT: "Élfico", Kind: "language"}},
		Proficiencies:    []rules.NamedEntry{{Key: "tool:a", NamePT: "A", Kind: "tool"}, {Key: "x", NamePT: "X", Kind: "unknown-kind"}},
		DamageTypes:      []rules.NamedEntry{{Key: "damage-type:fire", NamePT: "Fogo"}},
	}
}

// TestCatalogToProtoGolden freezes catalogToProto: the hand-made catalog, an empty one
// and the SRD's classes and subclasses (the rest of the SRD is lists of names).
func TestCatalogToProtoGolden(t *testing.T) {
	t.Parallel()
	choicesGolden(t, "derived_catalog_fixture", catalogToProto(catalogFixture()))
	choicesGolden(t, "derived_catalog_empty", catalogToProto(rules.Catalog{}))
	srd := loadRules(t).Catalog()
	choicesGolden(t, "derived_catalog_srd_classes", catalogToProto(rules.Catalog{Classes: srd.Classes, Subclasses: srd.Subclasses}))
}
