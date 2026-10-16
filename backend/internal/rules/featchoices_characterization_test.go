package rules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// The characterization tests of the small checks of the choices and feats split
// in E5: the shape of an SRD feat (checkFeatShape), what a character lacks for a
// feat (unmetPrerequisite), the feats a sheet's picks give (pickedChoiceFeats),
// the shape of a stored pick (choiceValueOK) and why a spell does not take a
// metamagic option (metamagicReason).

// TestCheckFeatShapeCharacterization freezes the field and the message of every
// refusal of checkFeatShape, and the order of its checks.
func TestCheckFeatShapeCharacterization(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	cases := map[string]srd51.Feat{
		"ok: nothing":                      {},
		"ok: everything":                   {Minimums: map[string]int{"str": 13}, AnyOf: map[string]int{"dex": 1, "int": MaxScore}, Proficiency: "proficiency:medium-armor", Race: "race:elf", Level: MaxLevel},
		"ok: a subrace":                    {Race: "subrace:high-elf"},
		"bad: unknown minimum":             {Minimums: map[string]int{"luck": 13}},
		"bad: minimum 0":                   {Minimums: map[string]int{"str": 0}},
		"bad: minimum over the max":        {Minimums: map[string]int{"str": MaxScore + 1}},
		"bad: unknown any of":              {AnyOf: map[string]int{"luck": 13}},
		"bad: any of 0":                    {AnyOf: map[string]int{"dex": 0}},
		"bad: unknown proficiency":         {Proficiency: "proficiency:nope"},
		"bad: unknown race":                {Race: "race:nope"},
		"bad: a class as race":             {Race: "class:fighter"},
		"bad: negative level":              {Level: -1},
		"bad: level over the max":          {Level: MaxLevel + 1},
		"order: minimums before any of":    {Minimums: map[string]int{"str": 0}, AnyOf: map[string]int{"luck": 1}},
		"order: any of before proficiency": {AnyOf: map[string]int{"luck": 1}, Proficiency: "proficiency:nope"},
		"order: proficiency before race":   {Proficiency: "proficiency:nope", Race: "race:nope"},
		"order: race before level":         {Race: "race:nope", Level: -1},
	}
	got := map[string]string{}
	for name, f := range cases {
		field, msg := c.checkFeatShape(&f)
		got[name] = field + ": " + msg
		if strings.HasPrefix(name, "ok:") != (field == "") {
			t.Errorf("%s: %s %s", name, field, msg)
		}
	}
	goldenText(t, "featchoices-shape", got)
}

// TestUnmetPrerequisiteCharacterization freezes what unmetPrerequisite lists,
// and in which order, for each kind of prerequisite met and not.
func TestUnmetPrerequisiteCharacterization(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	sheet := func(edit func(d *Derived)) Derived {
		d := Derived{TotalLevel: 4}
		for _, a := range AllAbilities() {
			d.Abilities = append(d.Abilities, AbilityScore{Ability: a, Score: 12})
		}
		if edit != nil {
			edit(&d)
		}
		return d
	}
	b := Build{Race: "race:elf", Subrace: "subrace:high-elf"}
	every := FeatPrerequisite{
		Minimums: map[Ability]int{STR: 13, DEX: 12, CON: 15}, AnyOf: map[Ability]int{INT: 13, WIS: 14},
		Proficiency: "proficiency:heavy-armor", Spellcasting: true, Race: "race:dwarf", Level: 5,
	}
	cases := map[string]struct {
		p FeatPrerequisite
		b Build
		d Derived
	}{
		"nothing asked":                {FeatPrerequisite{}, b, sheet(nil)},
		"everything unmet":             {every, b, sheet(nil)},
		"any of met by one":            {FeatPrerequisite{AnyOf: map[Ability]int{INT: 13, WIS: 12}}, b, sheet(nil)},
		"a zero minimum is no minimum": {FeatPrerequisite{Minimums: map[Ability]int{STR: 0}, AnyOf: map[Ability]int{INT: 0}}, b, sheet(nil)},
		"all armor covers heavy": {FeatPrerequisite{Proficiency: "proficiency:heavy-armor"}, b, sheet(func(d *Derived) {
			d.Proficiencies = []Proficiency{{Key: "proficiency:all-armor"}}
		})},
		"the proficiency itself": {FeatPrerequisite{Proficiency: "proficiency:shields"}, b, sheet(func(d *Derived) {
			d.Proficiencies = []Proficiency{{Key: "proficiency:shields"}}
		})},
		"all armor is not shields": {FeatPrerequisite{Proficiency: "proficiency:shields"}, b, sheet(func(d *Derived) {
			d.Proficiencies = []Proficiency{{Key: "proficiency:all-armor"}}
		})},
		"spellcasting by a class": {FeatPrerequisite{Spellcasting: true}, b, sheet(func(d *Derived) { d.Spellcasting = []Spellcasting{{}} })},
		"spellcasting by a pact":  {FeatPrerequisite{Spellcasting: true}, b, sheet(func(d *Derived) { d.PactMagic = &PactMagic{} })},
		"spellcasting by a spell": {FeatPrerequisite{Spellcasting: true}, b, sheet(func(d *Derived) { d.Spells = []CharacterSpell{{}} })},
		"the race":                {FeatPrerequisite{Race: "race:elf"}, b, sheet(nil)},
		"the subrace":             {FeatPrerequisite{Race: "subrace:high-elf"}, b, sheet(nil)},
		"the level reached":       {FeatPrerequisite{Level: 4}, b, sheet(nil)},
	}
	got := map[string]string{}
	for name, tc := range cases {
		raw, err := json.Marshal(c.unmetPrerequisite(FeatEntry{Prerequisite: tc.p}, tc.b, tc.d))
		if err != nil {
			t.Fatal(err)
		}
		got[name] = string(raw)
	}
	goldenText(t, "featchoices-unmet", got)
}

