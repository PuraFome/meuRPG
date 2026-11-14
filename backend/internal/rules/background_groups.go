package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// A table background's "this or that" groups (#580 S4). A group lets the player pick `Choose` of its options: skills,
// tool proficiencies (the SRD's dice set and playing card set among them, and the named gaming sets the master
// typed), or pieces of equipment. SRD 5.1, "Backgrounds": a background gives two skills, tools, languages and
// equipment; a group is how a table's background leaves some of that to the player.
//
// A pick is written "<background key>#group:<signature>=<value>" in Build.FeatureChoices, where the signature is
// worked out from the group's kind, its number of picks and its options (not their order). So a pick follows its
// group when the master reorders the groups, and stops counting when the master edits the group: the group then asks
// for its picks again (a locked sheet gets a pending choice).

// The kinds of a group.
const (
	GroupSkill     = "skill"
	GroupTool      = "tool"
	GroupEquipment = "equipment"
)

const (
	// MaxBackgroundGroups bounds the groups of a background.
	MaxBackgroundGroups = 4
	// MaxGroupOptions bounds the options of a group, the gaming sets included.
	MaxGroupOptions = 12
	// MaxGamingSetNameLength and MaxGroupItemLength bound the typed name of a gaming set and the text of a piece of
	// equipment, in characters. The table's short texts (a choice's free text) take 40; a piece of equipment takes more.
	MaxGamingSetNameLength = 40
	// MaxGroupItemLength is the most characters of a piece of equipment of a group.
	MaxGroupItemLength = 80

	groupKeyPart = textSeparator + "group:"
	// gamingSetPrefix and itemPrefix start the stored value of a typed gaming set and of a piece of equipment.
	gamingSetPrefix = "set:"
	itemPrefix      = "item:"
	// GamingSetLabel is how the sheet lists a typed gaming set: "Conjunto de jogo: Trilha".
	GamingSetLabel = "Conjunto de jogo: "
)

// BackgroundGroup is one "this or that" of a table background.
type BackgroundGroup struct {
	// Kind is GroupSkill, GroupTool or GroupEquipment.
	Kind   string
	Choose int
	// Options are skill keys, tool proficiency keys, or the texts of the equipment.
	Options []string
	// SetNames are the named gaming sets a tool group adds to its options.
	SetNames []string
}

// values are the stored values of the group's options, in the order the master wrote them.
func (g BackgroundGroup) values() []string {
	out := slices.Clone(g.Options)
	switch g.Kind {
	case GroupEquipment:
		for i, o := range out {
			out[i] = itemPrefix + o
		}
	case GroupTool:
		for _, n := range g.SetNames {
			out = append(out, gamingSetPrefix+n)
		}
	}
	return out
}

// Signature is what ties a stored pick to the group: the same for the same kind, picks and options in any order.
func (g BackgroundGroup) Signature() string {
	vals := g.values()
	slices.Sort(vals)
	// Every value is written with its length first, so no typed text can pass for a separator.
	var in strings.Builder
	fmt.Fprintf(&in, "%d:%s|%d", len(g.Kind), g.Kind, g.Choose)
	for _, v := range vals {
		fmt.Fprintf(&in, "|%d:%s", len(v), v)
	}
	sum := sha256.Sum256([]byte(in.String()))
	return hex.EncodeToString(sum[:4])
}

// BackgroundGroupKey is the key of the choice a group asks, and of the picks that answer it.
func BackgroundGroupKey(background string, g BackgroundGroup) string {
	return background + groupKeyPart + g.Signature()
}

// groupSkillPicks is how many skills the skill groups let the player pick.
func groupSkillPicks(groups []BackgroundGroup) int {
	n := 0
	for _, g := range groups {
		if g.Kind == GroupSkill {
			n += g.Choose
		}
	}
	return n
}

