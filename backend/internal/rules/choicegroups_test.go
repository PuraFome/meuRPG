package rules

import (
	"slices"
	"strings"
	"testing"
)

// choiceBuild is a character with every score at a playable value, a background
// and one class, for the tests of the choice engine.
func choiceBuild(race, subrace string, cl ClassLevel, picks ...string) Build {
	return Build{
		BaseScores: map[Ability]int{STR: 10, DEX: 14, CON: 14, INT: 12, WIS: 14, CHA: 12},
		Race:       race, Subrace: subrace, Background: "background:acolyte",
		Classes: []ClassLevel{cl}, FeatureChoices: picks,
	}
}

// choiceOf finds a choice by its key, failing the test when the set has none.
func choiceOf(t *testing.T, s ChoiceSet, key string) Choice {
	t.Helper()
	for _, g := range s.Groups {
		for _, ch := range g.Choices {
			if ch.Key == key {
				return ch
			}
		}
	}
	t.Fatalf("no choice %q in %v", key, choiceKeys(s))
	return Choice{}
}

func choiceKeys(s ChoiceSet) []string {
	var keys []string
	for _, g := range s.Groups {
		for _, ch := range g.Choices {
			keys = append(keys, ch.Key)
		}
	}
	return keys
}

func optionKeys(ch Choice) []string {
	keys := make([]string, len(ch.Options))
	for i, o := range ch.Options {
		keys[i] = o.Key
	}
	return keys
}

func optionOf(t *testing.T, ch Choice, key string) ChoiceOption {
	t.Helper()
	for _, o := range ch.Options {
		if o.Key == key {
			return o
		}
	}
	t.Fatalf("choice %q has no option %q", ch.Key, key)
	return ChoiceOption{}
}

// TestChoicesAskForEveryChoiceOfTheClassesAndRaces: each class and race that asks
// the player to pick something gets a choice for it, with the picks the SRD 5.1
// gives at that level and the options it lists (PM-05: the table of the first
// board).
func TestChoicesAskForEveryChoiceOfTheClassesAndRaces(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	styles := func(prefix string, names ...string) []string {
		out := make([]string, len(names))
		for i, n := range names {
			out[i] = prefix + n
		}
		return out
	}
	tests := []struct {
		name  string
		build Build
		key   string
		picks int
		// options are the option keys the choice lists (compared as sets); nil to skip.
		options []string
	}{
		{
			"the fighter's Fighting Style: the six styles of the Fighter", choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: 1}),
			"feature:fighter-fighting-style", 1,
			styles("feature:fighter-fighting-style-", "archery", "defense", "dueling", "great-weapon-fighting", "protection", "two-weapon-fighting"),
		},
		{
			"the paladin's Fighting Style: four, without Archery or Two-Weapon Fighting", choiceBuild("race:human", "", ClassLevel{Class: "class:paladin", Level: 2}),
			"feature:paladin-fighting-style", 1,
			styles("feature:fighting-style-", "defense", "dueling", "great-weapon-fighting", "protection"),
		},
		{
			"the ranger's Fighting Style: four, without Great Weapon Fighting or Protection", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 2}),
			"feature:ranger-fighting-style", 1,
			styles("feature:ranger-fighting-style-", "archery", "defense", "dueling", "two-weapon-fighting"),
		},
		{
			"the champion's second Fighting Style", choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Subclass: "subclass:champion", Level: 10}),
			"feature:additional-fighting-style", 1, nil,
		},
		{
			"the dragonborn's Draconic Ancestry: ten dragons", choiceBuild("race:dragonborn", "", ClassLevel{Class: "class:fighter", Level: 1}),
			"trait:draconic-ancestry", 1, styles("trait:draconic-ancestry-", "black", "blue", "brass", "bronze", "copper", "gold", "green", "red", "silver", "white"),
		},
		{
			"the Draconic sorcerer's Dragon Ancestor", choiceBuild("race:human", "", ClassLevel{Class: "class:sorcerer", Subclass: "subclass:draconic", Level: 1}),
			"feature:dragon-ancestor", 1, nil,
		},
		{
			"the warlock's Pact Boon at 3", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 3}),
			"feature:pact-boon", 1,
			[]string{"feature:pact-of-the-chain", "feature:pact-of-the-blade", "feature:pact-of-the-tome"},
		},
		{
			"two invocations at warlock 2", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 2}),
			"feature:eldritch-invocations", 2, nil,
		},
		{
			"three invocations at warlock 5", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 5}),
			"feature:eldritch-invocations", 3, nil,
		},
		{
			"four invocations at warlock 7", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 7}),
			"feature:eldritch-invocations", 4, nil,
		},
		{
			"five invocations at warlock 9", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 9}),
			"feature:eldritch-invocations", 5, nil,
		},
		{
			"six invocations at warlock 12", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 12}),
			"feature:eldritch-invocations", 6, nil,
		},
		{
			"seven invocations at warlock 15", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 15}),
			"feature:eldritch-invocations", 7, nil,
		},
		{
			"eight invocations at warlock 18", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 18}),
			"feature:eldritch-invocations", 8, nil,
		},
		{
			"two metamagic options at sorcerer 3", choiceBuild("race:human", "", ClassLevel{Class: "class:sorcerer", Level: 3}),
			"feature:metamagic-1", 2, styles("feature:metamagic-", "careful-spell", "distant-spell", "empowered-spell", "extended-spell", "heightened-spell", "quickened-spell", "subtle-spell", "twinned-spell"),
		},
		{
			"a third metamagic option at sorcerer 10", choiceBuild("race:human", "", ClassLevel{Class: "class:sorcerer", Level: 10}),
			"feature:metamagic-2", 1, nil,
		},
		{
			"a fourth metamagic option at sorcerer 17", choiceBuild("race:human", "", ClassLevel{Class: "class:sorcerer", Level: 17}),
			"feature:metamagic-3", 1, nil,
		},
		{
			"the Hunter's Prey at 3", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Subclass: "subclass:hunter", Level: 3}),
			"feature:hunters-prey", 1, styles("feature:hunters-prey-", "colossus-slayer", "giant-killer", "horde-breaker"),
		},
		{
			"the Hunter's Defensive Tactics at 7", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Subclass: "subclass:hunter", Level: 7}),
			"feature:defensive-tactics", 1, styles("feature:defensive-tactics-", "escape-the-horde", "multiattack-defense", "steel-will"),
		},
		{
			"the Hunter's Multiattack at 11: two options", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Subclass: "subclass:hunter", Level: 11}),
			"feature:multiattack", 1, styles("feature:multiattack-", "volley", "whirlwind-attack"),
		},
		{
			"the Hunter's Superior Defense at 15", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Subclass: "subclass:hunter", Level: 15}),
			"feature:superior-hunters-defense", 1, styles("feature:superior-hunters-defense-", "evasion", "stand-against-the-tide", "uncanny-dodge"),
		},
		{
			"the Land druid's terrain: seven", choiceBuild("race:human", "", ClassLevel{Class: "class:druid", Subclass: "subclass:land", Level: 2}),
			"feature:circle-of-the-land", 1, styles("feature:circle-of-the-land-", "arctic", "coast", "desert", "forest", "grassland", "mountain", "swamp"),
		},
		{
			"the Land druid's extra cantrip", choiceBuild("race:human", "", ClassLevel{Class: "class:druid", Subclass: "subclass:land", Level: 2}),
			"feature:bonus-cantrip", 1, nil,
		},
		{
			"the high elf's wizard cantrip", choiceBuild("race:elf", "subrace:high-elf", ClassLevel{Class: "class:wizard", Level: 1}),
			"trait:high-elf-cantrip", 1, nil,
		},
		{
			"the half-elf's two abilities", choiceBuild("race:half-elf", "", ClassLevel{Class: "class:fighter", Level: 1}),
			AbilityChoiceKey("race:half-elf"), 2,
			[]string{"ability:str", "ability:dex", "ability:con", "ability:int", "ability:wis"},
		},
		{
			"the first favored enemy", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 1}),
			"feature:favored-enemy-1-type", 1, nil,
		},
		{
			"the first favored enemy's language", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 1}),
			"feature:favored-enemy-1-type#language", 1, nil,
		},
		{
			"the first favored terrain", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 1}),
			"feature:natural-explorer-1-terrain-type", 1,
			[]string{"terrain:arctic", "terrain:coast", "terrain:desert", "terrain:forest", "terrain:grassland", "terrain:mountain", "terrain:swamp"},
		},
		{
			"the second favored enemy at 6", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 6}),
			"feature:favored-enemy-2-types", 1, nil,
		},
		{
			"the second terrain at 6", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 6}),
			"feature:natural-explorer-2-terrain-types", 1, nil,
		},
		{
			"the third terrain at 10", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 10}),
			"feature:natural-explorer-3-terrain-types", 1, nil,
		},
		{
			"the third favored enemy at 14", choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 14}),
			"feature:favored-enemy-3-enemies", 1, nil,
		},
		{
			"the Mystic Arcanum of the 6th circle at 11", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 11}),
			"feature:mystic-arcanum-6th-level", 1, nil,
		},
		{
			"the Mystic Arcanum of the 7th circle at 13", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 13}),
			"feature:mystic-arcanum-7th-level", 1, nil,
		},
		{
			"the Mystic Arcanum of the 8th circle at 15", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 15}),
			"feature:mystic-arcanum-8th-level", 1, nil,
		},
		{
			"the Mystic Arcanum of the 9th circle at 17", choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 17}),
			"feature:mystic-arcanum-9th-level", 1, nil,
		},
		{
			"the Spell Mastery of a 1st-level spell at wizard 18", choiceBuild("race:human", "", ClassLevel{Class: "class:wizard", Level: 18}),
			"feature:spell-mastery#1", 1, nil,
		},
		{
			"the Spell Mastery of a 2nd-level spell at wizard 18", choiceBuild("race:human", "", ClassLevel{Class: "class:wizard", Level: 18}),
			"feature:spell-mastery#2", 1, nil,
		},
		{
			"two Signature Spells at wizard 20", choiceBuild("race:human", "", ClassLevel{Class: "class:wizard", Level: 20}),
			"feature:signature-spell", 2, nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ch := choiceOf(t, c.Choices(tt.build), tt.key)
			if ch.Picks != tt.picks {
				t.Errorf("picks = %d, want %d", ch.Picks, tt.picks)
			}
			if tt.options != nil {
				got := optionKeys(ch)
				// A list that leads with "Nenhum" or "Duas raças" is not among these.
				if !sameSet(got, tt.options) {
					t.Errorf("options = %v, want %v", got, tt.options)
				}
			}
			if ch.Missing() != tt.picks {
				t.Errorf("missing = %d with nothing picked, want %d", ch.Missing(), tt.picks)
			}
		})
	}
}

