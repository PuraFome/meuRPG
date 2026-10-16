package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Characterization of encounterParty (encounters.go): the message of each
// refusal, the order of the checks and the party it builds.

func TestEncounterParty_RefusalsAndTheirMessages(t *testing.T) {
	t.Parallel()
	m := newMirathel(t)
	long := strings.Repeat("a", 200)
	elevenOK := make([]*playv1.PartyNpc, 11)
	for i := range elevenOK {
		elevenOK[i] = &playv1.PartyNpc{Name: "x", Level: 1}
	}
	cases := []struct {
		name  string
		extra []*playv1.PartyNpc
		want  string
	}{
		{"eleven NPCs", elevenOK, "extra_party has at most 10 NPCs"},
		{"level 0", []*playv1.PartyNpc{{Name: "x", Level: 0}}, "extra_party[0].level must be 1 to 20"},
		{"level 21 as the second", []*playv1.PartyNpc{{Name: "x", Level: 1}, {Name: "x", Level: 21}}, "extra_party[1].level must be 1 to 20"},
		{"level before name", []*playv1.PartyNpc{{Level: 0}}, "extra_party[0].level must be 1 to 20"},
		{"no name and no character", []*playv1.PartyNpc{{Level: 3}}, ""},
		{"a name of two lines", []*playv1.PartyNpc{{Name: "a\nb", Level: 3}}, ""},
		{"a name too long", []*playv1.PartyNpc{{Name: long, Level: 3}}, ""},
		{"a name too long with a character", []*playv1.PartyNpc{{CharacterId: m.orin.GetId(), Name: long, Level: 3}}, ""},
		{"a character that is not a UUID", []*playv1.PartyNpc{{CharacterId: "nope", Level: 3}}, "extra_party[0].character_id is not a character of the campaign"},
		{"a character twice", []*playv1.PartyNpc{{CharacterId: m.orin.GetId(), Level: 3}, {CharacterId: m.orin.GetId(), Level: 4}}, "extra_party[1] repeats a character"},
		{"a player's character", []*playv1.PartyNpc{{CharacterId: m.toren.GetId(), Level: 3}}, "extra_party[0].character_id is not an NPC of the campaign"},
		{"a stranger", []*playv1.PartyNpc{{CharacterId: newKey(), Level: 3}}, "extra_party[0].character_id is not an NPC of the campaign"},
	}
	for _, tc := range cases {
		_, err := m.master.encounters.EvaluateEncounter(t.Context(), connect.NewRequest(&playv1.EvaluateEncounterRequest{CampaignId: m.campaignID, Entries: groups(ogre, 1), ExtraParty: tc.extra}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: code = %v (%v), want invalid_argument", tc.name, connect.CodeOf(err), err)
			continue
		}
		if ce, ok := err.(*connect.Error); !ok { //nolint:errorlint // the client returns it bare
			t.Errorf("%s: %v is not a connect error", tc.name, err)
		} else if tc.want != "" && ce.Message() != tc.want {
			t.Errorf("%s: message = %q, want %q", tc.name, ce.Message(), tc.want)
		} else if tc.want == "" && !strings.HasPrefix(ce.Message(), "extra_party[0].name ") {
			t.Errorf("%s: message = %q, want one about extra_party[0].name", tc.name, ce.Message())
		}
	}
}

func TestEncounterParty_BuildsTheMembersInOrder(t *testing.T) {
	t.Parallel()
	m := newMirathel(t)
	res, err := m.master.encounters.EvaluateEncounter(t.Context(), connect.NewRequest(&playv1.EvaluateEncounterRequest{
		CampaignId: m.campaignID, Entries: groups(ogre, 1),
		ExtraParty: []*playv1.PartyNpc{{CharacterId: m.orin.GetId(), Level: 3}, {Name: "  Capanga  ", Level: 2}},
	}))
	if err != nil {
		t.Fatalf("EvaluateEncounter() error = %v", err)
	}
	var got []string
	for _, p := range res.Msg.GetEvaluation().GetParty() {
		got = append(got, p.GetName()+"/"+itoa(p.GetLevel())+"/"+map[bool]string{true: "npc", false: "pc"}[p.GetNpc()])
	}
	want := []string{"Brisa/4/pc", "Pensantus/4/pc", "Sálvia/5/pc", "Toren/4/pc", "Orin, o guia/3/npc", "Capanga/2/npc"}
	if strings.Join(got[len(got)-2:], ",") != strings.Join(want[4:], ",") || len(got) != 6 {
		t.Errorf("party = %v, want the four players then the NPCs in the order given: %v", got, want)
	}

	res, err = m.master.encounters.EvaluateEncounter(t.Context(), connect.NewRequest(&playv1.EvaluateEncounterRequest{
		CampaignId: m.campaignID, Entries: groups(ogre, 1),
		ExtraParty: []*playv1.PartyNpc{{CharacterId: m.orin.GetId(), Name: "Guia", Level: 3}},
	}))
	if err != nil {
		t.Fatalf("EvaluateEncounter() error = %v", err)
	}
	if p := res.Msg.GetEvaluation().GetParty(); p[len(p)-1].GetName() != "Guia" || p[len(p)-1].GetCharacterId() != m.orin.GetId() {
		t.Errorf("last member = %v, want the name the master gave over the NPC's own", p[len(p)-1])
	}
}
