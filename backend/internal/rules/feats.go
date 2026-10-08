package rules

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// Feats (MR-025): an optional rule of the game. The SRD 5.1 has one, Grappler
// (SRD 5.1, "Feats"); the table's own are added by the master (TableFeat). A
// character that takes a feat in place of an Ability Score Improvement keeps its
// key in Build.Feats, and Derive treats it like a feature: the feat's effects
// apply and the sheet lists it.
//
// What a feat asks of the character is its prerequisite (FeatPrerequisite). The
// SRD only states ability score minimums; the other conditions (a proficiency,
// spellcasting, a race, a level) are the table's.

// The kinds of FeatUnmet.
const (
	// FeatUnmetAbilityMinimum: an ability score is below the minimum, and every
	// minimum is asked (Abilities has the one that failed).
	FeatUnmetAbilityMinimum = "ability_minimum"
	// FeatUnmetAbilityAnyOf: none of the abilities reaches its minimum (Abilities
	// has all of them: one is enough).
	FeatUnmetAbilityAnyOf = "ability_any_of"
	// FeatUnmetProficiency: the character lacks the proficiency in Key.
	FeatUnmetProficiency = "proficiency"
	// FeatUnmetSpellcasting: the character cannot cast any spell.
	FeatUnmetSpellcasting = "spellcasting"
	// FeatUnmetRace: the character is not of the race or subrace in Key.
	FeatUnmetRace = "race"
	// FeatUnmetLevel: the character level is below Value.
	FeatUnmetLevel = "level"
	// FeatUnmetAbilityCap: the feat raises abilities and fewer than the ones it asks
	// for are below 20 (an ability never goes above 20).
	FeatUnmetAbilityCap = "ability_cap"
)

// FeatPrerequisite is what a feat asks of a character. Every condition that is set
// has to be met. The zero value asks nothing.
type FeatPrerequisite struct {
	// Minimums are ability scores that must all be met, and AnyOf ones of which a
	// single one is enough. A score of 0 is "not asked".
	Minimums map[Ability]int
	AnyOf    map[Ability]int
	// Proficiency is a proficiency key ("proficiency:medium-armor") the character
	// must have.
	Proficiency string
	// Spellcasting asks for the ability to cast at least one spell.
	Spellcasting bool
	// Race is a race or subrace key; the character's race or subrace must be it.
	Race string
	// Level is the least total character level; 0 is not asked.
	Level int
}

// IsZero says the prerequisite asks nothing.
func (p FeatPrerequisite) IsZero() bool {
	return len(p.Minimums) == 0 && len(p.AnyOf) == 0 && p.Proficiency == "" && !p.Spellcasting && p.Race == "" && p.Level == 0
}

// AbilityMinimum is an ability and the score asked of it.
type AbilityMinimum struct {
	Ability Ability
	Minimum int
}

// FeatUnmet is one thing a character lacks to take a feat.
type FeatUnmet struct {
	// Kind is a FeatUnmet* constant.
	Kind string
	// Abilities are the minimums not met (see the kinds).
	Abilities []AbilityMinimum
	// Key is the proficiency or race the character lacks, and Value the level.
	Key   string
	Value int
}

// FeatIncrease is the feat's ability_increase effect: the player chooses Count
// different abilities from From and adds Value to each, none above 20.
type FeatIncrease struct {
	Count int
	From  []Ability
	Value int
}

// FeatEntry is a feat as the screens show it.
type FeatEntry struct {
	Key string
	// Name is the SRD's English name (the table's feats have only NamePT).
	Name, NamePT string
	// Desc is the text: the SRD's, in English, or the table's, in Portuguese.
	Desc         []string
	Prerequisite FeatPrerequisite
	// Increase is the ability increase the feat gives, or nil.
	Increase *FeatIncrease
	// Table says the feat is the table's own; Archived that the table retired it
	// and Off that the master switched it off for the players. Neither is a new
	// choice for a player.
	Table, Archived, Off bool
}