// groupValueOK says a stored group pick is one the sheet may carry: for a background the content has, a value of the
// group it names, or any value when the master has edited that group since (its signature matches no group now: the
// pick counts for nothing, and the group asks again). A value that is no option of the group it names is refused.
func (c *content) groupValueOK(choiceKey, value string) bool {
	owner, sig, _ := strings.Cut(choiceKey, groupKeyPart)
	if c.backgrounds[owner] == nil {
		return false
	}
	for _, g := range c.bgGroups[owner] {
		if g.Signature() == sig {
			return slices.Contains(g.values(), value)
		}
	}
	// A stale pick is kept, but no longer than the longest value a group can hold.
	return utf8.RuneCountInString(value) <= len(itemPrefix)+MaxGroupItemLength
}

// groupPick is a group of the sheet's background and what the sheet picked in it.
type groupPick struct {
	Group  BackgroundGroup
	Key    string
	Values []string
}

// backgroundGroupPicks reads the groups of the sheet's background and, for each, the picks that match it (its key,
// one of its options, none twice, no more than it takes), in the order of the picks.
func (c *content) backgroundGroupPicks(b Build) []groupPick {
	groups := c.bgGroups[b.Background]
	out := make([]groupPick, len(groups))
	byKey := map[string]int{}
	for i, g := range groups {
		out[i] = groupPick{Group: g, Key: BackgroundGroupKey(b.Background, g)}
		byKey[out[i].Key] = i
	}
	for _, stored := range b.FeatureChoices {
		key, value, ok := SplitScopedChoice(stored)
		i, known := byKey[key]
		if !ok || !known {
			continue
		}
		p := &out[i]
		if len(p.Values) < p.Group.Choose && slices.Contains(p.Group.values(), value) && !slices.Contains(p.Values, value) {
			p.Values = append(p.Values, value)
		}
	}
	return out
}

// groupSkills are the skills the sheet's skill groups gave.
func (x *deriver) groupSkills() []string {
	var out []string
	for _, p := range x.groupPicks() {
		if p.Group.Kind != GroupSkill {
			continue
		}
		for _, v := range p.Values {
			if _, ok := x.c.skills[v]; ok {
				out = append(out, v)
			}
		}
	}
	return out
}

// groupPicks is backgroundGroupPicks of the sheet, once.
func (x *deriver) groupPicks() []groupPick {
	if !x.groupsRead {
		x.groupsRead = true
		x.groups = x.c.backgroundGroupPicks(x.b)
	}
	return x.groups
}

// groupTools gives the tool proficiencies of the tool groups: an SRD tool joins the proficiencies, a typed gaming set
// is listed by its label.
func (x *deriver) groupTools() (typed []string) {
	for _, p := range x.groupPicks() {
		if p.Group.Kind != GroupTool {
			continue
		}
		for _, v := range p.Values {
			if name, ok := strings.CutPrefix(v, gamingSetPrefix); ok {
				typed = append(typed, GamingSetLabel+name)
			} else if x.c.proficiencies[v] != nil {
				x.proficient[v] = true
			}
		}
	}
	return typed
}

// groupEquipment is the equipment text of the background with what its equipment groups gave added, a line each.
func (x *deriver) groupEquipment(base string) string {
	lines := []string{}
	if base != "" {
		lines = append(lines, base)
	}
	for _, p := range x.groupPicks() {
		if p.Group.Kind != GroupEquipment {
			continue
		}
		for _, v := range p.Values {
			lines = append(lines, strings.TrimPrefix(v, itemPrefix))
		}
	}
	return strings.Join(lines, "\n")
}

// checkBackgroundGroupPicks warns of the group picks the sheet has not made.
func (x *deriver) checkBackgroundGroupPicks() {
	missing := 0
	for _, p := range x.groupPicks() {
		missing += p.Group.Choose - len(p.Values)
	}
	if missing > 0 {
		x.issue(IssueMissing, choiceKeysField, "Falta escolher %d no antecedente: faça a escolha no passo “Escolhas”.", missing)
	}
}

