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
	// EntryPT is the name of the feat or the option.
	EntryPT string
	// Option says it is an option; otherwise it is a feat.
	Option bool
	// Conditions are the missing conditions, in the order the prerequisite lists them.
	Conditions []LostCondition
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
	if len(parts) < 2 {
		return strings.Join(parts, "")
	}
	last := len(parts) - 1
	return strings.Join(parts[:last], ", ") + " e " + parts[last]
}

// lost is the LostPrerequisite of the entry key for the missing conditions unmet, judged on the sheet d.
func (c *content) lost(key string, option bool, unmet []FeatUnmet, b Build, d Derived) *LostPrerequisite {
	l := &LostPrerequisite{EntryPT: c.namePT(key), Option: option}
	for _, u := range unmet {
		l.Conditions = append(l.Conditions, c.lostCondition(u, b, d))
	}
	return l
}

// lostCondition says one missing condition and what the character has of it.
func (c *content) lostCondition(u FeatUnmet, b Build, d Derived) LostCondition {
	var lc LostCondition
	switch u.Kind {
	case FeatUnmetClassLevel:
		class := c.namePT(u.Key)
		lc = LostCondition{AskPT: fmt.Sprintf("%s %d", class, u.Value), LinkPT: "e", HasPT: "não tem nível de " + class}
		if have := classLevelIn(b.Classes, u.Key); have > 0 {
			lc.HasPT = fmt.Sprintf("está no %d", have)
		}
	case FeatUnmetPact:
		lc = LostCondition{AskPT: "a Dádiva do Pacto " + optionLabel(c.namePT(u.Key)), LinkPT: "que", HasPT: "não tem mais"}
	case FeatUnmetSpell:
		lc = LostCondition{AskPT: "a magia " + c.namePT(u.Key), LinkPT: "que", HasPT: "não conhece mais"}
	case FeatUnmetOption:
		lc = LostCondition{AskPT: optionLabel(c.namePT(u.Key)), LinkPT: "que", HasPT: "não tem mais"}
	case FeatUnmetAbilityMinimum, FeatUnmetAbilityAnyOf:
		lc = c.lostAbilities(u, d)
	case FeatUnmetProficiency:
		lc = LostCondition{AskPT: "proficiência em " + c.namePT(u.Key), LinkPT: "que", HasPT: "não tem"}
	case FeatUnmetSpellcasting:
		lc = LostCondition{AskPT: "conjurar magias", LinkPT: "que", HasPT: "não consegue mais"}
	case FeatUnmetRace:
		asked := u.Keys
		if u.Key != "" {
			asked = []string{u.Key}
		}
		lc = LostCondition{AskPT: "ser " + c.racesPT(asked), LinkPT: "que", HasPT: "não é"}
		lc.Hidden = slices.ContainsFunc(asked, func(k string) bool { return c.hiddenOn(b, k) })
	case FeatUnmetSize:
		lc = LostCondition{AskPT: "tamanho " + strings.ToLower(sizeNamePT[u.Key]), LinkPT: "que", HasPT: "não tem"}
	case FeatUnmetLacksFeature:
		lc = LostCondition{AskPT: "não ter " + c.namePT(u.Key), LinkPT: "e", HasPT: "tem"}
	case FeatUnmetLevel:
		lc = LostCondition{AskPT: fmt.Sprintf("nível %d", u.Value), LinkPT: "e", HasPT: fmt.Sprintf("está no %d", d.TotalLevel)}
	}
	switch u.Kind {
	case FeatUnmetClassLevel, FeatUnmetPact, FeatUnmetSpell, FeatUnmetOption, FeatUnmetLacksFeature:
		lc.Hidden = c.hiddenOn(b, u.Key)
	}
	if lc.Hidden && c.player {
		lc.AskPT = "" // a player's view holds no name of it, whoever reads the sentence
	}
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
