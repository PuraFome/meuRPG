package rules

import (
	"slices"
	"strings"
	"testing"
)

// The texts of a player's feat cards name no hidden race, feat or feature (RN-23); the master's keep them.

const (
	hiddenRaceName  = "Anão das Brumas"
	hiddenTraitName = "Pele de Bruma"
	hiddenFeatName  = "Dom Oculto"
	visibleRaceName = "Elfo Escarlate"

	cardOnlyHidden   = "feat:so-das-brumas" + tableSuffix
	cardMixed        = "feat:misto" + tableSuffix
	cardLacksFeat    = "feat:sem-dom" + tableSuffix
	cardLacksTrait   = "feat:sem-pele" + tableSuffix
	cardHiddenFeat   = "feat:dom-oculto" + tableSuffix
	cardOpenFeat     = "feat:aberto" + tableSuffix
	cardVisibleRace  = "race:vale-claro" + tableSuffix
	cardScarletRace  = "race:elfo-escarlate" + tableSuffix
	cardHiddenTrait  = "trait:pele-de-bruma" + tableSuffix
	cardFeatGrantKey = "trait:talento-do-vale" + tableSuffix
)

// hiddenCardsTable is a table whose hidden (switched off) race and feat are named by feats of the content,
// and a visible race whose trait grants a feat of the player's choice.
func hiddenCardsTable(t *testing.T) *Content {
	t.Helper()
	hidden, _, _ := tableMisc()
	hidden.Traits = []TableFeature{{Key: cardHiddenTrait, NamePT: hiddenTraitName, DescPT: []string{"Texto."}}}
	visible, _, _ := tableMisc()
	visible.ChoiceBonuses = nil
	visible.Key, visible.NamePT = cardVisibleRace, "Gnomo do Vale"
	visible.Traits = []TableFeature{{
		Key: cardFeatGrantKey, NamePT: "Talento do vale", DescPT: []string{"Um talento à escolha."},
		Effects: []Effect{{Type: "choice", Choice: "feat", Count: 1}},
	}}
	scarlet, _, _ := tableMisc()
	scarlet.Key, scarlet.NamePT, scarlet.Traits = cardScarletRace, visibleRaceName, nil
	feat := func(key, name string, p FeatPrerequisite) TableFeat {
		return TableFeat{TableEntry: TableEntry{Key: key, NamePT: name}, DescPT: []string{"Texto de " + name + "."}, Prerequisite: p}
	}
	return withOverlay(t, Overlay{
		Revision: 1, Off: []string{hidden.Key, cardHiddenFeat}, Races: []TableRace{hidden, visible, scarlet},
		Feats: []TableFeat{
			feat(cardHiddenFeat, hiddenFeatName, FeatPrerequisite{}),
			feat(cardOnlyHidden, "Linhagem Antiga", FeatPrerequisite{Race: hidden.Key}),
			feat(cardMixed, "Linhagem Mista", FeatPrerequisite{Races: []string{hidden.Key, cardScarletRace}}),
			feat(cardLacksFeat, "Sem o dom", FeatPrerequisite{LacksFeature: cardHiddenFeat}),
			feat(cardLacksTrait, "Sem a pele", FeatPrerequisite{LacksFeature: cardHiddenTrait}),
			feat(cardOpenFeat, "Aberto", FeatPrerequisite{}),
		},
	})
}

func cardsBuild(picks []string, feats ...string) Build {
	b := torenLevelUp()
	b.Race = cardVisibleRace
	b.Classes[0].Level, b.Classes[0].Subclass = 1, ""
	b.Feats = feats
	for _, p := range picks {
		b.FeatureChoices = append(b.FeatureChoices, ScopedChoice(FeatChoiceKey(cardFeatGrantKey), p))
	}
	return b
}

// cardText is every text of the card's options, with the keys of the options.
func cardText(t *testing.T, s ChoiceSet) (string, []string) {
	t.Helper()
	var parts, keys []string
	for _, g := range s.Groups {
		for _, ch := range g.Choices {
			if ch.Key != FeatChoiceKey(cardFeatGrantKey) {
				continue
			}
			for _, o := range ch.Options {
				keys = append(keys, o.Key)
				parts = append(parts, o.NamePT, o.SummaryPT, o.ReasonPT)
				for _, p := range o.Prerequisites {
					parts = append(parts, p.NamePT, p.ReasonPT)
				}
			}
		}
	}
	if len(keys) == 0 {
		t.Fatal("no feat card")
	}
	return strings.Join(parts, "\n"), keys
}