func sameSet(a, b []string) bool {
	x, y := slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b))
	return slices.Equal(x, y)
}

// TestChoicesAreAskedOnlyWhenTheSheetHasThem: nothing is asked of a character the
// features do not reach (no choices for a human rogue at level 1, none of the
// subclass's before the subclass is chosen).
func TestChoicesAreAskedOnlyWhenTheSheetHasThem(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	if s := c.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:rogue", Level: 1})); len(s.Groups) != 0 {
		t.Errorf("a human rogue 1 is asked %v, want nothing", choiceKeys(s))
	}
	if s := c.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 5})); slices.Contains(choiceKeys(s), "feature:hunters-prey") {
		t.Error("the Hunter's Prey is asked before the Hunter is chosen")
	}
	if s := c.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 2})); slices.Contains(choiceKeys(s), "feature:pact-boon") {
		t.Error("the Pact Boon is asked at warlock 2, a level before the class has it")
	}
	if s := c.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:sorcerer", Level: 2})); slices.Contains(choiceKeys(s), "feature:metamagic-1") {
		t.Error("Metamagic is asked at sorcerer 2")
	}
	// A third of the Mystic Arcanum needs the level.
	if s := c.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 12})); slices.Contains(choiceKeys(s), "feature:mystic-arcanum-7th-level") {
		t.Error("the 7th-circle arcanum is asked at warlock 12")
	}
}

// TestChoiceGroupsAreOrderedByRaceThenClassLevelThenSubclass: the step lists the race
// first, then each class from its lowest level, a class's own features before its
// subclass's at the same level; the labels say where each choice came from.
func TestChoiceGroupsAreOrderedByRaceThenClassLevelThenSubclass(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := choiceBuild("race:half-elf", "", ClassLevel{Class: "class:ranger", Subclass: "subclass:hunter", Level: 5})
	s := c.Choices(b)
	want := []string{
		AbilityChoiceKey("race:half-elf"),
		"feature:favored-enemy-1-type", "feature:favored-enemy-1-type#language", "feature:natural-explorer-1-terrain-type",
		"feature:ranger-fighting-style", "feature:hunters-prey",
	}
	if got := choiceKeys(s); !slices.Equal(got, want) {
		t.Fatalf("choices = %v, want %v", got, want)
	}
	labels := map[string]string{
		AbilityChoiceKey("race:half-elf"):         "+1 em duas habilidades (Meio-elfo)",
		"feature:ranger-fighting-style":           "Estilo de Luta (Patrulheiro, nível 2)",
		"feature:hunters-prey":                    "Presa do Caçador (Caçador, nível 3)",
		"feature:favored-enemy-1-type#language":   "Inimigo Favorito: idioma que falam (Patrulheiro, nível 1)",
		"feature:natural-explorer-1-terrain-type": "Explorador Natural (Patrulheiro, nível 1)",
	}
	for key, label := range labels {
		if got := choiceOf(t, s, key).LabelPT; got != label {
			t.Errorf("label of %s = %q, want %q", key, got, label)
		}
	}
	done, total := s.Counts()
	if done != 0 || total != 7 {
		t.Errorf("counts = %d of %d, want 0 of 7 (two abilities, enemy and language, terrain, style, prey)", done, total)
	}
}

