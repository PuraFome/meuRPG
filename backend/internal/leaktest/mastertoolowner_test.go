package leaktest

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The master saving a tool pick: under an archived owner the sheet neither holds nor gets it is answered as for an owner that
// does not exist; under an owner that is only switched off the pick is saved; under an archived class feature the character
// gets it works like its skills.
func TestTheMasterSavesAToolPickByTheOwnerTheSheetHoldsOrGets(t *testing.T) {
	p := newChoicesProbe(t)
	w, ctx := p.w, t.Context()
	const tool, other, absent = "proficiency:smiths-tools", "proficiency:thieves-tools", "feature:nenhum--dono@mesa"
	feature := func(name string, level int32) *rulesv1.TableEntry {
		return must(w.master.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
			CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableClassFeature{TableClassFeature: &rulesv1.TableClassFeature{
				NamePt: name, DescPt: []string{"Texto de teste."}, ClassKey: "class:wizard", Level: level,
				Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "tool", Count: 1, From: []string{tool, other}}},
			}},
		}))).GetEntry()
	}
	away, off, got := feature("Oficio longe", 20), feature("Oficio fechado", 1), feature("Oficio obtido", 1)
	must(w.master.table.SetOptionSwitches(ctx, rq(&rulesv1.SetOptionSwitchesRequest{CampaignId: w.campaign, Switches: []*rulesv1.OptionSwitch{{Key: off.GetKey(), Off: true}}})))
	must(w.master.table.ArchiveTableEntry(ctx, rq(&rulesv1.ArchiveTableEntryRequest{CampaignId: w.campaign, Key: away.GetKey()})))
	must(w.master.table.ArchiveTableEntry(ctx, rq(&rulesv1.ArchiveTableEntryRequest{CampaignId: w.campaign, Key: got.GetKey()})))
	save := func(owner, value string) reply {
		s := sheet("class:wizard", 1)
		s.GetFull().FeatureChoiceKeys = []string{rules.ScopedChoice(rules.ToolPickKey(owner), value)}
		return w.master.call(charactersv1connect.CharacterServiceCreateCharacterProcedure, &charactersv1.CreateCharacterRequest{
			CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, Name: "Mestre", Sheet: s,
		})
	}
	none := save(absent, tool)
	if none.ok() {
		t.Fatalf("an owner that does not exist is accepted: the row measures nothing")
	}
	// (a) an archived owner the sheet neither holds nor gets.
	if named := save(away.GetKey(), tool); !sameAnswer(named, away.GetKey(), none, absent) {
		t.Errorf("archived owner the sheet does not get: HTTP %d %s\nan owner that does not exist: HTTP %d %s", named.status, shorten(named.body), none.status, shorten(none.body))
	}
	// (b) an owner that is only switched off: the master saves the pick.
	if named := save(off.GetKey(), tool); !named.ok() {
		t.Errorf("switched-off owner: HTTP %d %s, want the pick saved", named.status, shorten(named.body))
	}
	// (c) an archived class feature the character gets: its tool on the list is saved, one off the list is a pick no list allows
	// and the master's save keeps it (the fit issue tells it).
	for _, value := range []string{tool, "proficiency:glaives"} {
		if named := save(got.GetKey(), value); !named.ok() {
			t.Errorf("archived feature the sheet gets, %s: HTTP %d %s, want the pick saved", value, named.status, shorten(named.body))
		}
	}
}
