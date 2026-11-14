package rules

import (
	"slices"
	"strings"
	"testing"
)

// #580 S4: the "this or that" groups of a table background.

func groupBackground(groups ...BackgroundGroup) TableBackground {
	return TableBackground{
		TableEntry: TableEntry{Key: "background:marinheiro" + tableSuffix, NamePT: "Marinheiro de rio"},
		Skills:     []string{"skill:athletics"},
		Groups:     groups,
		Feature:    TableFeature{Key: "background-feature:rio-conhecido" + tableSuffix, NamePT: "Rio conhecido", DescPT: []string{"Texto de teste."}},
	}
}

var (
	skillGroup = BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:history", "skill:insight", "skill:survival"}}
	toolGroup  = BackgroundGroup{Kind: GroupTool, Choose: 2, Options: []string{"proficiency:dice-set", "proficiency:playing-card-set"}, SetNames: []string{"Trilha"}}
	itemGroup  = BackgroundGroup{Kind: GroupEquipment, Choose: 1, Options: []string{"Corda de seda", "Lanterna furta-fogo"}}
)

func groupsBuild(bg TableBackground, picks ...string) Build {
	b := pickBuild("race:human", "")
	b.Background = bg.Key
	b.FeatureChoices = picks
	return b
}

func TestABackgroundGroupGivesItsPicksToTheSheet(t *testing.T) {
	t.Parallel()
	bg := groupBackground(skillGroup, toolGroup, itemGroup)
	c := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Backgrounds: []TableBackground{bg}})
	open := Derive(groupsBuild(bg), c)
	if !slices.ContainsFunc(open.Issues, func(is Issue) bool {
		return is.Code == IssueMissing && strings.Contains(is.Message, "Falta escolher 4 no antecedente")
	}) {
		t.Errorf("issues = %v, want 4 picks missing", open.Issues)
	}
	ch := choiceOf(t, c.Choices(groupsBuild(bg)), BackgroundGroupKey(bg.Key, toolGroup))
	if ch.Picks != 2 || len(ch.Options) != 3 || ch.Options[2].NamePT != "Conjunto de jogo: Trilha" {
		t.Fatalf("tool choice = %d picks, %v; want 2 of 3, the named set last", ch.Picks, ch.Options)
	}
	b := groupsBuild(bg,
		ScopedChoice(BackgroundGroupKey(bg.Key, skillGroup), "skill:insight"),
		ScopedChoice(BackgroundGroupKey(bg.Key, toolGroup), "proficiency:dice-set"),
		ScopedChoice(BackgroundGroupKey(bg.Key, toolGroup), "set:Trilha"),
		ScopedChoice(BackgroundGroupKey(bg.Key, itemGroup), "item:Corda de seda"),
	)
	d := Derive(b, c)
	if slices.ContainsFunc(d.Issues, func(is Issue) bool { return is.Code == IssueMissing && strings.Contains(is.Message, "antecedente") }) {
		t.Errorf("issues = %v, want no pick missing", d.Issues)
	}
	if lvl := skillLevel(d, "skill:insight"); lvl != ProficiencyFull || skillLevel(d, "skill:athletics") != ProficiencyFull || skillLevel(d, "skill:history") != ProficiencyNone {
		t.Errorf("insight %v, athletics %v, history %v; want the picked and the fixed one only", lvl, skillLevel(d, "skill:athletics"), skillLevel(d, "skill:history"))
	}
	names := map[string]bool{}
	for _, p := range d.Proficiencies {
		names[p.NamePT] = true
	}
	if !names["Conjunto de dados"] || !names["Conjunto de jogo: Trilha"] || names["Baralho de cartas"] {
		t.Errorf("proficiencies = %v, want the dice set and the named set, not the cards", d.Proficiencies)
	}
	if !strings.Contains(d.BackgroundEquipmentPT, "Corda de seda") {
		t.Errorf("equipment = %q, want the picked piece", d.BackgroundEquipmentPT)
	}
}

