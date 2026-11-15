package rules

import (
	"fmt"
	"slices"
	"strings"
)

// LostPrerequisite is what a feat or an option the sheet keeps asks and the character lacks now (board 580-05): the
// issue's sentence is built from it for each reader (SentencePT), as the character's name is not in the Build and a
// condition the players may not read is named for the master alone (RN-23).
type LostPrerequisite struct {
	// Key is the feat's or the option's key: with the issue's code and field, what the issue is told apart by
	// (Issue.ID), whatever its sentence says now.
	Key string
	// EntryPT is the name of the feat or the option.
	EntryPT string
	// Option says it is an option; otherwise it is a feat.
	Option bool
	// Conditions are the missing conditions, in the order the prerequisite lists them.
	Conditions []LostCondition
	// unmet is what the conditions were made from, to say them again over the sheet as it ends up.
	unmet []FeatUnmet
}

// LostCondition is one missing condition: what is asked ("Guerreiro 7") and what the character has, told after the
// character's name ("e Tor está no 5", "que Tor não conhece mais").
type LostCondition struct {
	AskPT  string
	LinkPT string
	HasPT  string
	// Hidden says the condition is on an entry the players may not read and the sheet does not have: a player reads
	// hiddenAskPT for it. In a player's view of the content AskPT is empty.
	Hidden bool
}

// lostNobody stands for the character in the stored message, which is the same for every reader.
const lostNobody = "o personagem"

// SentencePT is the sentence a reader gets: "Asa Partida pede Guerreiro 7, e Tor está no 5: a opção só vale de novo
// quando Tor cumprir.". name is the character's (empty reads "o personagem"); master says the reader is the
// campaign's master, who reads a hidden condition named.
func (l LostPrerequisite) SentencePT(name string, master bool) string {
	if name == "" {
		name = lostNobody
	}
	var parts []string
	for _, c := range l.Conditions {
		part := hiddenAskPT
		if !c.Hidden || master && c.AskPT != "" {
			part = c.AskPT + ", " + c.LinkPT + " " + name + " " + c.HasPT
		}
		if !slices.Contains(parts, part) {
			parts = append(parts, part)
		}
	}
	what := "o talento"
	if l.Option {
		what = "a opção"
	}
	asked := listPT(parts)
	if asked == "" { // no condition to name: the prerequisite as a whole
		asked = "um pré-requisito que " + name + " não cumpre"
	}
	return fmt.Sprintf("%s pede %s: %s só vale de novo quando %s cumprir.", l.EntryPT, asked, what, name)
}

// listPT joins phrases as a list: "A", "A e B", "A, B e C".
func listPT(parts []string) string {
	last := len(parts) - 1
	if last < 1 {
		return strings.Join(parts, "")
	}
	return strings.Join(parts[:last], ", ") + " e " + parts[last]
}

// lost is the LostPrerequisite of the entry key for the missing conditions unmet, judged on the sheet d.
func (c *content) lost(key string, option bool, unmet []FeatUnmet, b Build, d Derived) *LostPrerequisite {
	l := &LostPrerequisite{Key: key, EntryPT: c.namePT(key), Option: option, unmet: unmet}
	for _, u := range unmet {
		l.Conditions = append(l.Conditions, c.lostCondition(u, b, d))
	}
	return l
}

// lostIDPrefix marks, in an issue's identity, the key of a lost feat or option where another issue has its sentence.
const lostIDPrefix = "lost:"

// ID is what an issue is told apart by, among the issues of a sheet and over time: its code, its field and its
// sentence; for a lost prerequisite the entry's key in the sentence's place, because the sentence says what the
// character has now and what the reader may see, and both change while the issue is the same one. The sheet stores
// these (known issues), so a stored lost prerequisite holds no sentence.
func (is Issue) ID() string {
	if is.Lost != nil {
		return is.Code + "|" + is.Field + "|" + lostIDPrefix + is.Lost.Key
	}
	return is.Code + "|" + is.Field + "|" + is.Message
}

// LegacyLostID says a stored identity is this lost prerequisite as the code before Issue.ID wrote it: the code, the
// field (without the pick's index for a feat taken through a choice) and the old sentence, which began the same for
// every reader and named the entry.
func (is Issue) LegacyLostID(stored string) bool {
	if is.Lost == nil {
		return false
	}
	sentence := "|O personagem não cumpre mais o pré-requisito de " + is.Lost.EntryPT + ":"
	field, _, _ := strings.Cut(is.Field, "[")
	return strings.HasPrefix(stored, is.Code+"|"+is.Field+sentence) || field == choiceKeysField && strings.HasPrefix(stored, is.Code+"|"+field+sentence)
}

// retold is the lost prerequisite said over the sheet d: the same conditions, with what the character has there.
func (c *content) retold(l *LostPrerequisite, b Build, d Derived) *LostPrerequisite {
	return c.lost(l.Key, l.Option, l.unmet, b, d)
}