// TestChoiceCountsFollowTheSelectionsMade: "Escolhas feitas: N de M" counts every
// selection (the half-elf's two abilities, a favored enemy's type and language), and
// a sheet with everything picked has nothing pending.
func TestChoiceCountsFollowTheSelectionsMade(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := choiceBuild("race:half-elf", "", ClassLevel{Class: "class:ranger", Subclass: "subclass:hunter", Level: 5},
		ScopedChoice(AbilityChoiceKey("race:half-elf"), "ability:dex"), ScopedChoice(AbilityChoiceKey("race:half-elf"), "ability:wis"),
		ScopedChoice("feature:favored-enemy-1-type", "creature-type:giant"), ScopedChoice("feature:favored-enemy-1-type#language", "language:giant"),
		ScopedChoice("feature:natural-explorer-1-terrain-type", "terrain:forest"),
		"feature:hunters-prey-colossus-slayer")
	s := c.Choices(b)
	if done, total := s.Counts(); done != 6 || total != 7 {
		t.Errorf("counts = %d of %d, want 6 of 7: the fighting style is open", done, total)
	}
	pending := s.Pending()
	if len(pending) != 1 || pending[0].ChoiceKey != "feature:ranger-fighting-style" || pending[0].Level != 2 || pending[0].LabelPT != "Estilo de Luta (Patrulheiro, nível 2)" {
		t.Errorf("pending = %+v, want the ranger's Fighting Style at level 2", pending)
	}
	b.FeatureChoices = append(b.FeatureChoices, "feature:ranger-fighting-style-archery")
	s = c.Choices(b)
	if done, total := s.Counts(); done != 7 || total != 7 || len(s.Pending()) != 0 || s.PendingCount() != 0 || len(s.ChoiceProblems(true)) != 0 {
		t.Errorf("counts = %d of %d, pending %v, problems %v; want everything done", done, total, s.Pending(), s.ChoiceProblems(true))
	}
}

// TestAnOptionTheSheetCannotTakeStaysInTheListWithItsReason: a prerequisite that is
// not met keeps the option visible, blocked, with the sentence that says what is
// missing (SRD 5.1, Warlock, Eldritch Invocations).
func TestAnOptionTheSheetCannotTakeStaysInTheListWithItsReason(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := choiceBuild("race:tiefling", "", ClassLevel{Class: "class:warlock", Subclass: "subclass:fiend", Level: 5}, "feature:pact-of-the-chain")
	inv := choiceOf(t, c.Choices(b), "feature:eldritch-invocations")
	if len(inv.Options) != 32 {
		t.Fatalf("%d invocations listed, want all 32 of the SRD", len(inv.Options))
	}
	for key, want := range map[string]string{ //nolint:gosec // G101: invocation keys, not credentials
		"feature:eldritch-invocation-agonizing-blast":           "Exige o truque Rajada Mística, que você ainda não escolheu.",
		"feature:eldritch-invocation-bewitching-whispers":       "Exige o nível 7 de Bruxo. Você está no 5.",
		"feature:eldritch-invocation-thirsting-blade":           "Exige o Pacto da Lâmina. Você tem o Pacto da Corrente.",
		"feature:eldritch-invocation-book-of-ancient-secrets":   "Exige o Pacto do Tomo. Você tem o Pacto da Corrente.",
		"feature:eldritch-invocation-lifedrinker":               "Exige o nível 12 de Bruxo e o Pacto da Lâmina. Você está no 5. Você tem o Pacto da Corrente.",
		"feature:eldritch-invocation-voice-of-the-chain-master": "",
		"feature:eldritch-invocation-armor-of-shadows":          "",
	} {
		o := optionOf(t, inv, key)
		if o.ReasonPT != want || o.Blocked() != (want != "") {
			t.Errorf("%s: reason = %q, want %q", key, o.ReasonPT, want)
		}
	}
	// The cantrip lifts the three that ask for it; nothing else changes.
	b.Cantrips = []string{"spell:eldritch-blast"}
	inv = choiceOf(t, c.Choices(b), "feature:eldritch-invocations")
	for _, key := range []string{"agonizing-blast", "eldritch-spear", "repelling-blast"} {
		if o := optionOf(t, inv, "feature:eldritch-invocation-"+key); o.Blocked() {
			t.Errorf("%s is blocked with Eldritch Blast known: %s", key, o.ReasonPT)
		}
	}
	// Without any Pact Boon the sentence says so.
	b.FeatureChoices = nil
	inv = choiceOf(t, c.Choices(b), "feature:eldritch-invocations")
	if got := optionOf(t, inv, "feature:eldritch-invocation-thirsting-blade").ReasonPT; got != "Exige o Pacto da Lâmina. Você ainda não escolheu a Dádiva do Pacto." {
		t.Errorf("without a boon: %q", got)
	}
}

// invocationRows are the prerequisites of the 32 Eldritch Invocations of the SRD 5.1
// (Warlock, "Eldritch Invocations"), written here apart from the data they check.
var invocationRows = []struct {
	key     string
	level   int
	cantrip bool
	pact    string
}{
	{"agonizing-blast", 0, true, ""},
	{"armor-of-shadows", 0, false, ""},
	{"ascendant-step", 9, false, ""},
	{"beast-speech", 0, false, ""},
	{"beguiling-influence", 0, false, ""},
	{"bewitching-whispers", 7, false, ""},
	{"book-of-ancient-secrets", 0, false, "tome"},
	{"chains-of-carceri", 15, false, "chain"},
	{"devils-sight", 0, false, ""},
	{"dreadful-word", 7, false, ""},
	{"eldritch-sight", 0, false, ""},
	{"eldritch-spear", 0, true, ""},
	{"eyes-of-the-rune-keeper", 0, false, ""},
	{"fiendish-vigor", 0, false, ""},
	{"gaze-of-two-minds", 0, false, ""},
	{"lifedrinker", 12, false, "blade"},
	{"mask-of-many-faces", 0, false, ""},
	{"master-of-myriad-forms", 15, false, ""},
	{"minions-of-chaos", 9, false, ""},
	{"mire-the-mind", 5, false, ""},
	{"misty-visions", 0, false, ""},
	{"one-with-shadows", 5, false, ""},
	{"otherworldly-leap", 9, false, ""},
	{"repelling-blast", 0, true, ""},
	{"sculptor-of-flesh", 7, false, ""},
	{"sign-of-ill-omen", 5, false, ""},
	{"thief-of-five-fates", 0, false, ""},
	{"thirsting-blade", 5, false, "blade"},
	{"visions-of-distant-realms", 15, false, ""},
	{"voice-of-the-chain-master", 0, false, "chain"},
	{"whispers-of-the-grave", 9, false, ""},
	{"witch-sight", 15, false, ""},
}

