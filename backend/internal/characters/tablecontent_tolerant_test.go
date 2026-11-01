package characters

import (
	"slices"
	"testing"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// storedEntry is a stored entry as the engine reads it; name is the stored name column.
func storedEntry(kind rulesv1.TableContentKind, key, name string, body tableBody) entryRow {
	featureKeys(key, body, nil) // the keys the editor gives the features
	return entryRow{row: charactersdb.CampaignContent{ContentKey: key, Kind: kindPrefix(kind), NamePt: name}, kind: kind, body: body}
}

func TestBuildTolerantLeavesOutARefusedEntryAndWhatDependsOnIt(t *testing.T) {
	t.Parallel()
	srd := loadRules(t)
	entries := []entryRow{
		storedEntry(rulesv1.TableContentKind_TABLE_CONTENT_KIND_CLASS, "class:guardiao@mesa", "", testClass("Guardião")), // no name: refused
		storedEntry(rulesv1.TableContentKind_TABLE_CONTENT_KIND_SUBCLASS, "subclass:caminho@mesa", "Caminho", testSubclass("Caminho", "class:guardiao@mesa")),
		storedEntry(rulesv1.TableContentKind_TABLE_CONTENT_KIND_RACE, "race:gnomo-teste@mesa", "Gnomo de teste", testRace("Gnomo de teste")),
	}
	c, out, err := buildTolerant(srd, entries, 3, nil)
	if err != nil {
		t.Fatalf("buildTolerant() error = %v", err)
	}
	if c.NamePT("race:gnomo-teste@mesa") != "Gnomo de teste" {
		t.Errorf("the healthy race is not in the content")
	}
	if c.NamePT("class:guardiao@mesa") != "" || c.NamePT("subclass:caminho@mesa") != "" {
		t.Errorf("the refused class or its subclass is in the content")
	}
	reasons := map[string]string{}
	for _, l := range out {
		reasons[l.Key] = l.Violations[0].GetReason()
	}
	want := map[string]string{"class:guardiao@mesa": rules.ReasonName, "subclass:caminho@mesa": rules.ReasonReference}
	if len(reasons) != len(want) || reasons["class:guardiao@mesa"] != want["class:guardiao@mesa"] || reasons["subclass:caminho@mesa"] != want["subclass:caminho@mesa"] {
		t.Errorf("left out = %v, want %v", reasons, want)
	}
	got := c.Refused()
	if len(got) != 2 || got[0].Key != "class:guardiao@mesa" || got[1].Key != "subclass:caminho@mesa" || got[0].Reason == "" {
		t.Errorf("Refused() = %v, want the class and the subclass in key order", got)
	}
	if kinds := []rulesv1.TableContentKind{out[0].Kind, out[1].Kind}; !slices.Contains(kinds, rulesv1.TableContentKind_TABLE_CONTENT_KIND_SUBCLASS) {
		t.Errorf("kinds = %v, want the subclass among them", kinds)
	}
}

func TestBuildTolerantHealthyLeavesNothingOut(t *testing.T) {
	t.Parallel()
	srd := loadRules(t)
	entries := []entryRow{storedEntry(rulesv1.TableContentKind_TABLE_CONTENT_KIND_RACE, "race:gnomo-teste@mesa", "Gnomo de teste", testRace("Gnomo de teste"))}
	c, out, err := buildTolerant(srd, entries, 1, nil)
	if err != nil || len(out) != 0 || len(c.Refused()) != 0 || c.NamePT("race:gnomo-teste@mesa") != "Gnomo de teste" {
		t.Errorf("buildTolerant() = %v, %v, %v; want the race and nothing left out", c, out, err)
	}
}

func TestBuildTolerantFailsWhenTheErrorNamesNoEntryOrAStrictOne(t *testing.T) {
	t.Parallel()
	srd := loadRules(t)
	bad := storedEntry(rulesv1.TableContentKind_TABLE_CONTENT_KIND_RACE, "race:sem-nome@mesa", "", testRace(""))
	if _, _, err := buildTolerant(srd, []entryRow{bad}, 1, nil, "race:sem-nome@mesa"); err == nil {
		t.Error("a strict entry was left out")
	}
	if _, out, err := buildTolerant(srd, []entryRow{bad}, 1, nil); err != nil || len(out) != 1 {
		t.Errorf("a non-strict one: %v, %v; want it left out", out, err)
	}
	// A violation of the content as a whole (a negative revision) names no entry.
	race := storedEntry(rulesv1.TableContentKind_TABLE_CONTENT_KIND_RACE, "race:gnomo-teste@mesa", "Gnomo de teste", testRace("Gnomo de teste"))
	if _, _, err := buildTolerant(srd, []entryRow{race}, -1, nil); err == nil {
		t.Error("a keyless violation was tolerated")
	}
}
