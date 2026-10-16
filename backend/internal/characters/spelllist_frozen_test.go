package characters

import (
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Characterization of ListSpells: what each viewer reads of the same requests (a table
// spell, a class switched off, an archived spell, a character's lists, the pages), and
// every refusal with its code, message and field, in the order of the checks. The
// answers are golden files (testdata/spells_*.golden.json, see choicesGolden).

// TestListSpellsFrozenViewers freezes what the master, the owner of a wizard, another
// player, a pending member, a stranger and nobody read of the same requests.
func TestListSpellsFrozenViewers(t *testing.T) {
	t.Parallel()
	tb := newChoicesFrozenTable(t)
	master, owner, other := tb.master, tb.ana, tb.caio
	master.addEntry(t, tb.campaign, testSpell("Lâmina de Nanquim", "class:wizard", "class:sorcerer"))
	oculta := master.addEntry(t, tb.campaign, testSpell("Nanquim Oculto", "class:wizard"))
	if _, err := master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: tb.campaign, Key: oculta.GetKey()})); err != nil {
		t.Fatalf("ArchiveTableEntry() error = %v", err)
	}
	pens := tb.name(owner.createPensantus(t, tb.campaign), "pensantus")
	// The wizard is switched off: its key leaves the spells' classes for a player, and
	// asking for its list answers as an unknown class to one whose sheet lacks it.
	master.setOff(t, tb.campaign, true, "class:wizard")
	tb.logs.take(t)

	requests := []struct {
		name string
		req  *rulesv1.ListSpellsRequest
	}{
		{"by name, a table spell and an archived one", &rulesv1.ListSpellsRequest{Query: "nanquim"}},
		{"the switched-off wizard's 3rd circle of evocation", &rulesv1.ListSpellsRequest{ClassKey: "class:wizard", Levels: []int32{3}, SchoolKeys: []string{"school:evocation"}}},
		{"the sorcerer's 1st circle, by name", &rulesv1.ListSpellsRequest{ClassKey: "class:sorcerer", Levels: []int32{1}, Query: "mag"}},
		{"an unknown class", &rulesv1.ListSpellsRequest{ClassKey: "class:nope"}},
		{"what Pensantus can learn of the 1st circle, by name", &rulesv1.ListSpellsRequest{CharacterId: pens.GetId(), Levels: []int32{1}, Query: "es"}},
		{"the 9th circle in pages of 10", &rulesv1.ListSpellsRequest{Levels: []int32{9}, PageSize: 10}},
		{"the default page of the 9th circle", &rulesv1.ListSpellsRequest{Levels: []int32{9}}},
	}
	viewers := []struct {
		name string
		u    *user
	}{{"master", master}, {"owner", owner}, {"another player", other}, {"pending member", tb.pending}, {"stranger", tb.alien}, {"nobody", tb.h.anonymous()}}
	var pendingGot []any
	for _, v := range viewers {
		var got []any
		for _, r := range requests {
			req := &rulesv1.ListSpellsRequest{
				CampaignId: tb.campaign, Query: r.req.GetQuery(), ClassKey: r.req.GetClassKey(), Levels: r.req.GetLevels(),
				SchoolKeys: r.req.GetSchoolKeys(), PageSize: r.req.GetPageSize(), CharacterId: r.req.GetCharacterId(),
			}
			entry := map[string]any{"request": r.name}
			res, err := v.u.content.ListSpells(t.Context(), connect.NewRequest(req))
			if err != nil {
				entry["refusal"] = choicesRefusalShape(t, err)
			} else {
				entry["answer"] = choicesProtoJSON(t, res.Msg)
				if token := res.Msg.GetNextPageToken(); token != "" {
					req.PageToken = token
					next, err := v.u.content.ListSpells(t.Context(), connect.NewRequest(req))
					if err != nil {
						entry["next page refusal"] = choicesRefusalShape(t, err)
					} else {
						entry["next page"] = choicesProtoJSON(t, next.Msg)
					}
				}
			}
			entry["logs"] = tb.logs.take(t)
			got = append(got, entry)
		}
		switch v.name {
		case "pending member":
			// A pending member is refused as a stranger is: the stranger's golden is theirs too.
			pendingGot = got
			continue
		case "stranger":
			if !reflect.DeepEqual(got, pendingGot) {
				t.Errorf("the pending member reads %v, want what the stranger reads %v", pendingGot, got)
			}
		}
		choicesGolden(t, "spells_"+strings.ReplaceAll(v.name, " ", "_"), got, tb.names...)
	}
}

