package rules

import (
	"fmt"
	"slices"
	"strings"
)

// A feat of the player's choice (MR-025). A race, subrace, background, class or
// subclass feature, a trait or a feat of the table may carry a `choice` effect
// with `choice: "feat"`: the character picks `count` feats, from the effect's
// `from` list or, without one, from every feat of the content. The engine offers
// it like any other choice (choicegroups.go) and the pick is stored the same way
// as the others: "<owner key>#feat=<feat key>" in Build.FeatureChoices, where
// the owner is the thing that grants it. The feat applies exactly like one in
// Build.Feats (its effects, its prerequisite, its place among the features) for
// as long as the owner is on the sheet, and it is never on the sheet twice.
//
// The table rule "Talentos" (feats_allowed) does not gate this: the table wrote
// the grant itself. A feat's ability increase is not applied through a choice:
// the player notes it in the extra ability bonuses.

// featChoiceSuffix ends the key of the choice a feat grant asks.
const featChoiceSuffix = textSeparator + "feat"

// FeatChoiceKey is the key of the choice of a feat that owner grants.
func FeatChoiceKey(owner string) string { return owner + featChoiceSuffix }

// choiceFeat is a feat picked through a choice, and the owner that granted it.
type choiceFeat struct{ Feat, Owner string }

// featChoiceSpec is what the feat choices of an owner ask in all: how many feats
// and the list they come from (empty: any feat).
func (c *content) featChoiceSpec(owner string) (picks int, from []string, ok bool) {
	open := false
	for _, e := range c.effectChoices(owner, "feat") {
		ok = true
		picks += max(e.Count, 1)
		if len(e.From) == 0 {
			open = true
			continue
		}
		for _, k := range e.From {
			if !slices.Contains(from, k) {
				from = append(from, k)
			}
		}
	}
	if open {
		from = nil
	}
	return picks, from, ok
}

// pickedChoiceFeats are the feats the picks of Build.FeatureChoices give: the ones
// whose owner is on the sheet (owned), that the owner's list offers, up to the
// number it grants, none the sheet already has. In the order of the picks.
func (c *content) pickedChoiceFeats(b Build, owned func(string) bool) []choiceFeat {
	var out []choiceFeat
	var count map[string]int
	for _, k := range b.FeatureChoices {
		choiceKey, value, ok := SplitScopedChoice(k)
		if !ok || !strings.HasPrefix(value, "feat:") {
			continue
		}
		owner, isFeat := strings.CutSuffix(choiceKey, featChoiceSuffix)
		if !isFeat || !owned(owner) {
			continue
		}
		picks, from, granted := c.featChoiceSpec(owner)
		if !granted || count[owner] >= picks || c.feats[value] == nil {
			continue
		}
		if len(from) > 0 && !slices.Contains(from, value) {
			continue
		}
		if slices.Contains(b.Feats, value) || slices.ContainsFunc(out, func(cf choiceFeat) bool { return cf.Feat == value }) {
			continue
		}
		if count == nil {
			count = map[string]int{}
		}
		count[owner]++
		out = append(out, choiceFeat{Feat: value, Owner: owner})
	}
	return out
}

// ownedBy says whether a key is on a derived sheet: a feature, trait or feat it
// lists, or the race, subrace, background, class or subclass.
func ownedBy(b Build, d Derived) func(string) bool {
	return func(key string) bool {
		if key == "" {
			return false
		}
		if key == b.Race || key == b.Subrace || key == b.Background {
			return true
		}
		for _, cl := range b.Classes {
			if key == cl.Class || key == cl.Subclass {
				return true
			}
		}
		return slices.ContainsFunc(d.Features, func(f Feature) bool { return f.Key == key })
	}
}

// choiceFeats adds to the sheet the feats its picks give, after the feats of
// Build.Feats. A feat whose prerequisite is no longer met is listed and not applied.
func (x *deriver) choiceFeats(owned map[string]bool, add func(string), feature func(key, name, source string, level int, desc []string)) {
	c := x.c
	for _, cf := range c.pickedChoiceFeats(x.b, func(k string) bool { return owned[k] }) {
		f := c.feats[cf.Feat]
		if x.inactiveFeats[cf.Feat] {
			x.issue(IssueFeatPrerequisite, choiceKeysField, "O personagem não cumpre mais o pré-requisito de %s: o talento só vale de novo quando cumprir.", c.namePT(cf.Feat))
		} else {
			add(cf.Feat)
		}
		feature(cf.Feat, f.Name, cf.Feat, 0, f.Desc)
		source := c.namePT(cf.Owner)
		for _, of := range x.d.Features {
			if of.Key == cf.Owner && of.SourcePT != "" {
				source = of.SourcePT
				break
			}
		}
		x.d.Features[len(x.d.Features)-1].SourcePT = "Talento · " + source
	}
}

// background adds the choices of the background: a feat it grants.
func (cb *choiceBuilder) background() {
	bg, ok := cb.c.backgrounds[cb.b.Background]
	if !ok {
		return
	}
	g := cb.group(ChoiceOriginBackground, bg.Key, 0, 0)
	cb.featChoice(g, bg.Key)
	cb.featChoice(g, bg.Feature.Key)
}

// featOwners adds the choices the feats of the sheet ask (a feat that grants another).
func (cb *choiceBuilder) featOwners() {
	for _, key := range cb.b.Feats {
		if cb.c.feats[key] == nil {
			continue
		}
		cb.featChoice(cb.group(ChoiceOriginFeat, key, 0, 0), key)
	}
}