// TestPickedChoiceFeatsCharacterization freezes which picks pickedChoiceFeats
// keeps: the shape of the pick, the owner on the sheet, the grant, the count,
// the list, the feats the sheet has and the repeats.
func TestPickedChoiceFeatsCharacterization(t *testing.T) {
	t.Parallel()
	open, _ := featChoiceTable(t)
	listed, _ := featChoiceTable(t, alertaFeat)
	owner := FeatChoiceKey(talentoTrait)
	pick := func(feat string) string { return ScopedChoice(owner, feat) }
	all := func(string) bool { return true }
	cases := map[string]struct {
		c     *Content
		b     Build
		owned func(string) bool
	}{
		"nothing":                       {open, Build{}, all},
		"one pick":                      {open, Build{FeatureChoices: []string{pick(alertaFeat)}}, all},
		"not scoped":                    {open, Build{FeatureChoices: []string{alertaFeat}}, all},
		"not a feat":                    {open, Build{FeatureChoices: []string{ScopedChoice(owner, "spell:light")}}, all},
		"not a feat choice":             {open, Build{FeatureChoices: []string{ScopedChoice(talentoTrait, alertaFeat)}}, all},
		"the owner is not on the sheet": {open, Build{FeatureChoices: []string{pick(alertaFeat)}}, func(string) bool { return false }},
		"an owner that grants nothing":  {open, Build{FeatureChoices: []string{ScopedChoice(FeatChoiceKey("trait:darkvision"), alertaFeat)}}, all},
		"past the count":                {open, Build{FeatureChoices: []string{pick(alertaFeat), pick(livreFeat)}}, all},
		"an unknown feat":               {open, Build{FeatureChoices: []string{pick("feat:nope"), pick(livreFeat)}}, all},
		"off the list":                  {listed, Build{FeatureChoices: []string{pick(livreFeat), pick(alertaFeat)}}, all},
		"already on the sheet":          {open, Build{Feats: []string{alertaFeat}, FeatureChoices: []string{pick(alertaFeat), pick(livreFeat)}}, all},
		"twice":                         {open, Build{FeatureChoices: []string{pick(alertaFeat), pick(alertaFeat)}}, all},
	}
	// Two feats from one owner, the second a repeat of the first.
	_, race := featChoiceTable(t)
	race.Traits[len(race.Traits)-1].Effects[0].Count = 2
	two := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Races: []TableRace{race}, Feats: []TableFeat{
		{TableEntry: TableEntry{Key: alertaFeat, NamePT: "Alerta"}}, {TableEntry: TableEntry{Key: livreFeat, NamePT: "Livre"}},
	}})
	cases["two of one owner, a repeat"] = struct {
		c     *Content
		b     Build
		owned func(string) bool
	}{two, Build{FeatureChoices: []string{pick(alertaFeat), pick(alertaFeat), pick(livreFeat)}}, all}
	got := map[string]string{}
	for name, tc := range cases {
		raw, err := json.Marshal(tc.c.c.pickedChoiceFeats(tc.b, tc.owned))
		if err != nil {
			t.Fatal(err)
		}
		got[name] = string(raw)
	}
	goldenText(t, "featchoices-picked", got)
}