// checkBackgroundGroups checks the groups of a table background, each at its own path.
func (b *overlayBuilder) checkBackgroundGroups(c *entryErrors, tb *TableBackground, path string) {
	if len(tb.Groups) > MaxBackgroundGroups {
		c.at(tb.Key, path, ".groups", ReasonLimit, "at most %d groups", MaxBackgroundGroups)
		return
	}
	seen := map[string]bool{}
	for i, g := range tb.Groups {
		at := fmt.Sprintf(".groups[%d]", i)
		b.checkGroup(c, tb, g, path+at)
		sig := g.Signature()
		if seen[sig] {
			c.at(tb.Key, path, at, ReasonValue, "two groups are the same")
		}
		seen[sig] = true
	}
}

func (b *overlayBuilder) checkGroup(c *entryErrors, tb *TableBackground, g BackgroundGroup, path string) {
	key := tb.Key
	vals := g.values()
	switch {
	case g.Kind != GroupSkill && g.Kind != GroupTool && g.Kind != GroupEquipment:
		c.at(key, path, ".kind", ReasonValue, "a group offers skills, tools or equipment")
		return
	case len(vals) < 2 || len(vals) > MaxGroupOptions:
		c.at(key, path, ".options", ReasonLimit, "a group has 2 to %d options", MaxGroupOptions)
	case g.Choose < 1 || g.Choose >= len(vals):
		c.at(key, path, ".choose", ReasonValue, "a group picks at least 1, and fewer than its %d options", len(vals))
	}
	if g.Kind != GroupTool && len(g.SetNames) > 0 {
		c.at(key, path, ".gaming_set_names", ReasonValue, "only a tool group has gaming sets")
	}
	for i, o := range g.Options {
		b.checkGroupOption(c, tb, g, o, g.Options[:i], fmt.Sprintf("%s.options[%d]", path, i))
	}
	checkGamingSetNames(c, key, g.SetNames, path)
}

// checkGamingSetNames checks the named gaming sets of a group, each at its own path.
func checkGamingSetNames(c *entryErrors, key string, names []string, path string) {
	for i, n := range names {
		at := fmt.Sprintf("%s.gaming_set_names[%d]", path, i)
		switch l := utf8.RuneCountInString(strings.TrimSpace(n)); {
		case l < 1 || l > MaxGamingSetNameLength:
			c.at(key, at, "", ReasonLimit, "a gaming set name has 1 to %d characters", MaxGamingSetNameLength)
		case plainTextProblem(c, key, at, n):
		case slices.ContainsFunc(names[:i], func(o string) bool { return foldPT(o) == foldPT(n) }):
			c.at(key, at, "", ReasonValue, "the gaming set %q is twice in the group", n)
		}
	}
}

// plainTextProblem refuses a typed text with a hidden character or spaces around it, and says whether it did.
func plainTextProblem(c *entryErrors, key, at, text string) bool {
	switch {
	case strings.IndexFunc(text, isHiddenRune) >= 0:
		c.at(key, at, "", ReasonText, "the text has hidden characters")
	case strings.TrimSpace(text) != text:
		c.at(key, at, "", ReasonText, "the text must not start or end with spaces")
	default:
		return false
	}
	return true
}

// inOtherGroup says an option (a skill or a tool) is already in another group of the background.
func inOtherGroup(tb *TableBackground, self BackgroundGroup, o string) bool {
	return slices.ContainsFunc(tb.Groups, func(g BackgroundGroup) bool {
		return g.Kind == self.Kind && g.Signature() != self.Signature() && slices.Contains(g.Options, o)
	})
}

