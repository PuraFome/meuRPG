package characters

import (
	"slices"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// resilientFeat is a feat in the shape of Resilient: a named pick of an ability, the proficiency in its save,
// and a list of two options.
func resilientFeat() *rulesv1.TableFeat {
	return &rulesv1.TableFeat{
		NamePt: "Resistente", DescPt: []string{"Resiste."},
		Effects: []*rulesv1.TableEffect{
			{Type: "choice", Choice: "ability", Name: "habilidade", Count: 1, From: []string{"str", "dex"}},
			{Type: "proficiency", Proficiency: "save.@habilidade"},
			{Type: "modifier", Target: "score.str", Mode: "add", Value: "1", Cap: 20},
			{Type: "choice", Choice: "feature", Count: 1},
		},
		Options: []*rulesv1.TableFeature{
			{NamePt: "Opção A", DescPt: []string{"A."}, Effects: []*rulesv1.TableEffect{{Type: "note", TextPt: "A vale."}}},
			{NamePt: "Opção B", DescPt: []string{"B."}},
		},
	}
}

// TestAFeatKeepsItsOptionsPicksAndCap: the options get keys from the server, the name, set and cap of the
// effects come back as they were sent, and a send-back keeps the keys.
func TestAFeatKeepsItsOptionsPicksAndCap(t *testing.T) {
	t.Parallel()
	ft := newFeatsTable(t)
	entry := ft.master.addEntry(t, ft.campaign, resilientFeat())
	feat := entry.GetTableFeat()
	if len(feat.GetOptions()) != 2 || feat.GetOptions()[0].GetKey() == "" || feat.GetOptions()[0].GetKey() == feat.GetOptions()[1].GetKey() {
		t.Fatalf("options = %v, want two with their own keys", feat.GetOptions())
	}
	if e := feat.GetEffects()[0]; e.GetName() != "habilidade" || e.GetChoice() != "ability" {
		t.Errorf("the pick = %v", e)
	}
	if feat.GetEffects()[2].GetCap() != 20 {
		t.Errorf("the cap = %d", feat.GetEffects()[2].GetCap())
	}
	// An effect that reads a pick that was never named is refused at the path of the feat.
	bad := resilientFeat()
	bad.Effects[1].Proficiency = "save.@outra"
	_, err := ft.master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(ft.campaign, bad)))
	wantCode(t, "a feat reading a pick it never named", err, connect.CodeInvalidArgument)
	// An option past the limit of 60 is a limit violation at the options.
	many := resilientFeat()
	for range 60 {
		many.Options = append(many.Options, &rulesv1.TableFeature{NamePt: "x", DescPt: []string{"x"}})
	}
	_, err = ft.master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(ft.campaign, many)))
	wantCode(t, "a feat with too many options", err, connect.CodeInvalidArgument)
}

// TestAPlayersSaveWithAFeatPickIsCheckedAsAnyChoice: the master gives the feat, the player's save keeps it; a pick
// the feat does not offer is CHOICE_NOT_OFFERED, one it offers is saved.
func TestAPlayersSaveWithAFeatPickIsCheckedAsAnyChoice(t *testing.T) {
	t.Parallel()
	ft := newFeatsTable(t)
	draftCampaign := ft.h.newCampaign(ft.master, "Rascunho", ft.owner)
	entry := ft.master.addEntry(t, draftCampaign, resilientFeat())
	key := entry.GetKey()
	options := entry.GetTableFeat().GetOptions()
	draft := ft.owner.create(t, draftCampaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", pensantusSheet())
	// The feat is the master's to give; the draft is not complete until the picks are made.
	given := proto.CloneOf(draft.GetSheet())
	given.GetFull().FeatKeys = []string{key}
	held, err := ft.master.update(t, draft, draft.GetName(), given)
	if err != nil {
		t.Fatalf("the master gives the feat: %v", err)
	}
	pick := func(value string) string { return rules.ScopedChoice(rules.PickChoiceKey(key, "habilidade"), value) }
	save := func(picks ...string) error {
		s := proto.CloneOf(held.GetSheet())
		s.GetFull().FeatureChoiceKeys = append(slices.Clone(s.GetFull().GetFeatureChoiceKeys()), picks...)
		_, err := ft.owner.update(t, held, held.GetName(), s)
		return err
	}
	for name, picks := range map[string][]string{
		"an ability outside the list": {pick("ability:cha"), options[0].GetKey()},
		"an option of no feat":        {pick("ability:dex"), "feature:fighter-fighting-style-defense"},
	} {
		err := save(picks...)
		if r := choiceRefusal(t, name, err); r.GetReason() != charactersv1.ChoiceRefusalReason_CHOICE_REFUSAL_REASON_CHOICE_NOT_OFFERED {
			t.Errorf("%s: refusal = %v, want CHOICE_NOT_OFFERED", name, r)
		}
	}
	if err := save(pick("ability:dex"), options[1].GetKey()); err != nil {
		t.Fatalf("a valid pick and option: %v", err)
	}
}