// TestEveryInvocationAsksForExactlyWhatTheSRDSays: one case for each of the 32
// invocations, checked three ways: a sheet that has what it asks can take it, and
// a sheet missing the level, the cantrip or the Pact Boon cannot.
func TestEveryInvocationAsksForExactlyWhatTheSRDSays(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	if len(invocationRows) != 32 {
		t.Fatalf("the table has %d rows, the SRD has 32 invocations", len(invocationRows))
	}
	for _, row := range invocationRows {
		t.Run(row.key, func(t *testing.T) {
			t.Parallel()
			key := "feature:eldritch-invocation-" + row.key
			takable := func(level int, cantrip bool, pact string) (bool, string) {
				b := choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: level})
				if cantrip {
					b.Cantrips = []string{"spell:eldritch-blast"}
				}
				if pact != "" {
					b.FeatureChoices = []string{"feature:pact-of-the-" + pact}
				}
				o := optionOf(t, choiceOf(t, c.Choices(b), "feature:eldritch-invocations"), key)
				return !o.Blocked(), o.ReasonPT
			}
			level := max(row.level, 2)
			everything := "chain"
			if row.pact != "" {
				everything = row.pact
			}
			if ok, why := takable(max(level, 3), row.cantrip, everything); !ok {
				t.Errorf("a sheet with everything it asks cannot take it: %s", why)
			}
			if row.level > 0 {
				if ok, _ := takable(row.level-1, row.cantrip, everything); ok {
					t.Errorf("a warlock %d takes it: it asks level %d", row.level-1, row.level)
				}
			}
			if row.cantrip {
				if ok, _ := takable(max(level, 3), false, everything); ok {
					t.Error("taken without Eldritch Blast")
				}
			}
			if row.pact != "" {
				other := "blade"
				if row.pact == "blade" {
					other = "tome"
				}
				if ok, _ := takable(max(level, 3), row.cantrip, other); ok {
					t.Errorf("taken with the Pact of the %s: it asks the %s", other, row.pact)
				}
				if ok, _ := takable(max(level, 3), row.cantrip, ""); ok {
					t.Error("taken before any Pact Boon")
				}
			}
		})
	}
	// The data and the table agree on every key.
	got := map[string]bool{}
	for _, o := range choiceOf(t, c.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 2})), "feature:eldritch-invocations").Options {
		got[o.Key] = true
	}
	for _, row := range invocationRows {
		if !got["feature:eldritch-invocation-"+row.key] {
			t.Errorf("%s is not among the invocations the engine lists", row.key)
		}
	}
}

// TestSwitchingThePactBoonUnmeetsTheInvocationsThatNeedIt: the invocation that asks
// the Pact of the Chain is unmet once the draft takes the Pact of the Blade, and the
// engine says which pick it is.
func TestSwitchingThePactBoonUnmeetsTheInvocationsThatNeedIt(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	voice := "feature:eldritch-invocation-voice-of-the-chain-master"
	b := choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 5}, "feature:pact-of-the-chain", voice, "feature:eldritch-invocation-armor-of-shadows")
	if inv := choiceOf(t, c.Choices(b), "feature:eldritch-invocations"); len(inv.Unmet) != 0 {
		t.Fatalf("with the Chain the invocations %v are unmet", inv.Unmet)
	}
	b.FeatureChoices[0] = "feature:pact-of-the-blade"
	s := c.Choices(b)
	inv := choiceOf(t, s, "feature:eldritch-invocations")
	if !slices.Equal(inv.Unmet, []string{voice}) {
		t.Errorf("unmet = %v, want only %s", inv.Unmet, voice)
	}
	problems := s.ChoiceProblems(false)
	if len(problems) != 1 || problems[0].Code != ChoiceProblemPrerequisite || problems[0].OptionKey != voice || problems[0].ChoiceKey != "feature:eldritch-invocations" {
		t.Errorf("problems = %+v, want one unmet prerequisite for %s", problems, voice)
	}
}

// TestAChampionsSecondStyleCannotRepeatTheFirst: SRD 5.1, Fighter, Fighting Style: a
// style cannot be taken twice. The picks of the two features share their options and
// are dealt in the order of the levels.
func TestAChampionsSecondStyleCannotRepeatTheFirst(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	prefix := "feature:fighter-fighting-style-"
	b := choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Subclass: "subclass:champion", Level: 10}, prefix+"defense")
	s := c.Choices(b)
	first, second := choiceOf(t, s, "feature:fighter-fighting-style"), choiceOf(t, s, "feature:additional-fighting-style")
	if !slices.Equal(first.Picked, []string{prefix + "defense"}) || len(second.Picked) != 0 || second.Missing() != 1 {
		t.Fatalf("first %v, second %v (missing %d), want the Defense in the first and the second open", first.Picked, second.Picked, second.Missing())
	}
	if o := optionOf(t, second, prefix+"defense"); !o.Blocked() || o.Prerequisites[0].Kind != PrerequisiteTaken {
		t.Errorf("Defense in the second choice = %+v, want it blocked as taken", o)
	}
	if o := optionOf(t, second, prefix+"dueling"); o.Blocked() {
		t.Errorf("Dueling is blocked in the second choice: %s", o.ReasonPT)
	}
	b.FeatureChoices = append(b.FeatureChoices, prefix+"dueling")
	s = c.Choices(b)
	if second := choiceOf(t, s, "feature:additional-fighting-style"); !slices.Equal(second.Picked, []string{prefix + "dueling"}) {
		t.Errorf("second = %v, want Dueling", second.Picked)
	}
	// A pick past what the two take is an overflow, not a silent extra.
	b.FeatureChoices = append(b.FeatureChoices, prefix+"archery")
	s = c.Choices(b)
	if p := s.ChoiceProblems(false); len(p) != 1 || p[0].Code != ChoiceProblemNotOffered || p[0].OptionKey != prefix+"archery" {
		t.Errorf("problems = %+v, want the third style refused", p)
	}
}