// FeatOption is a feat a character may take, with whether it qualifies and what
// it lacks when it does not.
type FeatOption struct {
	FeatEntry
	Qualifies bool
	Unmet     []FeatUnmet
}

const (
	// abilityScoreImprovementPoints is what an Ability Score Improvement gives in all:
	// +2 to one ability or +1 to two (SRD 5.1, "Ability Score Improvement"). It is also the
	// most an ability_increase adds to each ability it raises.
	abilityScoreImprovementPoints = 2
	// abilityCount is the number of abilities: the most an ability_increase picks.
	abilityCount = 6
)

// featEntry is the FeatEntry of a feat in the content.
func (c *content) featEntry(f *srd51.Feat) FeatEntry {
	e := FeatEntry{
		Key: f.Key, Name: f.Name, NamePT: c.namePT(f.Key), Desc: slices.Clone(f.Desc),
		Prerequisite: FeatPrerequisite{
			Minimums: abilityMap(f.Minimums), AnyOf: abilityMap(f.AnyOf), Proficiency: f.Proficiency,
			Spellcasting: f.Spellcasting, Race: f.Race, Level: f.Level,
		},
		Table: isTableKey(f.Key), Archived: c.archived[f.Key], Off: c.off[f.Key],
	}
	for _, ef := range c.effects[f.Key] {
		if ef.Type == "ability_increase" {
			inc := &FeatIncrease{Count: ef.Count, Value: increaseValue(ef)}
			for _, a := range ef.From {
				inc.From = append(inc.From, Ability(a))
			}
			e.Increase = inc
		}
	}
	return e
}

// increaseValue is the whole number an ability_increase adds (its Value is a
// plain integer; the loader refuses anything else).
func increaseValue(e *Effect) int {
	n, _ := strconv.Atoi(e.Value)
	return n
}

// Feats lists every feat of the content, the SRD's and the table's, sorted by
// Portuguese name.
func (c *Content) Feats() []FeatEntry {
	x := c.c
	out := make([]FeatEntry, 0, len(x.feats))
	for _, k := range sortedKeys(x.feats) {
		out = append(out, x.featEntry(x.feats[k]))
	}
	slices.SortStableFunc(out, func(a, b FeatEntry) int { return comparePT(a.NamePT, b.NamePT) })
	return out
}

// Feat gives one feat of the content by key.
func (c *Content) Feat(key string) (FeatEntry, bool) {
	f, ok := c.c.feats[key]
	if !ok {
		return FeatEntry{}, false
	}
	return c.c.featEntry(f), true
}

// FeatOptions lists the feats a character may take now, each with whether it
// qualifies and, if not, what it lacks. The feats the build already has are left
// out. Retired and switched-off feats are in the list with their marks: the
// caller (the server) leaves them out for a player.
func FeatOptions(b Build, c *Content) []FeatOption { return featOptions(b, c.c) }

func featOptions(b Build, x *content) []FeatOption {
	d := derive(b, x)
	out := make([]FeatOption, 0, len(x.feats))
	for _, e := range (&Content{c: x}).Feats() {
		if slices.Contains(b.Feats, e.Key) {
			continue
		}
		unmet := x.unmetFeat(e, b, d)
		out = append(out, FeatOption{FeatEntry: e, Qualifies: len(unmet) == 0, Unmet: unmet})
	}
	return out
}

// CheckFeat says what a character lacks to take the feat key, or nil when it may.
// An unknown key is an error.
func CheckFeat(b Build, key string, c *Content) ([]FeatUnmet, error) {
	e, ok := c.Feat(key)
	if !ok {
		return nil, fmt.Errorf("rules: unknown feat %q", key)
	}
	return c.c.unmetFeat(e, b, derive(b, c.c)), nil
}