func (b *overlayBuilder) checkGroupOption(c *entryErrors, tb *TableBackground, g BackgroundGroup, o string, before []string, at string) {
	if slices.Contains(before, o) {
		c.at(tb.Key, at, "", ReasonValue, "%q is twice in the group", o)
		return
	}
	if g.Kind != GroupEquipment && inOtherGroup(tb, g, o) {
		c.at(tb.Key, at, "", ReasonValue, "%q is in another group of the background", o)
		return
	}
	switch g.Kind {
	case GroupSkill:
		if _, ok := b.base.skills[o]; !ok {
			c.at(tb.Key, at, "", ReasonReference, "skill %q is not in the SRD", o)
		} else if slices.Contains(tb.Skills, o) {
			c.at(tb.Key, at, "", ReasonValue, "the background gives %q already", o)
		}
	case GroupTool:
		if p, ok := b.base.proficiencies[o]; !ok || p.Kind != "tool" {
			c.at(tb.Key, at, "", ReasonReference, "%q is not a tool proficiency of the SRD", o)
		} else if slices.Contains(tb.Tools, o) {
			c.at(tb.Key, at, "", ReasonValue, "the background gives %q already", o)
		}
	case GroupEquipment:
		if l := utf8.RuneCountInString(strings.TrimSpace(o)); l < 1 || l > MaxGroupItemLength {
			c.at(tb.Key, at, "", ReasonLimit, "a piece of equipment has 1 to %d characters", MaxGroupItemLength)
		} else {
			plainTextProblem(c, tb.Key, at, o)
		}
	}
}

// backgroundGroups adds a choice for each group of the sheet's background: the options are the group's, in the order
// the master wrote them, and the player takes `Choose` of them. It only warns while open, like the tool picks.
func (cb *choiceBuilder) backgroundGroups(g *groupBuild, background string) {
	had := cb.toolsHad()
	skillsHad := cb.skillsHad()
	for _, p := range cb.c.backgroundGroupPicks(cb.b) {
		title, hint, family := "Equipamento", "Peças de equipamento à escolha.", ""
		switch p.Group.Kind {
		case GroupSkill:
			title, hint, family = "Perícias", "Perícias do antecedente à escolha.", "background-skill"
		case GroupTool:
			title, hint, family = "Ferramentas", "Ferramentas do antecedente à escolha.", familyToolPick
		}
		if p.Group.Choose == 1 {
			title = map[string]string{GroupSkill: "Perícia", GroupTool: "Ferramenta", GroupEquipment: "Equipamento"}[p.Group.Kind]
		}
		ch := cb.addChoice(g, &Choice{Key: p.Key, FeatureKey: background, Kind: ChoiceKindOptions, TitlePT: title, HintPT: hint, Picks: p.Group.Choose, family: family, Advisory: true})
		for _, v := range p.Group.values() {
			o := ChoiceOption{Key: v, Stored: ScopedChoice(p.Key, v), NamePT: cb.groupOptionName(v)}
			if (p.Group.Kind == GroupTool && had[v]) || (p.Group.Kind == GroupSkill && skillsHad[v]) {
				blockOption(&o, "Você já tem esta proficiência.")
			}
			ch.Options = append(ch.Options, o)
		}
		cb.takeScoped(ch)
		unmeetBlockedPicks(ch)
	}
}

// groupOptionName is how a group's option reads in the step.
func (cb *choiceBuilder) groupOptionName(value string) string {
	switch {
	case strings.HasPrefix(value, gamingSetPrefix):
		return GamingSetLabel + strings.TrimPrefix(value, gamingSetPrefix)
	case strings.HasPrefix(value, itemPrefix):
		return strings.TrimPrefix(value, itemPrefix)
	case strings.HasPrefix(value, "skill:"):
		return cb.c.namePT(value)
	}
	return cb.c.proficiencyNamePT(value)
}

// skillsHad are the skills the character has from another source than the background's groups: the race, the
// class, a feat, an effect, the background's own skills and the ones the sheet lists. It works the sheet out again
// without the picks of the groups, and only when a skill group needs it.
func (cb *choiceBuilder) skillsHad() map[string]bool {
	out := map[string]bool{}
	if !slices.ContainsFunc(cb.c.bgGroups[cb.b.Background], func(g BackgroundGroup) bool { return g.Kind == GroupSkill }) {
		return out
	}
	b := cb.b
	b.FeatureChoices = slices.DeleteFunc(slices.Clone(b.FeatureChoices), func(k string) bool { return strings.Contains(k, groupKeyPart) })
	for _, sk := range derive(b, cb.c).Skills {
		out[sk.Key] = sk.Proficiency >= ProficiencyFull
	}
	return out
}