func TestFeatCardsNameNoHiddenEntryToAPlayer(t *testing.T) {
	t.Parallel()
	c := hiddenCardsTable(t)
	// The sheet picked the feat that asks only for the hidden race, is no Elfo Escarlate and holds nothing hidden.
	b := cardsBuild([]string{cardOnlyHidden})
	hiddenWords := []string{hiddenRaceName, hiddenTraitName, hiddenFeatName, "race:anao-das-brumas", "dom-oculto", "pele-de-bruma"}

	master, masterKeys := cardText(t, c.Choices(b))
	for _, w := range []string{hiddenRaceName, hiddenFeatName, hiddenTraitName} { // the positive control
		if !strings.Contains(master, w) {
			t.Errorf("the master's card does not name %q:\n%s", w, master)
		}
	}
	player, playerKeys := cardText(t, c.ForPlayer().Choices(b))
	for _, w := range hiddenWords {
		if strings.Contains(player, w) {
			t.Errorf("the player's card names %q:\n%s", w, player)
		}
	}
	// What the player reads: the visible race only, and a text that names nothing for a condition they may not read.
	for _, want := range []string{"Só para " + visibleRaceName + ".", hiddenUnmetPT, "Pede: " + hiddenAskPT + "."} {
		if !strings.Contains(player, want) {
			t.Errorf("the player's card lacks %q:\n%s", want, player)
		}
	}
	// The feat that asks only for the hidden race, and the hidden feat, are offered to the master only,
	// unless already picked: the pick stays, with a text that names nothing.
	if !slices.Contains(masterKeys, cardOnlyHidden) || !slices.Contains(playerKeys, cardOnlyHidden) {
		t.Errorf("the picked feat is not on the cards: master %v, player %v", masterKeys, playerKeys)
	}
	if slices.Contains(playerKeys, cardHiddenFeat) || !slices.Contains(playerKeys, cardOpenFeat) {
		t.Errorf("the player's feats = %v, want the open one and not the hidden one", playerKeys)
	}
}

func TestFeatCardsLeaveOutAFeatOnlyAHiddenRaceCanTake(t *testing.T) {
	t.Parallel()
	c := hiddenCardsTable(t)
	b := cardsBuild(nil)
	_, masterKeys := cardText(t, c.Choices(b))
	_, playerKeys := cardText(t, c.ForPlayer().Choices(b))
	if !slices.Contains(masterKeys, cardOnlyHidden) || slices.Contains(playerKeys, cardOnlyHidden) {
		t.Errorf("the feat of the hidden race: master %v, player %v; want the master's only", masterKeys, playerKeys)
	}
	// The feat that also asks for a visible race stays, and says only that race.
	text, keys := cardText(t, c.ForPlayer().Choices(b))
	if !slices.Contains(keys, cardMixed) || !strings.Contains(text, "Pede: ser "+visibleRaceName+".") {
		t.Errorf("the player's card of the mixed feat:\n%s", text)
	}
	// The character of the hidden race is shown the race it has.
	own := cardsBuild(nil)
	own.Race = "race:anao-das-brumas" + tableSuffix
	if got := c.ForPlayer().c.asksOnlyHiddenRaces(FeatPrerequisite{Race: own.Race}, own); got {
		t.Error("a feat asking for the sheet's own race is left out")
	}
}

func TestFeatOptionsOfAPlayerLeaveOutAFeatOnlyAHiddenRaceCanTake(t *testing.T) {
	t.Parallel()
	c := hiddenCardsTable(t)
	b := cardsBuild(nil)
	keys := func(x *Content) []string {
		var out []string
		for _, o := range FeatOptions(b, x) {
			out = append(out, o.Key)
		}
		return out
	}
	if master := keys(c); !slices.Contains(master, cardOnlyHidden) || !slices.Contains(master, cardMixed) {
		t.Errorf("the master's feat options = %v", master)
	}
	if player := keys(c.ForPlayer()); slices.Contains(player, cardOnlyHidden) || !slices.Contains(player, cardMixed) {
		t.Errorf("the player's feat options = %v, want the mixed feat and not the one of the hidden race", player)
	}
}