// TestMetamagicIsDealtAcrossTheLevelsItComesAt: two options at 3, a third at 10, a
// fourth at 17 (SRD 5.1, Sorcerer table), one shared pool.
func TestMetamagicIsDealtAcrossTheLevelsItComesAt(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	careful, distant, empowered := "feature:metamagic-careful-spell", "feature:metamagic-distant-spell", "feature:metamagic-empowered-spell"
	b := choiceBuild("race:human", "", ClassLevel{Class: "class:sorcerer", Level: 10}, careful, distant)
	s := c.Choices(b)
	if m1, m2 := choiceOf(t, s, "feature:metamagic-1"), choiceOf(t, s, "feature:metamagic-2"); m1.Missing() != 0 || m2.Missing() != 1 {
		t.Errorf("missing %d and %d, want the first done and the level 10 option open", m1.Missing(), m2.Missing())
	}
	b.FeatureChoices = append(b.FeatureChoices, empowered)
	s = c.Choices(b)
	if len(s.Pending()) != 0 {
		t.Errorf("pending %v with three options at level 10", s.Pending())
	}
	if m2 := choiceOf(t, s, "feature:metamagic-2"); !slices.Equal(m2.Picked, []string{empowered}) {
		t.Errorf("the level 10 choice holds %v, want the third pick", m2.Picked)
	}
}

// TestScopedPicksAnswerTheChoiceTheyNameAndNothingElse: a pick is bound to its choice
// by its key, so the same value in two choices is two picks, a pick for a choice the
// sheet does not have is not offered, and a value past the picks overflows.
func TestScopedPicksAnswerTheChoiceTheyNameAndNothingElse(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	enemy1, enemy2 := "feature:favored-enemy-1-type", "feature:favored-enemy-2-types"
	b := choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 6},
		ScopedChoice(enemy1, "creature-type:giant"), ScopedChoice(enemy1+"#language", "language:giant"),
		ScopedChoice(enemy2, "creature-type:undead"), ScopedChoice(enemy2+"#language", LanguageNone),
		ScopedChoice("feature:natural-explorer-1-terrain-type", "terrain:forest"),
		ScopedChoice("feature:natural-explorer-2-terrain-types", "terrain:swamp"))
	s := c.Choices(b)
	if got := choiceOf(t, s, enemy2+"#language"); !slices.Equal(got.Picked, []string{LanguageNone}) {
		t.Errorf("the second enemy's language = %v, want none", got.Picked)
	}
	if got := choiceOf(t, s, enemy1+"#language"); !slices.Equal(got.Picked, []string{"language:giant"}) {
		t.Errorf("the first enemy's language = %v, want Giant", got.Picked)
	}
	if len(s.NotOffered) != 0 {
		t.Errorf("not offered: %v", s.NotOffered)
	}

	// A pick for a choice the sheet does not have, one whose value is not an option,
	// and a second value for a one-pick choice.
	b.FeatureChoices = append(b.FeatureChoices,
		ScopedChoice("feature:favored-enemy-3-enemies", "creature-type:beast"),
		ScopedChoice(enemy1, "creature-type:nonsense"),
		ScopedChoice("feature:natural-explorer-1-terrain-type", "terrain:desert"))
	s = c.Choices(b)
	wantGone := []string{
		ScopedChoice("feature:favored-enemy-3-enemies", "creature-type:beast"),
		ScopedChoice(enemy1, "creature-type:nonsense"),
	}
	if !sameSet(s.NotOffered, wantGone) {
		t.Errorf("not offered = %v, want %v", s.NotOffered, wantGone)
	}
	terrain := choiceOf(t, s, "feature:natural-explorer-1-terrain-type")
	if !slices.Equal(terrain.Picked, []string{"terrain:forest"}) || !slices.Equal(terrain.Overflow, []string{"terrain:desert"}) {
		t.Errorf("terrain picked %v overflow %v, want Forest and Desert past it", terrain.Picked, terrain.Overflow)
	}
}

// TestAFavoredEnemyTypeOrTerrainCannotBeTakenTwice: each new favored enemy and each
// new terrain is another one (SRD 5.1, Ranger: "additional favored enemies",
// "additional favored terrain types").
func TestAFavoredEnemyTypeOrTerrainCannotBeTakenTwice(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	enemy1, enemy2 := "feature:favored-enemy-1-type", "feature:favored-enemy-2-types"
	b := choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 6},
		ScopedChoice(enemy1, "creature-type:giant"), ScopedChoice(enemy2, "creature-type:giant"))
	s := c.Choices(b)
	second := choiceOf(t, s, enemy2)
	if o := optionOf(t, second, "creature-type:giant"); !o.Blocked() {
		t.Error("Giant is not blocked in the second enemy's list")
	}
	if len(second.Unmet) != 1 {
		t.Errorf("unmet = %v, want the repeated Giant", second.Unmet)
	}
}

// TestTheHumanoidFavoredEnemyNeedsTwoRaces: SRD 5.1, Ranger, Favored Enemy: "or two
// races of humanoid". The type is done only when both races are written.
func TestTheHumanoidFavoredEnemyNeedsTwoRaces(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	key := "feature:favored-enemy-1-type"
	b := choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 1}, ScopedChoice(key, "creature-type:humanoid"))
	ch := choiceOf(t, c.Choices(b), key)
	if ch.Missing() != 1 || len(ch.Texts) != 2 {
		t.Fatalf("humanoid without races: missing %d, texts %v; want it open with two empty texts", ch.Missing(), ch.Texts)
	}
	b.FeatureChoiceText = map[string]string{ChoiceTextKey(key, 1): "gnolls", ChoiceTextKey(key, 2): " "}
	if ch = choiceOf(t, c.Choices(b), key); ch.Missing() != 1 {
		t.Errorf("one race written: missing %d, want 1", ch.Missing())
	}
	b.FeatureChoiceText[ChoiceTextKey(key, 2)] = "orcs"
	ch = choiceOf(t, c.Choices(b), key)
	if ch.Missing() != 0 || !slices.Equal(ch.Texts, []string{"gnolls", "orcs"}) {
		t.Errorf("both races: missing %d, texts %v; want done with gnolls and orcs", ch.Missing(), ch.Texts)
	}
	// A type that is not the humanoid has no texts.
	b.FeatureChoices = []string{ScopedChoice(key, "creature-type:beast")}
	if ch = choiceOf(t, c.Choices(b), key); ch.Texts != nil || ch.Missing() != 0 {
		t.Errorf("a beast has texts %v, missing %d", ch.Texts, ch.Missing())
	}
}

