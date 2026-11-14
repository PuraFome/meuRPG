package rules

import (
	"slices"
	"strings"
	"testing"
)

// #580 S4: the "this or that" groups of a table background.

// groupBackground names as many skills as its skill groups leave of the 2 a background gives.
func groupBackground(groups ...BackgroundGroup) TableBackground {
	fixed := []string{"skill:athletics", "skill:arcana"}[:max(CustomBackgroundSkillCount-groupSkillPicks(groups), 0)]
	return TableBackground{
		TableEntry: TableEntry{Key: "background:marinheiro" + tableSuffix, NamePT: "Marinheiro de rio"},
		Skills:     fixed,
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
	edited.Options = []string{"skill:history", "skill:insight", "skill:nature"} // survival is gone too
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
		msg    string
	}{
		{"picks as many as the options", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 2, Options: []string{"skill:history", "skill:insight"}}), "backgrounds[0].groups[0].choose", ReasonValue, ""},
		{"one option", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:history"}}), "backgrounds[0].groups[0].options", ReasonLimit, ""},
		{"a repeat", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:history", "skill:history", "skill:insight"}}), "backgrounds[0].groups[0].options[1]", ReasonValue, ""},
		{"a fixed skill", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:athletics", "skill:insight"}}), "backgrounds[0].groups[0].options[0]", ReasonValue, ""},
		{"not a skill", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:nothing", "skill:insight"}}), "backgrounds[0].groups[0].options[0]", ReasonReference, ""},
		{"not a tool", groupBackground(BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"skill:history", "proficiency:dice-set"}}), "backgrounds[0].groups[0].options[0]", ReasonReference, ""},
		{"an empty set name", groupBackground(BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"proficiency:dice-set"}, SetNames: []string{" "}}), "backgrounds[0].groups[0].gaming_set_names[0]", ReasonLimit, ""},
		{"a long set name", groupBackground(BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"proficiency:dice-set"}, SetNames: []string{strings.Repeat("a", 41)}}), "backgrounds[0].groups[0].gaming_set_names[0]", ReasonLimit, ""},
		{"a set twice", groupBackground(BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"proficiency:dice-set"}, SetNames: []string{"Trilha", "trilha"}}), "backgrounds[0].groups[0].gaming_set_names[1]", ReasonValue, ""},
		{"a set in a skill group", groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:history", "skill:insight"}, SetNames: []string{"Trilha"}}), "backgrounds[0].groups[0].gaming_set_names", ReasonValue, ""},
		{"a long piece of equipment", groupBackground(BackgroundGroup{Kind: GroupEquipment, Choose: 1, Options: []string{long, "Corda"}}), "backgrounds[0].groups[0].options[0]", ReasonLimit, ""},
		{"two groups the same", groupBackground(skillGroup, skillGroup), "backgrounds[0].groups[1]", ReasonValue, ""},
		{"two skills and more", withSkills(groupBackground(BackgroundGroup{Kind: GroupSkill, Choose: 2, Options: []string{"skill:history", "skill:insight", "skill:survival"}}), "skill:athletics"), "backgrounds[0].skills", ReasonValue, ""},
		{"a skill in two groups", groupBackground(
			BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:history", "skill:insight"}},
			BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:insight", "skill:survival"}},
		), "backgrounds[0].groups[1].options[0]", ReasonValue, "is in another group of the background"},
		{"a tool in two groups", groupBackground(
			BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"proficiency:dice-set", "proficiency:playing-card-set"}},
			BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"proficiency:playing-card-set"}, SetNames: []string{"Trilha"}},
		), "backgrounds[0].groups[1].options[0]", ReasonValue, "is in another group of the background"},
		{"a line break in a piece of equipment", groupBackground(BackgroundGroup{Kind: GroupEquipment, Choose: 1, Options: []string{"Corda\nfalsa", "Lanterna"}}), "backgrounds[0].groups[0].options[0]", ReasonText, "hidden characters"},
		{"a NUL in a gaming set", groupBackground(BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"proficiency:dice-set"}, SetNames: []string{"Tri\x00lha"}}), "backgrounds[0].groups[0].gaming_set_names[0]", ReasonText, "hidden characters"},
		{"a piece with spaces around", groupBackground(BackgroundGroup{Kind: GroupEquipment, Choose: 1, Options: []string{"Corda ", "Lanterna"}}), "backgrounds[0].groups[0].options[0]", ReasonText, "start or end with spaces"},
		{"a set with spaces around", groupBackground(BackgroundGroup{Kind: GroupTool, Choose: 1, Options: []string{"proficiency:dice-set"}, SetNames: []string{" Trilha"}}), "backgrounds[0].groups[0].gaming_set_names[0]", ReasonText, "start or end with spaces"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadForTest(t).With(Overlay{Revision: 1, Backgrounds: []TableBackground{tc.bg}})
			violationAt(t, err, tc.field, tc.reason)
			if tc.msg != "" && !strings.Contains(err.Error(), tc.msg) {
				t.Errorf("error = %q, want it to say %q", err, tc.msg)
			}
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

func TestAnEquipmentPieceMayRepeatAcrossGroups(t *testing.T) {
	t.Parallel()
	bg := groupBackground(
		BackgroundGroup{Kind: GroupEquipment, Choose: 1, Options: []string{"Corda", "Lanterna"}},
		BackgroundGroup{Kind: GroupEquipment, Choose: 1, Options: []string{"Corda", "Tocha"}},
	)
	if _, err := loadForTest(t).With(Overlay{Revision: 1, Backgrounds: []TableBackground{bg}}); err != nil {
		t.Errorf("With() = %v, want equipment allowed in two groups", err)
	}
}

// A skill the sheet has from another source is no option, as a tool the sheet has is none.
func TestASkillTheSheetAlreadyHasIsNoGroupOption(t *testing.T) {
	t.Parallel()
	g := BackgroundGroup{Kind: GroupSkill, Choose: 1, Options: []string{"skill:intimidation", "skill:insight"}}
	bg := groupBackground(g)
	c := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Backgrounds: []TableBackground{bg}})
	b := groupsBuild(bg)
	b.Race = "race:half-orc" // Menacing: skill:intimidation
	ch := choiceOf(t, c.Choices(b), BackgroundGroupKey(bg.Key, g))
	if o := pickOption(t, ch, "skill:intimidation"); !o.Blocked() || o.ReasonPT != "Você já tem esta proficiência." {
		t.Errorf("intimidation: reason %q, want the sentence of a held proficiency", o.ReasonPT)
	}
	if pickOption(t, ch, "skill:insight").Blocked() {
		t.Error("insight is blocked")
	}
}