// lostCondition says one missing condition and what the character has of it.
func (c *content) lostCondition(u FeatUnmet, b Build, d Derived) LostCondition {
	lc := c.lostAsk(u, b, d)
	switch u.Kind {
	case FeatUnmetClassLevel, FeatUnmetPact, FeatUnmetSpell, FeatUnmetOption, FeatUnmetLacksFeature:
		lc.Hidden = c.hiddenOn(b, u.Key)
	}
	if lc.Hidden && c.player {
		lc.AskPT = "" // a player's view holds no name of it, whoever reads the sentence
	}
	return lc
}

// lostAsk is the condition's wording by its kind; a race's says by itself whether it is hidden.
func (c *content) lostAsk(u FeatUnmet, b Build, d Derived) LostCondition {
	switch u.Kind {
	case FeatUnmetClassLevel:
		return c.lostClassLevel(u, b)
	case FeatUnmetAbilityMinimum, FeatUnmetAbilityAnyOf:
		return c.lostAbilities(u, d)
	case FeatUnmetRace:
		return c.lostRace(u, b)
	case FeatUnmetLevel:
		return LostCondition{AskPT: fmt.Sprintf("nível %d", u.Value), LinkPT: "e", HasPT: fmt.Sprintf("está no %d", d.TotalLevel)}
	case FeatUnmetLacksFeature:
		return LostCondition{AskPT: "não ter " + c.namePT(u.Key), LinkPT: "e", HasPT: "tem"}
	}
	return LostCondition{AskPT: c.lostAskPT(u), LinkPT: "que", HasPT: lostHasPT[u.Kind]}
}

// lostHasPT is what the character has of a condition that is plainly missing, told after "que <name>".
var lostHasPT = map[string]string{
	FeatUnmetPact:         "não tem mais",
	FeatUnmetSpell:        "não conhece mais",
	FeatUnmetOption:       "não tem mais",
	FeatUnmetProficiency:  "não tem",
	FeatUnmetSpellcasting: "não consegue mais",
	FeatUnmetSize:         "não tem",
}

// lostAskPT names what a plainly missing condition asks.
func (c *content) lostAskPT(u FeatUnmet) string {
	switch u.Kind {
	case FeatUnmetPact:
		return "a Dádiva do Pacto " + optionLabel(c.namePT(u.Key))
	case FeatUnmetSpell:
		return "a magia " + c.namePT(u.Key)
	case FeatUnmetOption:
		return optionLabel(c.namePT(u.Key))
	case FeatUnmetProficiency:
		return "proficiência em " + c.namePT(u.Key)
	case FeatUnmetSpellcasting:
		return "conjurar magias"
	case FeatUnmetSize:
		return "tamanho " + strings.ToLower(sizeNamePT[u.Key])
	}
	return ""
}

// lostClassLevel says the class level asked and the one the character is at: "Guerreiro 7, e Tor está no 5".
func (c *content) lostClassLevel(u FeatUnmet, b Build) LostCondition {
	class := c.namePT(u.Key)
	lc := LostCondition{AskPT: fmt.Sprintf("%s %d", class, u.Value), LinkPT: "e", HasPT: "não tem nível de " + class}
	if have := classLevelIn(b.Classes, u.Key); have > 0 {
		lc.HasPT = fmt.Sprintf("está no %d", have)
	}
	return lc
}

// lostRace says the races asked: "ser Anão ou Elfo, que Tor não é".
func (c *content) lostRace(u FeatUnmet, b Build) LostCondition {
	asked := u.Keys
	if u.Key != "" {
		asked = []string{u.Key}
	}
	lc := LostCondition{AskPT: "ser " + c.racesPT(asked), LinkPT: "que", HasPT: "não é"}
	lc.Hidden = slices.ContainsFunc(asked, func(k string) bool { return c.hiddenOn(b, k) })
	return lc
}

// lostAbilities says the scores asked and the ones the character has: "Força 13, e Tor tem 11".
func (c *content) lostAbilities(u FeatUnmet, d Derived) LostCondition {
	score := map[Ability]int{}
	for _, s := range d.Abilities {
		score[s.Ability] = s.Score
	}
	var asked, has []string
	for _, am := range u.Abilities {
		name := c.namePT(string(am.Ability))
		asked = append(asked, fmt.Sprintf("%s %d", name, am.Minimum))
		has = append(has, fmt.Sprintf("%s %d", name, score[am.Ability]))
	}
	join := " e "
	if u.Kind == FeatUnmetAbilityAnyOf {
		join = " ou "
	}
	lc := LostCondition{AskPT: strings.Join(asked, join), LinkPT: "e", HasPT: "tem " + strings.Join(has, " e ")}
	if len(u.Abilities) == 1 {
		lc.HasPT = fmt.Sprintf("tem %d", score[u.Abilities[0].Ability])
	}
	return lc
}

// hiddenOn says key is an entry the players may not read and the sheet b does not have, whoever the view is for.
func (c *content) hiddenOn(b Build, key string) bool {
	return c.hidden(key) && !c.sheetHas(b, key)
}