// TestTheCreatureTypesUseTheNamesOfTheBestiary: the labels of the Favored Enemy are
// the ones the bestiary shows (web/src/app/core/creatures/creature-types.ts).
func TestTheCreatureTypesUseTheNamesOfTheBestiary(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	want := map[string]string{
		"aberration": "Aberração", "beast": "Fera", "celestial": "Celestial", "construct": "Constructo", "dragon": "Dragão", //nolint:misspell // "Constructo" is the Portuguese term
		"elemental": "Elemental", "fey": "Fada", "fiend": "Ínfero", "giant": "Gigante", "humanoid": "Humanoide",
		"monstrosity": "Monstruosidade", "ooze": "Limo", "plant": "Planta", "undead": "Morto-vivo",
	}
	for key, name := range want {
		if got := c.c.namePT("creature-type:" + key); got != name {
			t.Errorf("creature-type:%s = %q, want %q", key, got, name)
		}
	}
	enemy := choiceOf(t, c.Choices(choiceBuild("race:human", "", ClassLevel{Class: "class:ranger", Level: 1})), "feature:favored-enemy-1-type")
	if len(enemy.Options) != 14 {
		t.Fatalf("%d options, want the 13 types and the humanoid races", len(enemy.Options))
	}
	last := enemy.Options[13]
	if last.Key != "creature-type:humanoid" || !last.NeedsText || last.NamePT != "Duas raças de humanoides…" {
		t.Errorf("last option = %+v, want the two humanoid races", last)
	}
	var names []string
	for _, o := range enemy.Options[:13] {
		names = append(names, o.NamePT)
	}
	if !slices.IsSortedFunc(names, func(a, b string) int { return strings.Compare(foldPT(a), foldPT(b)) }) {
		t.Errorf("the 13 types are not in alphabetical order: %v", names)
	}
}

// TestTheLandTerrainShowsTheCircleSpellsItGives: SRD 5.1, Druid, Circle of the Land:
// the terrain decides the spells of the circle, at druid levels 3, 5, 7 and 9.
func TestTheLandTerrainShowsTheCircleSpellsItGives(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := choiceBuild("race:human", "", ClassLevel{Class: "class:druid", Subclass: "subclass:land", Level: 5})
	terrain := choiceOf(t, c.Choices(b), "feature:circle-of-the-land")
	forest := optionOf(t, terrain, "feature:circle-of-the-land-forest")
	var got []string
	for _, s := range forest.CircleSpells {
		got = append(got, s.Key)
		if s.Reached != (s.Level <= 5) {
			t.Errorf("%s at druid %d reached = %v at level 5", s.Key, s.Level, s.Reached)
		}
	}
	want := []string{"spell:barkskin", "spell:spider-climb", "spell:call-lightning", "spell:plant-growth", "spell:divination", "spell:freedom-of-movement", "spell:commune-with-nature", "spell:tree-stride"}
	if !sameSet(got, want) {
		t.Errorf("forest circle spells = %v, want %v", got, want)
	}
	if forest.CircleSpells[0].Level != 3 || forest.CircleSpells[len(forest.CircleSpells)-1].Level != 9 {
		t.Errorf("circle spells run from %d to %d, want 3 to 9", forest.CircleSpells[0].Level, forest.CircleSpells[len(forest.CircleSpells)-1].Level)
	}
	if forest.NamePT != "Floresta" || terrain.TitlePT != "Terreno do Círculo" {
		t.Errorf("forest named %q under %q, want Floresta under Terreno do Círculo", forest.NamePT, terrain.TitlePT)
	}
	// The terrain is a level 2 feature of the Land subclass in the class table.
	if row := c.c.subclassLevels["subclass:land"][2]; !slices.Contains(row.Features, "feature:circle-of-the-land") {
		t.Errorf("the Land druid's level 2 features = %v, want Circle of the Land among them", row.Features)
	}
}

// TestTheDragonAncestryTableIsTheSRDs: SRD 5.1, Dragonborn, Draconic Ancestry: each
// dragon's damage type, the area of its breath and the saving throw. The Draconic
// sorcerer's Dragon Ancestor uses the same damage types.
func TestTheDragonAncestryTableIsTheSRDs(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	want := []struct {
		color, damage, shape string
		size, width          int
		save                 Ability
	}{
		{"black", "acid", BreathLine, 30, 5, DEX},
		{"blue", "lightning", BreathLine, 30, 5, DEX},
		{"brass", "fire", BreathLine, 30, 5, DEX},
		{"bronze", "lightning", BreathLine, 30, 5, DEX},
		{"copper", "acid", BreathLine, 30, 5, DEX},
		{"gold", "fire", BreathCone, 15, 0, DEX},
		{"green", "poison", BreathCone, 15, 0, CON},
		{"red", "fire", BreathCone, 15, 0, DEX},
		{"silver", "cold", BreathCone, 15, 0, CON},
		{"white", "cold", BreathCone, 15, 0, CON},
	}
	if len(c.c.choices.ancestryByTrait) != len(want) {
		t.Fatalf("%d ancestries, want %d", len(c.c.choices.ancestryByTrait), len(want))
	}
	for _, w := range want {
		a := c.c.choices.ancestryByTrait["trait:draconic-ancestry-"+w.color]
		if a == nil || a.DamageType != "damage-type:"+w.damage || a.Shape != w.shape || a.SizeFt != w.size || a.WidthFt != w.width || a.Save != string(w.save) {
			t.Errorf("%s = %+v, want %s %s %d by %d, save %s", w.color, a, w.damage, w.shape, w.size, w.width, w.save)
			continue
		}
		if f := c.c.choices.ancestryByFeature["feature:dragon-ancestor-"+w.color+"---"+w.damage+"-damage"]; f != a {
			t.Errorf("the sorcerer's %s ancestor is not the same row", w.color)
		}
	}
}