// featChoice adds the choice of a feat that owner grants, if any: the options are
// the feats of the list (or of the content), each with whether the character
// qualifies. A feat the sheet already has is not an option; one the table retired
// or switched off stays only for whoever already picked it.
func (cb *choiceBuilder) featChoice(g *groupBuild, owner string) {
	c := cb.c
	picks, from, ok := c.featChoiceSpec(owner)
	if !ok || owner == "" {
		return
	}
	key := FeatChoiceKey(owner)
	ch := cb.addChoice(g, &Choice{
		Key: key, FeatureKey: owner, Kind: ChoiceKindOptions, TitlePT: "Talento", Picks: picks, family: "feat",
		HintPT: "Um talento à escolha. Um talento pede o que o texto dele diz; os que o personagem ainda não cumpre aparecem indisponíveis.",
	})
	if picks > 1 {
		ch.TitlePT = "Talentos"
		ch.HintPT = fmt.Sprintf("%d talentos à escolha. Um talento pede o que o texto dele diz; os que o personagem ainda não cumpre aparecem indisponíveis.", picks)
	}
	d := cb.derive()
	pickedNow := cb.scoped[key]
	for _, e := range (&Content{c: c}).Feats() {
		if len(from) > 0 && !slices.Contains(from, e.Key) {
			continue
		}
		if slices.Contains(cb.b.Feats, e.Key) {
			continue
		}
		if (e.Archived || e.Off) && !slices.Contains(pickedNow, e.Key) {
			continue
		}
		o := ChoiceOption{Key: e.Key, Stored: ScopedChoice(key, e.Key), NamePT: e.NamePT, SummaryPT: featSummaryPT(c, e)}
		if unmet := c.unmetPrerequisite(e, cb.b, *d); len(unmet) > 0 {
			var reasons []string
			for _, u := range unmet {
				reasons = append(reasons, c.featUnmetPT(u))
			}
			o.ReasonPT = "Você ainda não cumpre o pré-requisito: " + strings.Join(reasons, " ")
			o.Prerequisites = append(o.Prerequisites, OptionPrerequisite{Kind: PrerequisiteFeature, Met: false, ReasonPT: o.ReasonPT})
		}
		ch.Options = append(ch.Options, o)
	}
	cb.takeScoped(ch)
	for _, k := range ch.Picked {
		for _, o := range ch.Options {
			if o.Key == k && o.Blocked() {
				ch.Unmet = append(ch.Unmet, k)
			}
		}
	}
}

// featSummaryPT is the feat's text as a choice card shows it, with what it asks first.
func featSummaryPT(c *content, e FeatEntry) string {
	text := e.DescPT
	if len(text) == 0 {
		text = e.Desc
	}
	var parts []string
	if asks := c.featAsksPT(e.Prerequisite); asks != "" {
		parts = append(parts, "Pede: "+asks+".")
	}
	parts = append(parts, text...)
	return strings.Join(parts, " ")
}

// featAsksPT is what a feat asks of a character, in a phrase ("Força 13 e nível 4 ou mais").
func (c *content) featAsksPT(p FeatPrerequisite) string {
	var out []string
	for _, a := range AllAbilities() {
		if least := p.Minimums[a]; least > 0 {
			out = append(out, fmt.Sprintf("%s %d", c.namePT(string(a)), least))
		}
	}
	var anyOf []string
	for _, a := range AllAbilities() {
		if least := p.AnyOf[a]; least > 0 {
			anyOf = append(anyOf, fmt.Sprintf("%s %d", c.namePT(string(a)), least))
		}
	}
	if len(anyOf) > 0 {
		out = append(out, strings.Join(anyOf, " ou "))
	}
	if p.Proficiency != "" {
		out = append(out, "proficiência em "+c.namePT(p.Proficiency))
	}
	if p.Spellcasting {
		out = append(out, "conseguir conjurar magias")
	}
	if p.Race != "" {
		out = append(out, "ser "+c.namePT(p.Race))
	}
	if p.Level > 0 {
		out = append(out, fmt.Sprintf("nível %d ou mais", p.Level))
	}
	return strings.Join(out, " e ")
}

// featUnmetPT says in a sentence what a character lacks for one condition of a feat.
func (c *content) featUnmetPT(u FeatUnmet) string {
	switch u.Kind {
	case FeatUnmetAbilityMinimum, FeatUnmetAbilityAnyOf:
		var parts []string
		for _, am := range u.Abilities {
			parts = append(parts, fmt.Sprintf("%s %d", c.namePT(string(am.Ability)), am.Minimum))
		}
		if u.Kind == FeatUnmetAbilityAnyOf {
			return "Precisa de " + strings.Join(parts, " ou ") + "."
		}
		return "Precisa de " + strings.Join(parts, " e ") + "."
	case FeatUnmetProficiency:
		return "Precisa de proficiência em " + c.namePT(u.Key) + "."
	case FeatUnmetSpellcasting:
		return "Precisa conseguir conjurar magias."
	case FeatUnmetRace:
		return "Só para " + c.namePT(u.Key) + "."
	case FeatUnmetLevel:
		return fmt.Sprintf("Precisa de nível %d ou mais.", u.Value)
	}
	return ""
}