// A stored pick that names the group's own signature but a value it does not offer is refused; one under a stale
// signature is kept, up to the longest value a group can hold.
func TestAGroupPickWithTheGroupsSignatureAndAForeignValueIsRefused(t *testing.T) {
	t.Parallel()
	bg := groupBackground(skillGroup)
	c := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Backgrounds: []TableBackground{bg}})
	key := BackgroundGroupKey(bg.Key, skillGroup)
	if err := Validate(groupsBuild(bg, ScopedChoice(key, "skill:arcana")), c); err == nil {
		t.Error("Validate() accepted a value the group does not offer")
	}
	stale := bg.Key + groupKeyPart + "00000000"
	if err := Validate(groupsBuild(bg, ScopedChoice(stale, "skill:arcana")), c); err != nil {
		t.Errorf("Validate() = %v, want a stale pick kept", err)
	}
	if err := Validate(groupsBuild(bg, ScopedChoice(stale, "item:"+strings.Repeat("a", MaxGroupItemLength+1))), c); err == nil {
		t.Error("Validate() accepted a stale pick longer than any value a group holds")
	}
}

// The equipment and the tool picks go stale on an edit as the skill pick does.
func TestEquipmentAndToolPicksGoStaleOnAnEdit(t *testing.T) {
	t.Parallel()
	bg := groupBackground(toolGroup, itemGroup)
	picks := []string{
		ScopedChoice(BackgroundGroupKey(bg.Key, toolGroup), "set:Trilha"),
		ScopedChoice(BackgroundGroupKey(bg.Key, itemGroup), "item:Corda de seda"),
	}
	b := groupsBuild(bg, picks...)
	c := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Backgrounds: []TableBackground{bg}})
	if d := Derive(b, c); !strings.Contains(d.BackgroundEquipmentPT, "Corda de seda") {
		t.Fatalf("equipment = %q, want the pick before the edit", d.BackgroundEquipmentPT)
	}
	tool, item := toolGroup, itemGroup
	tool.SetNames = []string{"Trilha", "Gamão"}
	item.Options = []string{"Lanterna furta-fogo", "Tocha"}
	c = withOverlayOn(t, loadForTest(t), Overlay{Revision: 2, Backgrounds: []TableBackground{groupBackground(tool, item)}})
	d := Derive(b, c)
	if strings.Contains(d.BackgroundEquipmentPT, "Corda de seda") || slices.ContainsFunc(d.Proficiencies, func(p Proficiency) bool { return p.NamePT == "Conjunto de jogo: Trilha" }) {
		t.Errorf("after the edit: equipment %q, proficiencies %v; want both stale picks to count for nothing", d.BackgroundEquipmentPT, d.Proficiencies)
	}
	if n := groupPending(c.Choices(b)); n != 3 {
		t.Errorf("pending = %d, want the 2 tool picks and the 1 piece asked again", n)
	}
}

// A stale pick under an SRD background is accepted and counts for nothing.
func TestAStalePickUnderAnSRDBackgroundCountsForNothing(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := pickBuild("race:human", "")
	b.Background = "background:acolyte"
	b.FeatureChoices = []string{ScopedChoice("background:acolyte"+groupKeyPart+"deadbeef", "skill:insight")}
	if err := Validate(b, c); err != nil {
		t.Errorf("Validate() = %v, want the stale pick accepted", err)
	}
	with := Derive(b, c)
	b.FeatureChoices = nil
	if without := Derive(b, c); len(with.Skills) != len(without.Skills) || skillLevel(with, "skill:insight") != skillLevel(without, "skill:insight") {
		t.Error("the stale pick changed the skills")
	}
}

// No typed text forges a separator: two different groups never share a signature.
func TestTwoDifferentGroupsDoNotShareASignature(t *testing.T) {
	t.Parallel()
	a := BackgroundGroup{Kind: GroupEquipment, Choose: 1, Options: []string{"a", "b\x00c"}}
	b := BackgroundGroup{Kind: GroupEquipment, Choose: 1, Options: []string{"a\x00b", "c"}}
	if a.Signature() == b.Signature() {
		t.Error("two different groups hash alike")
	}
}

func withSkills(bg TableBackground, skills ...string) TableBackground {
	bg.Skills = skills
	return bg
}