// A player who holds a hidden feat, or a trait of a hidden race, reads its name where the condition is theirs; a condition
// they may not read is never dropped from the text.
func TestFeatCardsNameAHiddenFeatOrTraitTheSheetHas(t *testing.T) {
	t.Parallel()
	c := hiddenCardsTable(t).ForPlayer()
	holder, _ := cardText(t, c.Choices(cardsBuild(nil, cardHiddenFeat)))
	for _, want := range []string{"Não pode ter " + hiddenFeatName + ".", "Pede: não ter " + hiddenFeatName + "."} {
		if !strings.Contains(holder, want) {
			t.Errorf("the holder's card lacks %q:\n%s", want, holder)
		}
	}
	// The sheet of the hidden race reads the trait it has.
	own := cardsBuild(nil)
	own.Race = "race:anao-das-brumas" + tableSuffix
	if !c.c.sheetHas(own, cardHiddenTrait) || !strings.Contains(c.c.featAsksPT(FeatPrerequisite{LacksFeature: cardHiddenTrait}, own), hiddenTraitName) {
		t.Errorf("the sheet of the hidden race does not read its own trait: %q", c.c.featAsksPT(FeatPrerequisite{LacksFeature: cardHiddenTrait}, own))
	}
	// Whoever does not hold it reads a placeholder in the "Pede:" clause, not nothing.
	other := cardsBuild(nil)
	if got := c.c.featAsksPT(FeatPrerequisite{LacksFeature: cardHiddenTrait, Level: 4}, other); got != hiddenAskPT+" e nível 4 ou mais" {
		t.Errorf("the clause for a trait the sheet does not have = %q", got)
	}
}

// An option switched off itself stays unnamed to a fighter who never picked it; one the sheet picked is named.
func TestFeatCardsNameASwitchedOffOptionOnlyToWhoPickedIt(t *testing.T) {
	t.Parallel()
	const option, name = "option:fighting_style:guarda-dupla" + tableSuffix, "Guarda Dupla"
	visible, _, _ := tableMisc()
	visible.ChoiceBonuses = nil
	visible.Key, visible.NamePT = cardVisibleRace, "Gnomo do Vale"
	visible.Traits = []TableFeature{{
		Key: cardFeatGrantKey, NamePT: "Talento do vale", DescPT: []string{"Um talento à escolha."},
		Effects: []Effect{{Type: "choice", Choice: "feat", Count: 1}},
	}}
	c := withOverlay(t, Overlay{
		Revision: 1, Off: []string{option}, Races: []TableRace{visible},
		Options: []TableOption{{TableEntry: TableEntry{Key: option, NamePT: name}, Set: "fighting_style"}},
		Feats:   []TableFeat{{TableEntry: TableEntry{Key: cardOpenFeat, NamePT: "Sem guarda"}, DescPT: []string{"Texto."}, Prerequisite: FeatPrerequisite{LacksFeature: option}}},
	})
	if !c.Hidden(option) {
		t.Fatalf("Hidden(%s) = false when switched off", option)
	}
	fighter := cardsBuild(nil)
	if c.SheetHas(fighter, option) {
		t.Errorf("a fighter who never picked the option has it")
	}
	text, _ := cardText(t, c.ForPlayer().Choices(fighter))
	if strings.Contains(text, name) || !strings.Contains(text, "Pede: "+hiddenAskPT+".") {
		t.Errorf("the fighter's card:\n%s", text)
	}
	if master, _ := cardText(t, c.Choices(fighter)); !strings.Contains(master, "Pede: não ter "+name+".") {
		t.Errorf("the master's card does not name the option:\n%s", master)
	}
	picked := cardsBuild(nil)
	picked.FeatureChoices = append(picked.FeatureChoices, option)
	if !c.SheetHas(picked, option) {
		t.Fatal("the sheet that picked the option does not have it")
	}
	if text, _ := cardText(t, c.ForPlayer().Choices(picked)); !strings.Contains(text, "Pede: não ter "+name+".") {
		t.Errorf("the card of the sheet that picked the option:\n%s", text)
	}
}