// TestTheBreathWeaponFollowsTheAncestryAndTheLevel: SRD 5.1, Dragonborn, Breath
// Weapon: the DC is 8 + the Constitution modifier + the proficiency bonus, and the
// damage is 2d6, 3d6 from level 6, 4d6 from 11 and 5d6 from 16.
func TestTheBreathWeaponFollowsTheAncestryAndTheLevel(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, tt := range []struct {
		name    string
		trait   string
		level   int
		con     int
		damage  string
		dice    string
		shape   string
		save    Ability
		dc      int
		resists string
	}{
		{"red at 1", "red", 1, 14, "damage-type:fire", "2d6", BreathCone, DEX, 12, "damage-type:fire"},
		{"brass at 6", "brass", 6, 14, "damage-type:fire", "3d6", BreathLine, DEX, 13, "damage-type:fire"},
		{"green at 11", "green", 11, 16, "damage-type:poison", "4d6", BreathCone, CON, 8 + 3 + 4, "damage-type:poison"},
		{"white at 16", "white", 16, 10, "damage-type:cold", "5d6", BreathCone, CON, 8 + 0 + 5, "damage-type:cold"},
		{"blue at 5", "blue", 5, 12, "damage-type:lightning", "2d6", BreathLine, DEX, 8 + 1 + 3, "damage-type:lightning"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := choiceBuild("race:dragonborn", "", ClassLevel{Class: "class:fighter", Level: tt.level}, "trait:draconic-ancestry-"+tt.trait)
			b.BaseScores[CON] = tt.con - 0 // dragonborn adds no Constitution
			d := Derive(b, c)
			bw := d.BreathWeapon
			if bw == nil {
				t.Fatal("no breath weapon")
			}
			if bw.DamageType != tt.damage || bw.Dice != tt.dice || bw.Shape != tt.shape || bw.SaveAbility != tt.save || bw.DC != tt.dc {
				t.Errorf("breath weapon = %+v, want %s %s %s save %s DC %d", *bw, tt.dice, tt.damage, tt.shape, tt.save, tt.dc)
			}
			if len(d.Resistances) != 1 || d.Resistances[0].DamageType != tt.resists || d.Resistances[0].SourceKey != "trait:draconic-ancestry-"+tt.trait {
				t.Errorf("resistances = %+v, want only %s from the ancestry", d.Resistances, tt.resists)
			}
		})
	}
	// The text says the numbers of the draft.
	b := choiceBuild("race:dragonborn", "", ClassLevel{Class: "class:fighter", Level: 1}, "trait:draconic-ancestry-red")
	text := c.BreathWeaponText(Derive(b, c))
	for _, want := range []string{"2d6 de fogo num cone de 4,5 m", "teste de resistência de Destreza", "CD 12 com Constituição 14 e proficiência +2", "3d6 no nível 6, 4d6 no nível 11 e 5d6 no nível 16", "Resistência a dano de fogo"} {
		if !strings.Contains(text, want) {
			t.Errorf("breath weapon text lacks %q: %s", want, text)
		}
	}
	// No ancestry yet, no breath weapon.
	if d := Derive(choiceBuild("race:dragonborn", "", ClassLevel{Class: "class:fighter", Level: 1}), c); d.BreathWeapon != nil || len(d.Resistances) != 0 {
		t.Errorf("breath weapon %+v and resistances %v without an ancestry", d.BreathWeapon, d.Resistances)
	}
	// The line of a brass dragon, in feet and in the app's meters.
	if got := breathAreaPT(&BreathWeapon{Shape: BreathLine, SizeFt: 30, WidthFt: 5}); got != "linha de 9 m por 1,5 m" {
		t.Errorf("line = %q", got)
	}
}

// TestRacesThatResistADamageTypeSayWhich: the tiefling (fire) and the dwarf (poison)
// resist by their traits (SRD 5.1, Tiefling, Hellish Resistance; Dwarf, Dwarven
// Resilience); a human resists nothing.
func TestRacesThatResistADamageTypeSayWhich(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for race, want := range map[string]string{"race:tiefling": "damage-type:fire", "race:dwarf": "damage-type:poison"} {
		b := choiceBuild(race, "", ClassLevel{Class: "class:fighter", Level: 1})
		if race == "race:dwarf" {
			b.Subrace = "subrace:hill-dwarf"
		}
		d := Derive(b, c)
		if len(d.Resistances) != 1 || d.Resistances[0].DamageType != want {
			t.Errorf("%s resistances = %+v, want %s", race, d.Resistances, want)
		}
	}
	if d := Derive(choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: 1}), c); len(d.Resistances) != 0 {
		t.Errorf("a human resists %v", d.Resistances)
	}
}

// TestTheHalfElfsAbilitiesRaiseTheScoresAndLeaveTheManualBonuses: SRD 5.1, Half-Elf,
// Ability Score Increase: +2 Charisma and +1 to two other abilities. The picks add
// to the race bonus, and the manual bonuses are not used for them.
func TestTheHalfElfsAbilitiesRaiseTheScoresAndLeaveTheManualBonuses(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	key := AbilityChoiceKey("race:half-elf")
	b := choiceBuild("race:half-elf", "", ClassLevel{Class: "class:fighter", Level: 1}, ScopedChoice(key, "ability:dex"), ScopedChoice(key, "ability:wis"))
	d := Derive(b, c)
	if dex, wis, cha, con := abilityOf(d, DEX), abilityOf(d, WIS), abilityOf(d, CHA), abilityOf(d, CON); dex.Score != 15 || dex.RaceBonus != 1 || dex.ManualBonus != 0 || wis.Score != 15 || cha.Score != 14 || con.Score != 14 {
		t.Errorf("scores dex %+v wis %+v cha %+v con %+v, want +1 in Dexterity and Wisdom, +2 in Charisma", dex, wis, cha, con)
	}
	for _, h := range d.Hints {
		if h.Source == "race:half-elf" {
			t.Errorf("the reminder to place the +1 is still shown with both picked: %s", h.TextPT)
		}
	}
	// With none picked the sheet reminds the player, and the scores wait.
	d = Derive(choiceBuild("race:half-elf", "", ClassLevel{Class: "class:fighter", Level: 1}), c)
	if hint, ok := hintFrom(d, "race:half-elf"); !ok || !strings.Contains(hint.TextPT, "Escolhas") {
		t.Errorf("hint = %+v, want the reminder to choose in the Escolhas step", hint)
	}
	if abilityOf(d, DEX).Score != 14 {
		t.Errorf("Dexterity = %d without picks, want 14", abilityOf(d, DEX).Score)
	}
	// Charisma is not offered, and a third pick does not count.
	ch := choiceOf(t, c.Choices(b), key)
	if slices.Contains(optionKeys(ch), "ability:cha") {
		t.Error("Charisma is offered for the +1")
	}
	b.FeatureChoices = append(b.FeatureChoices, ScopedChoice(key, "ability:str"))
	if p := c.Choices(b).ChoiceProblems(false); len(p) != 1 || p[0].Code != ChoiceProblemNotOffered || p[0].OptionKey != "ability:str" {
		t.Errorf("problems = %+v, want the third ability refused", p)
	}
	if d := Derive(b, c); abilityOf(d, STR).Score != 10 {
		t.Errorf("Strength = %d with a third pick, want 10: only two count", abilityOf(d, STR).Score)
	}
}

