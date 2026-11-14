package characters

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// #580 S4 through the service: a table background with "this or that" groups, as the master writes it and a sheet
// takes it.

func groupedBackground() *rulesv1.TableBackground {
	k := rulesv1.TableBackgroundGroupKind_TABLE_BACKGROUND_GROUP_KIND_SKILL
	return &rulesv1.TableBackground{
		NamePt: "Marinheiro de rio", Skills: []string{"skill:athletics"},
		Feature: testNote("rio", "Rio conhecido", "Conhece os barqueiros."),
		Groups: []*rulesv1.TableBackgroundGroup{
			{Kind: k, Choose: 1, Options: []string{"skill:history", "skill:insight", "skill:survival"}},
			{Kind: rulesv1.TableBackgroundGroupKind_TABLE_BACKGROUND_GROUP_KIND_TOOL, Choose: 1, Options: []string{"proficiency:dice-set"}, GamingSetNames: []string{"Trilha"}},
		},
	}
}

func TestABackgroundGroupRefusalsNameTheirFieldAtTheService(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel")
	cases := []struct {
		name  string
		edit  func(b *rulesv1.TableBackground)
		field string
	}{
		{"as many picks as options", func(b *rulesv1.TableBackground) { b.Groups[1].Choose = 2 }, "table_background.groups[1].choose"},
		{"an empty set name", func(b *rulesv1.TableBackground) { b.Groups[1].GamingSetNames = []string{""} }, "table_background.groups[1].gaming_set_names[0]"},
		{"a fixed skill in a group", func(b *rulesv1.TableBackground) { b.Groups[0].Options[0] = "skill:athletics" }, "table_background.groups[0].options[0]"},
		{"three skills", func(b *rulesv1.TableBackground) { b.Skills = []string{"skill:athletics", "skill:arcana"} }, "table_background.skills"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := groupedBackground()
			tc.edit(b)
			_, err := master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(campaign, b)))
			if got := violationFields(violationsOfErr(t, err)); !slices.Contains(got, tc.field) {
				t.Errorf("violations at %v, want one at %s", got, tc.field)
			}
		})
	}
}

func TestASheetTakesABackgroundGroupPickAndTheMasterEditingTheGroupAsksAgain(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", player)
	bg := master.addEntry(t, campaign, groupedBackground())
	body := bodyMessage(bg).(*rulesv1.TableBackground)
	skillKey := rules.BackgroundGroupKey(bg.GetKey(), rules.BackgroundGroup{
		Kind: rules.GroupSkill, Choose: 1, Options: body.GetGroups()[0].GetOptions(),
	})
	toolKey := rules.BackgroundGroupKey(bg.GetKey(), rules.BackgroundGroup{
		Kind: rules.GroupTool, Choose: 1, Options: body.GetGroups()[1].GetOptions(), SetNames: body.GetGroups()[1].GetGamingSetNames(),
	})

	sheet := pensantusSheet()
	sheet.GetFull().Background = &charactersv1.FullSheet_BackgroundKey{BackgroundKey: bg.GetKey()}
	sheet.GetFull().SkillProficiencyKeys = []string{"skill:investigation", "skill:arcana"}
	sheet.GetFull().FeatureChoiceKeys = []string{rules.ScopedChoice(skillKey, "skill:insight"), rules.ScopedChoice(toolKey, "set:Trilha")}
	pc := player.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", sheet)
	d := pc.GetDerived()
	if !slices.ContainsFunc(d.GetSkills(), func(s *rulesv1.DerivedSkill) bool {
		return s.GetKey() == "skill:insight" && s.GetProficiency() == rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_PROFICIENT
	}) || !slices.Contains(d.GetProficiencies().GetTools(), "Conjunto de jogo: Trilha") {
		t.Fatalf("skills %v, proficiencies %v; want insight and the named set", d.GetSkills(), d.GetProficiencies())
	}
	if h.lockSheets(campaign) != 1 {
		t.Fatal("expected one sheet to lock")
	}
	// The master edits the skill group: its picks are asked again, and the sheet says so.
	edited := proto.Clone(body).(*rulesv1.TableBackground)
	edited.Groups[0].Options = append(edited.Groups[0].Options, "skill:nature")
	if _, err := master.table.UpdateTableEntry(t.Context(), connect.NewRequest(updateReq(campaign, bg.GetKey(), bg.GetRevision(), edited))); err != nil {
		t.Fatalf("UpdateTableEntry() error = %v", err)
	}
	got := player.get(t, campaign, pc.GetId()).GetDerived()
	if !slices.ContainsFunc(got.GetIssues(), func(is *rulesv1.Issue) bool {
		return strings.Contains(is.GetMessage(), "Falta escolher 1 no antecedente")
	}) {
		t.Errorf("issues = %v, want one pick missing in the background", got.GetIssues())
	}
	if slices.ContainsFunc(got.GetSkills(), func(s *rulesv1.DerivedSkill) bool {
		return s.GetKey() == "skill:insight" && s.GetProficiency() == rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_PROFICIENT
	}) {
		t.Errorf("insight still proficient after the group changed")
	}
}