// The pick follows its group when the master reorders the groups; it stops counting when the master edits the group,
// which then asks for its picks again.
func TestAGroupPickFollowsItsGroupAndStopsWhenTheGroupChanges(t *testing.T) {
	t.Parallel()
	bg := groupBackground(skillGroup, itemGroup)
	picks := []string{
		ScopedChoice(BackgroundGroupKey(bg.Key, skillGroup), "skill:survival"),
		ScopedChoice(BackgroundGroupKey(bg.Key, itemGroup), "item:Lanterna furta-fogo"),
	}
	reordered := groupBackground(itemGroup, skillGroup)
	c := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Backgrounds: []TableBackground{reordered}})
	b := groupsBuild(bg, picks...)
	if n := groupPending(c.Choices(b)); n != 0 {
		t.Errorf("after a reorder: %d picks pending, want none (the picks follow their groups)", n)
	}
	if lvl := skillLevel(Derive(b, c), "skill:survival"); lvl != ProficiencyFull {
		t.Errorf("survival after the reorder = %v, want proficient", lvl)
	}
	edited := skillGroup
	edited.Options = []string{"skill:history", "skill:insight", "skill:survival", "skill:nature"}
	c = withOverlayOn(t, loadForTest(t), Overlay{Revision: 2, Backgrounds: []TableBackground{groupBackground(edited, itemGroup)}})
	if n := groupPending(c.Choices(b)); n != 1 {
		t.Errorf("after an edit: %d picks pending, want the edited group's 1", n)
	}
	if lvl := skillLevel(Derive(b, c), "skill:survival"); lvl != ProficiencyNone {
		t.Errorf("survival after the edit = %v, want the stale pick to count for nothing", lvl)
	}
	if err := Validate(b, c); err != nil {
		t.Errorf("Validate() = %v, want a sheet with a stale group pick still saved", err)
	}
	if got := c.Choices(b).NotOffered; len(got) != 0 {
		t.Errorf("not offered = %v, want the stale pick left out", got)
	}
}

func TestABackgroundGroupRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 81)
	cases := []struct {
		name   string
		bg     TableBackground
		field  string
		reason string
	}{
		{"picks as many as the options", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 2, Options: []string{"skill:history", "skill:insight"}}), "backgrounds[0].groups[0].choose", ReasonValue},
		{"one option", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:history"}}), "backgrounds[0].groups[0].options", ReasonLimit},
		{"a repeat", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:history", "skill:history", "skill:insight"}}), "backgrounds[0].groups[0].options[1]", ReasonValue},
		{"a fixed skill", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:athletics", "skill:insight"}}), "backgrounds[0].groups[0].options[0]", ReasonValue},
		{"not a skill", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:nothing", "skill:insight"}}), "backgrounds[0].groups[0].options[0]", ReasonReference},
		{"not a tool", groupBackground(BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"skill:history", "proficiency:dice-set"}}), "backgrounds[0].groups[0].options[0]", ReasonReference},
		{"an empty set name", groupBackground(BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"proficiency:dice-set"}, SetNames: []string{" "}}), "backgrounds[0].groups[0].gaming_set_names[0]", ReasonLimit},
		{"a long set name", groupBackground(BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"proficiency:dice-set"}, SetNames: []string{strings.Repeat("a", 41)}}), "backgrounds[0].groups[0].gaming_set_names[0]", ReasonLimit},
		{"a set twice", groupBackground(BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"proficiency:dice-set"}, SetNames: []string{"Trilha", "trilha"}}), "backgrounds[0].groups[0].gaming_set_names[1]", ReasonValue},
		{"a set in a skill group", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:history", "skill:insight"}, SetNames: []string{"Trilha"}}), "backgrounds[0].groups[0].gaming_set_names", ReasonValue},
		{"a long piece of equipment", groupBackground(BackgroundGroup{Kind: GroupEquipment, Choose: 1, Options: []string{long, "Corda"}}), "backgrounds[0].groups[0].options[0]", ReasonLimit},
		{"two groups the same", groupBackground(skillGroup, skillGroup), "backgrounds[0].groups[1]", ReasonValue},
		{"two skills and more", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 2, Options: []string{"skill:history", "skill:insight", "skill:survival"}}), "backgrounds[0].skills", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadForTest(t).With(Overlay{Revision: 1, Backgrounds: []TableBackground{tc.bg}})
			violationAt(t, err, tc.field, tc.reason)
		})
	}
}

// groupPending is how many picks the background's groups still ask.
func groupPending(s ChoiceSet) int {
	n := 0
	for _, p := range s.Pending() {
		if strings.Contains(p.ChoiceKey, groupKeyPart) {
			n += p.Missing
		}
	}
	return n
}