// TestChoiceValueOKCharacterization freezes which stored picks have the shape
// of one.
func TestChoiceValueOKCharacterization(t *testing.T) {
	t.Parallel()
	open, _ := featChoiceTable(t)
	c := open.c
	noChoices := *c
	noChoices.choices = nil
	values := []string{
		"plain",
		ScopedChoice("feature:nope", "language:elvish"),
		ScopedChoice("feature:favored-enemy-1-type", LanguageNone),
		ScopedChoice("feature:favored-enemy-1-type#language", "language:elvish"),
		ScopedChoice("feature:favored-enemy-1-type", "language:nope"),
		ScopedChoice("trait:draconic-ancestry", "ability:dex"),
		ScopedChoice("race:half-elf#abilities", "ability:dex"),
		ScopedChoice("race:half-elf#abilities", "ability:luck"),
		ScopedChoice("subrace:high-elf", "spell:light"),
		ScopedChoice("subrace:high-elf", "spell:nope"),
		ScopedChoice("background:acolyte", "feat:grappler"),
		ScopedChoice("background:acolyte", "feat:nope"),
		ScopedChoice("feat:grappler", "terrain:forest"),
		ScopedChoice("feature:favored-enemy-1-type", "creature-type:beast"),
		ScopedChoice("feature:favored-enemy-1-type", "creature-type:humanoid"),
		ScopedChoice("feature:favored-enemy-1-type", "creature-type:nope"),
		ScopedChoice("feature:natural-explorer-1-terrain-type", "terrain:forest"),
		ScopedChoice("feature:natural-explorer-1-terrain-type", "terrain:nope"),
		ScopedChoice("feature:natural-explorer-1-terrain-type", "class:fighter"),
		ScopedChoice(FeatChoiceKey(talentoTrait), alertaFeat),
		ScopedChoice(FeatChoiceKey("trait:nope"), alertaFeat),
		ScopedChoice(FeatChoiceKey("trait:nope"), "spell:light"),
	}
	got := map[string]string{}
	for _, v := range values {
		got[v] = fmt.Sprintf("%v, without the choices file %v", c.choiceValueOK(v), noChoices.choiceValueOK(v))
	}
	goldenText(t, "featchoices-values", got)
}

// TestMetamagicReasonCharacterization freezes the reason of each option for
// spells made to sit on each side of its checks.
func TestMetamagicReasonCharacterization(t *testing.T) {
	t.Parallel()
	spell := func(edit func(d *SpellDetails)) *SpellDetails {
		d := &SpellDetails{
			Spell:       SpellEntry{NamePT: "Magia", Level: 1},
			CastingTime: CastingTime{Amount: 1, Unit: CastAction},
			Range:       SpellRange{Kind: RangeRanged, DistanceFt: 60},
			Duration:    SpellDuration{Kind: DurationTimed, Amount: 1, Unit: DurationMinute},
			Save:        &SpellSave{},
			Damage:      []SpellDamage{{}},
			Target:      SpellTarget{Kind: TargetCreature},
		}
		if edit != nil {
			edit(d)
		}
		return d
	}
	spells := map[string]*SpellDetails{
		"takes everything": spell(nil),
		"no save":          spell(func(d *SpellDetails) { d.Save = nil }),
		"touch":            spell(func(d *SpellDetails) { d.Range = SpellRange{Kind: RangeTouch} }),
		"self":             spell(func(d *SpellDetails) { d.Range = SpellRange{Kind: RangeSelf} }),
		"ranged 5 ft":      spell(func(d *SpellDetails) { d.Range.DistanceFt = 5 }),
		"ranged 0 ft":      spell(func(d *SpellDetails) { d.Range.DistanceFt = 0 }),
		"no damage":        spell(func(d *SpellDetails) { d.Damage = nil }),
		"instantaneous":    spell(func(d *SpellDetails) { d.Duration = SpellDuration{Kind: DurationInstantaneous} }),
		"rounds":           spell(func(d *SpellDetails) { d.Duration.Unit = DurationRound }),
		"hours":            spell(func(d *SpellDetails) { d.Duration.Unit = DurationHour }),
		"days":             spell(func(d *SpellDetails) { d.Duration.Unit = DurationDay }),
		"bonus action":     spell(func(d *SpellDetails) { d.CastingTime.Unit = CastBonusAction }),
		"two actions":      spell(func(d *SpellDetails) { d.CastingTime.Amount = 2 }),
		"an area":          spell(func(d *SpellDetails) { d.Target = SpellTarget{Kind: TargetArea} }),
		"self target":      spell(func(d *SpellDetails) { d.Target = SpellTarget{Kind: TargetSelf} }),
		"more creatures by slot": spell(func(d *SpellDetails) {
			d.Target.PerSlotLevel = 1
		}),
		"two creatures": spell(func(d *SpellDetails) { d.Target = SpellTarget{Kind: TargetCreatures, Count: 2} }),
		"a cantrip":     spell(func(d *SpellDetails) { d.Spell.Level = 0 }),
	}
	keys := []string{MetamagicCareful, MetamagicDistant, MetamagicEmpowered, MetamagicExtended, MetamagicHeightened, MetamagicQuickened, MetamagicSubtle, MetamagicTwinned, "feature:nope"}
	got := map[string]string{}
	for name, d := range spells {
		for _, slot := range []int{0, 1, 3} {
			var parts []string
			for _, k := range keys {
				parts = append(parts, strings.TrimPrefix(k, "feature:metamagic-")+"="+metamagicReason(k, d, slot))
			}
			got[fmt.Sprintf("%s, slot %d", name, slot)] = strings.Join(parts, " | ")
		}
	}
	goldenText(t, "featchoices-metamagic", got)
}