// TestTheSpellsAFeatureGrantsJoinTheSheetOutsideTheClassNumbers: the high elf's
// cantrip, the Pact of the Tome's three and a Mystic Arcanum are spells the sheet
// knows besides the class's own numbers.
func TestTheSpellsAFeatureGrantsJoinTheSheetOutsideTheClassNumbers(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	hasSpell := func(d Derived, key string) bool {
		return slices.ContainsFunc(d.Spells, func(s CharacterSpell) bool { return s.Spell.Key == key })
	}
	t.Run("the high elf's cantrip", func(t *testing.T) {
		t.Parallel()
		b := choiceBuild("race:elf", "subrace:high-elf", ClassLevel{Class: "class:wizard", Level: 1}, ScopedChoice("trait:high-elf-cantrip", "spell:mage-hand"))
		b.Cantrips = []string{"spell:fire-bolt", "spell:light", "spell:prestidigitation"}
		d := Derive(b, c)
		if !hasSpell(d, "spell:mage-hand") {
			t.Error("the high elf's cantrip is not in the spell list")
		}
		for _, is := range d.Issues {
			if is.Code == IssueSpellCount || is.Code == IssueSpellNotOnList {
				t.Errorf("issue %s: %s", is.Code, is.Message)
			}
		}
		// Four class cantrips on top of the elf's is one too many.
		b.Cantrips = append(b.Cantrips, "spell:ray-of-frost")
		if !slices.ContainsFunc(Derive(b, c).Issues, func(is Issue) bool { return is.Code == IssueSpellCount }) {
			t.Error("a fourth class cantrip next to the elf's is not an issue")
		}
	})
	t.Run("a sheet that kept the elf's cantrip in the list counts it as done", func(t *testing.T) {
		t.Parallel()
		b := choiceBuild("race:elf", "subrace:high-elf", ClassLevel{Class: "class:wizard", Level: 1})
		b.Cantrips = []string{"spell:fire-bolt", "spell:light", "spell:prestidigitation", "spell:mage-hand"}
		s := c.Choices(b)
		if ch := choiceOf(t, s, "trait:high-elf-cantrip"); ch.Missing() != 0 {
			t.Errorf("missing %d: the fourth cantrip of the list is the elf's", ch.Missing())
		}
		b.Cantrips = b.Cantrips[:3]
		if ch := choiceOf(t, c.Choices(b), "trait:high-elf-cantrip"); ch.Missing() != 1 {
			t.Errorf("missing %d with only the class's three cantrips, want 1", ch.Missing())
		}
	})
	t.Run("the Pact of the Tome's cantrips", func(t *testing.T) {
		t.Parallel()
		key := "feature:pact-of-the-tome#cantrips"
		b := choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 3}, "feature:pact-of-the-tome",
			ScopedChoice(key, "spell:guidance"), ScopedChoice(key, "spell:shillelagh"), ScopedChoice(key, "spell:eldritch-blast"))
		b.Cantrips = []string{"spell:chill-touch", "spell:mage-hand"}
		d := Derive(b, c)
		for _, k := range []string{"spell:guidance", "spell:shillelagh", "spell:eldritch-blast"} {
			if !hasSpell(d, k) {
				t.Errorf("%s is not in the spell list", k)
			}
		}
		for _, is := range d.Issues {
			if is.Code == IssueSpellCount || is.Code == IssueSpellNotOnList {
				t.Errorf("issue %s: %s", is.Code, is.Message)
			}
		}
		ch := choiceOf(t, c.Choices(b), key)
		if ch.Missing() != 0 || len(ch.Picked) != 3 {
			t.Errorf("the Tome's cantrips: missing %d, picked %v", ch.Missing(), ch.Picked)
		}
		// They are warlock cantrips for the sheet, so Eldritch Blast opens its invocations.
		inv := choiceOf(t, c.Choices(b), "feature:eldritch-invocations")
		_ = inv
		w := choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 5}, "feature:pact-of-the-tome", ScopedChoice(key, "spell:eldritch-blast"))
		if o := optionOf(t, choiceOf(t, c.Choices(w), "feature:eldritch-invocations"), "feature:eldritch-invocation-agonizing-blast"); o.Blocked() {
			t.Errorf("Agonizing Blast is blocked with Eldritch Blast from the Tome: %s", o.ReasonPT)
		}
		// The choice appears only with the Tome.
		w.FeatureChoices = []string{"feature:pact-of-the-blade"}
		if slices.Contains(choiceKeys(c.Choices(w)), key) {
			t.Error("the Tome's cantrips are asked with the Pact of the Blade")
		}
	})
	t.Run("a Mystic Arcanum", func(t *testing.T) {
		t.Parallel()
		key := "feature:mystic-arcanum-6th-level"
		b := choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 11}, ScopedChoice(key, "spell:eyebite"))
		d := Derive(b, c)
		if !hasSpell(d, "spell:eyebite") {
			t.Error("the arcanum spell is not in the spell list")
		}
		for _, is := range d.Issues {
			if is.Code == IssueSpellLevel || is.Code == IssueSpellNotOnList {
				t.Errorf("issue %s: %s", is.Code, is.Message)
			}
		}
		ch := choiceOf(t, c.Choices(b), key)
		for _, o := range ch.Options {
			if o.SpellLevel != 6 {
				t.Errorf("%s is a spell of the %dth circle in the 6th-circle arcanum", o.Key, o.SpellLevel)
			}
		}
		if !slices.Contains(optionKeys(ch), "spell:eyebite") {
			t.Error("Eyebite, a 6th-level warlock spell, is not offered")
		}
	})
}

// TestTheArcanumOffersTheWarlockListOnly: the Fiend's expanded list (SRD 5.1, The
// Fiend) stops at the 5th circle, so every arcanum spell is on the class list.
func TestTheArcanumOffersTheWarlockListOnly(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Subclass: "subclass:fiend", Level: 11})
	for _, o := range choiceOf(t, c.Choices(b), "feature:mystic-arcanum-6th-level").Options {
		if !slices.Contains(c.c.spells[o.Key].Classes, "class:warlock") {
			t.Errorf("%s is offered but is not on the warlock list", o.Key)
		}
	}
}

// TestDevilsSightIsAppliedAndTheOtherInvocationsAreReminders: Devil's Sight sees in
// the dark to 120 feet (a sense), Agonizing Blast adds the Charisma modifier to
// Eldritch Blast, and the other passive invocations appear in the features with the
// rule in a line.
func TestDevilsSightIsAppliedAndTheOtherInvocationsAreReminders(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := choiceBuild("race:human", "", ClassLevel{Class: "class:warlock", Level: 5}, "feature:pact-of-the-chain",
		"feature:eldritch-invocation-devils-sight", "feature:eldritch-invocation-agonizing-blast", "feature:eldritch-invocation-armor-of-shadows")
	b.Cantrips = []string{"spell:eldritch-blast", "spell:mage-hand"}
	d := Derive(b, c)
	darkvision := 0
	for _, s := range d.Senses {
		if s.Key == "darkvision" {
			darkvision = s.RangeFt
		}
	}
	if darkvision != 120 {
		t.Errorf("darkvision = %d, want 120 feet from Devil's Sight", darkvision)
	}
	atk, ok := attackOf(d, "spell:eldritch-blast")
	if !ok || atk.Damage != "1d10+1" {
		t.Errorf("Eldritch Blast = %+v, want 1d10+1: the Charisma modifier from Agonizing Blast", atk)
	}
	summaries := map[string]string{}
	for _, f := range d.Features {
		summaries[f.Key] = f.SummaryPT
	}
	if summaries["feature:eldritch-invocation-armor-of-shadows"] != "Conjura Armadura Arcana em você à vontade." {
		t.Errorf("Armor of Shadows summary = %q", summaries["feature:eldritch-invocation-armor-of-shadows"])
	}
	if summaries["feature:pact-of-the-chain"] == "" {
		t.Error("the Pact of the Chain has no summary on the sheet")
	}
}
