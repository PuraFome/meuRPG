package leaktest

import (
	"slices"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The skills the editor offers come from the sources of the sheet: a table class feature the table switched off is no source,
// so the lists of PreviewChoices for a draft of the sheet are those of a table without it, for the player who owns the sheet
// and for the master alike.
func TestASwitchedOffClassFeatureAddsNoSkillKeyToTheListsForADraft(t *testing.T) {
	p := newChoicesProbe(t)
	w, ctx := p.w, t.Context()
	const extra = "skill:performance"
	held := must(w.ana.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Menestrel", Sheet: sheet("class:wizard", 1),
	}))).GetCharacter()
	// lists are the lists of PreviewChoices for the draft as who reads it: skills and expertise, joined.
	lists := func(who *person) string {
		r := who.call(charactersv1connect.CharacterServicePreviewChoicesProcedure, &charactersv1.PreviewChoicesRequest{
			CampaignId: w.campaign, CharacterId: held.GetId(), Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Sheet: sheet("class:wizard", 1),
		})
		if !r.ok() {
			t.Fatalf("PreviewChoices: HTTP %d %s", r.status, shorten(r.body))
		}
		c := choicesOf(t, r)
		return strings.Join(c.GetSkillsAllowed().GetKeys(), ",") + " | " + strings.Join(c.GetExpertiseAllowed().GetKeys(), ",")
	}
	without := map[string]string{"the master": lists(w.master), "Ana": lists(w.ana)}
	if strings.Contains(without["Ana"], extra) {
		t.Fatalf("without the feature the list holds %s: the row measures nothing", extra)
	}
	feature := must(w.master.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableClassFeature{TableClassFeature: &rulesv1.TableClassFeature{
			NamePt: "Canto", DescPt: []string{"Texto de teste."}, ClassKey: "class:wizard", Level: 1,
			Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "skill", Count: 1, From: []string{extra}}, {Type: "choice", Choice: "expertise", Count: 1, From: []string{extra}}},
		}},
	}))).GetEntry()
	for name, who := range map[string]*person{"the master": w.master, "Ana": w.ana} { // the positive control: the open feature is listed
		if got := lists(who); !strings.Contains(got, extra) || got == without[name] {
			t.Fatalf("%s: before the switch the lists are %q, want %s listed (without the feature %q)", name, got, extra, without[name])
		}
	}
	must(w.master.table.SetOptionSwitches(ctx, rq(&rulesv1.SetOptionSwitchesRequest{
		CampaignId: w.campaign, Switches: []*rulesv1.OptionSwitch{{Key: feature.GetKey(), Off: true}},
	})))
	for name, who := range map[string]*person{"the master": w.master, "Ana": w.ana} {
		if got := lists(who); got != without[name] {
			t.Errorf("%s: with the feature switched off the lists are %q, want those of a table without it: %q", name, got, without[name])
		}
	}
}

// The same list with the source one table class feature the master archived: the feature is on a class the table keeps (the
// SRD wizard) and the character still gets it, so its key is on a player's list as on the master's.
func TestAnArchivedClassFeatureTheCharacterGetsAddsItsSkillKeyToAPlayersListForADraft(t *testing.T) {
	p := newChoicesProbe(t)
	w, ctx := p.w, t.Context()
	const extra = "skill:performance"
	held := must(w.ana.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Menestrel", Sheet: sheet("class:wizard", 1),
	}))).GetCharacter()
	listed := func(who *person) bool {
		r := who.call(charactersv1connect.CharacterServicePreviewChoicesProcedure, &charactersv1.PreviewChoicesRequest{
			CampaignId: w.campaign, CharacterId: held.GetId(), Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Sheet: sheet("class:wizard", 1),
		})
		if !r.ok() {
			t.Fatalf("PreviewChoices: HTTP %d %s", r.status, shorten(r.body))
		}
		return slices.Contains(choicesOf(t, r).GetSkillsAllowed().GetKeys(), extra)
	}
	feature := must(w.master.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableClassFeature{TableClassFeature: &rulesv1.TableClassFeature{
			NamePt: "Canto", DescPt: []string{"Texto de teste."}, ClassKey: "class:wizard", Level: 1,
			Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "skill", Count: 1, From: []string{extra}}},
		}},
	}))).GetEntry()
	for name, who := range map[string]*person{"the master": w.master, "Ana": w.ana} {
		if !listed(who) {
			t.Fatalf("%s: before the archive the list lacks %s, the positive control", name, extra)
		}
	}
	must(w.master.table.ArchiveTableEntry(ctx, rq(&rulesv1.ArchiveTableEntryRequest{CampaignId: w.campaign, Key: feature.GetKey()})))
	if !listed(w.master) {
		t.Errorf("the master reads what is archived, and the list lacks %s", extra)
	}
	if !listed(w.ana) {
		t.Errorf("Ana's list lacks %s of an archived class feature the character gets", extra)
	}
}