// unmetFeat is what the character lacks of the feat's prerequisite, and the
// abilities the feat raises when not enough of them can still go up.
func (c *content) unmetFeat(e FeatEntry, b Build, d Derived) []FeatUnmet {
	p := e.Prerequisite
	score := map[Ability]int{}
	for _, s := range d.Abilities {
		score[s.Ability] = s.Score
	}
	var out []FeatUnmet
	for _, a := range AllAbilities() {
		if least := p.Minimums[a]; least > 0 && score[a] < least {
			out = append(out, FeatUnmet{Kind: FeatUnmetAbilityMinimum, Abilities: []AbilityMinimum{{Ability: a, Minimum: least}}})
		}
	}
	if len(p.AnyOf) > 0 {
		met := false
		var all []AbilityMinimum
		for _, a := range AllAbilities() {
			if least := p.AnyOf[a]; least > 0 {
				all = append(all, AbilityMinimum{Ability: a, Minimum: least})
				met = met || score[a] >= least
			}
		}
		if !met {
			out = append(out, FeatUnmet{Kind: FeatUnmetAbilityAnyOf, Abilities: all})
		}
	}
	if p.Proficiency != "" && !hasProficiency(d, p.Proficiency) {
		out = append(out, FeatUnmet{Kind: FeatUnmetProficiency, Key: p.Proficiency})
	}
	if p.Spellcasting && len(d.Spellcasting) == 0 && d.PactMagic == nil && len(d.Spells) == 0 {
		out = append(out, FeatUnmet{Kind: FeatUnmetSpellcasting})
	}
	if p.Race != "" && b.Race != p.Race && b.Subrace != p.Race {
		out = append(out, FeatUnmet{Kind: FeatUnmetRace, Key: p.Race})
	}
	if p.Level > 0 && d.TotalLevel < p.Level {
		out = append(out, FeatUnmet{Kind: FeatUnmetLevel, Value: p.Level})
	}
	if inc := e.Increase; inc != nil {
		room := 0
		for _, a := range inc.From {
			if score[a]+inc.Value <= MaxNormalScore {
				room++
			}
		}
		if room < inc.Count {
			out = append(out, FeatUnmet{Kind: FeatUnmetAbilityCap})
		}
	}
	return out
}

// hasProficiency says whether the sheet has the proficiency key, or one that
// covers it: "All armor" (the Fighter's) is proficiency in light, medium and heavy
// armor.
func hasProficiency(d Derived, key string) bool {
	have := func(k string) bool {
		return slices.ContainsFunc(d.Proficiencies, func(pr Proficiency) bool { return pr.Key == k })
	}
	switch key {
	case "proficiency:light-armor", "proficiency:medium-armor", "proficiency:heavy-armor":
		return have(key) || have("proficiency:all-armor")
	}
	return have(key)
}

// checkFeatShape checks a feat's own data: the abilities of its prerequisite are
// real, the keys it names exist and the numbers are in range. owner is the feat's
// key, for the messages; it returns the first problem and the field it is at.
func (c *content) checkFeatShape(f *srd51.Feat) (field, msg string) {
	for _, m := range []struct {
		name string
		set  map[string]int
	}{{"minimums", f.Minimums}, {"any_of", f.AnyOf}} {
		for k, v := range m.set {
			if _, ok := abilityIndex[Ability(k)]; !ok {
				return m.name, fmt.Sprintf("unknown ability %q", k)
			}
			if v < 1 || v > MaxScore {
				return m.name, fmt.Sprintf("the minimum score of %s is 1 to %d", k, MaxScore)
			}
		}
	}
	if f.Proficiency != "" {
		if _, ok := c.proficiencies[f.Proficiency]; !ok {
			return "proficiency", fmt.Sprintf("unknown proficiency %q", f.Proficiency)
		}
	}
	if f.Race != "" {
		_, race := c.races[f.Race]
		_, subrace := c.subraces[f.Race]
		if !race && !subrace {
			return "race", fmt.Sprintf("unknown race or subrace %q", f.Race)
		}
	}
	if f.Level < 0 || f.Level > MaxLevel {
		return "level", fmt.Sprintf("the level is 0 to %d", MaxLevel)
	}
	return "", ""
}