// TestListSpellsFrozenRefusals freezes every refusal of ListSpells with its code, message
// and field, and which one wins when a request breaks several rules.
func TestListSpellsFrozenRefusals(t *testing.T) {
	t.Parallel()
	tb := newChoicesFrozenTable(t)
	pens := tb.name(tb.ana.createPensantus(t, tb.campaign), "pensantus")
	npc := tb.name(tb.master.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_STORY, "Taverneiro", basicSheet()), "story-npc")
	beyond := &rulesv1.ListSpellsRequest{CampaignId: tb.campaign, Query: "escudo"}
	beyond.PageToken = spellsToken(10000, spellsFilterID(beyond))
	atEnd := &rulesv1.ListSpellsRequest{CampaignId: tb.campaign, Query: "escudo arcano"}
	atEnd.PageToken = spellsToken(1, spellsFilterID(atEnd))
	tb.logs.take(t)
	long := strings.Repeat("a", maxSpellQuery+1)
	cases := []struct {
		name string
		u    *user
		req  *rulesv1.ListSpellsRequest
	}{
		{"a query of 101 characters", tb.ana, &rulesv1.ListSpellsRequest{Query: long}},
		{"a query of 100 accented characters", tb.ana, &rulesv1.ListSpellsRequest{Query: strings.Repeat("á", maxSpellQuery)}},
		{"the query before the levels", tb.ana, &rulesv1.ListSpellsRequest{Query: long, Levels: make([]int32, maxSpellFilters+1)}},
		{"21 levels before the schools", tb.ana, &rulesv1.ListSpellsRequest{Levels: make([]int32, maxSpellFilters+1), SchoolKeys: make([]string, maxSpellFilters+1)}},
		{"21 schools before a level out of range", tb.ana, &rulesv1.ListSpellsRequest{Levels: []int32{10}, SchoolKeys: make([]string, maxSpellFilters+1)}},
		{"circle 10", tb.ana, &rulesv1.ListSpellsRequest{Levels: []int32{1, 10}}},
		{"circle -1", tb.ana, &rulesv1.ListSpellsRequest{Levels: []int32{-1}}},
		{"a level before the page size", tb.ana, &rulesv1.ListSpellsRequest{Levels: []int32{10}, PageSize: -1}},
		{"page size 401", tb.ana, &rulesv1.ListSpellsRequest{PageSize: maxSpellsPage + 1}},
		{"page size -1", tb.ana, &rulesv1.ListSpellsRequest{PageSize: -1}},
		{"the page size before the token", tb.ana, &rulesv1.ListSpellsRequest{PageSize: -1, PageToken: "x"}},
		{"a token that is not base64", tb.ana, &rulesv1.ListSpellsRequest{PageToken: "!!"}},
		{"a token of other filters", tb.ana, &rulesv1.ListSpellsRequest{Query: "a", PageToken: spellsToken(5, spellsFilterID(&rulesv1.ListSpellsRequest{}))}},
		{"a token with a negative offset", tb.ana, &rulesv1.ListSpellsRequest{PageToken: spellsToken(-1, spellsFilterID(&rulesv1.ListSpellsRequest{}))}},
		{"the token before the character", tb.ana, &rulesv1.ListSpellsRequest{PageToken: "x", CharacterId: "not-an-id"}},
		{"a token past the end of the list", tb.ana, beyond},
		{"a token at the end of the list", tb.ana, atEnd},
		{"a character that is no uuid", tb.ana, &rulesv1.ListSpellsRequest{CharacterId: "not-an-id"}},
		{"a character that does not exist", tb.ana, &rulesv1.ListSpellsRequest{CharacterId: "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e003"}},
		{"another player's character", tb.caio, &rulesv1.ListSpellsRequest{CharacterId: pens.GetId()}},
		{"a story NPC, as the master", tb.master, &rulesv1.ListSpellsRequest{CharacterId: npc.GetId()}},
		{"the character before a token past the end", tb.caio, &rulesv1.ListSpellsRequest{CharacterId: pens.GetId(), Query: "escudo", PageToken: spellsToken(10000, spellsFilterID(&rulesv1.ListSpellsRequest{CharacterId: pens.GetId(), Query: "escudo"}))}},
		{"a pending member, before the query", tb.pending, &rulesv1.ListSpellsRequest{Query: long}},
		{"a stranger", tb.alien, &rulesv1.ListSpellsRequest{}},
		{"nobody", tb.h.anonymous(), &rulesv1.ListSpellsRequest{}},
	}
	var got []any
	for _, tc := range cases {
		if tc.req.CampaignId == "" {
			tc.req.CampaignId = tb.campaign
		}
		entry := map[string]any{"case": tc.name}
		res, err := tc.u.content.ListSpells(t.Context(), connect.NewRequest(tc.req))
		if err != nil {
			entry["refusal"] = choicesRefusalShape(t, err)
		} else {
			entry["answer"] = choicesProtoJSON(t, res.Msg)
		}
		entry["logs"] = tb.logs.take(t)
		got = append(got, entry)
	}
	choicesGolden(t, "spells_refusals", got, tb.names...)
}
