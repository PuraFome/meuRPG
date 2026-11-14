package characters

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// RN-23 for the class level, the pact, the spell and the option a feat asks (#580): a player's level-up options name
// none of them, in a key, a name or a condition; the master reads them as the positive control.

const (
	forgottenClass  = "Ordem Esquecida"
	forgottenPact   = "Pacto do Eco"
	forgottenSpell  = "Raio Esquecido"
	forgottenOption = "Estilo Esquecido"
)

func TestAPlayersLevelUpNamesNoHiddenClassPactSpellOrOption(t *testing.T) {
	t.Parallel()
	tb := newHiddenNamesTable(t)
	m := tb.master
	class := m.addEntry(t, tb.campaign, testClass(forgottenClass)).GetKey()
	pact := m.addEntry(t, tb.campaign, &rulesv1.TableOption{NamePt: forgottenPact, Set: "pact_boon", DescPt: []string{"Texto."}}).GetKey()
	spell := m.addEntry(t, tb.campaign, testSpell(forgottenSpell, "class:wizard")).GetKey()
	style := m.addEntry(t, tb.campaign, &rulesv1.TableOption{NamePt: forgottenOption, Set: "fighting_style", DescPt: []string{"Texto."}}).GetKey()
	feat := func(name string, p *rulesv1.FeatPrerequisite) string {
		return m.addEntry(t, tb.campaign, &rulesv1.TableFeat{NamePt: name, DescPt: []string{"Texto."}, Prerequisite: p}).GetKey()
	}
	byClass := feat("Pede a ordem", &rulesv1.FeatPrerequisite{ClassLevel: &rulesv1.FeatClassLevel{ClassKey: class, Level: 3}})
	byPact := feat("Pede o pacto", &rulesv1.FeatPrerequisite{RequiresPact: pact})
	bySpell := feat("Pede a magia", &rulesv1.FeatPrerequisite{RequiresSpellKey: spell})
	byOption := feat("Pede o estilo", &rulesv1.FeatPrerequisite{RequiresOptionKeys: []string{style}})
	m.setOff(t, tb.campaign, true, class, pact, spell, style)

	player, err := tb.options(tb.owner, tb.pc)
	if err != nil {
		t.Fatal(err)
	}
	master, err := tb.options(m, tb.pc)
	if err != nil {
		t.Fatal(err)
	}
	words := []string{forgottenClass, forgottenPact, forgottenSpell, forgottenOption, class, pact, spell, style}
	wantNames(t, "the master's options", master, true, forgottenClass, forgottenPact, forgottenSpell, forgottenOption, class, pact, spell, style)
	wantNames(t, "the player's options", player, false, words...)
	for _, key := range []string{byClass, byPact, bySpell, byOption} {
		got := optionOf(player, key)
		if got == nil {
			t.Fatalf("the feat %s is not offered to the player", key)
		}
		if got.GetQualifies() {
			t.Errorf("%s: the player qualifies for a feat asking for what they lack", key)
		}
		if len(got.GetUnmet()) != 1 || !got.GetUnmet()[0].GetHidden() || got.GetUnmet()[0].GetKey() != "" || got.GetUnmet()[0].GetNamePt() != "" {
			t.Errorf("%s: unmet = %v, want one hidden condition with no key or name", key, got.GetUnmet())
		}
		if control := optionOf(master, key); len(control.GetUnmet()) != 1 || control.GetUnmet()[0].GetHidden() || control.GetUnmet()[0].GetKey() == "" {
			t.Errorf("%s: the master's unmet = %v, want the condition itself", key, control.GetUnmet())
		}
	}
	// The player's creation cards say the same, with the words of the "Pede:" line.
	sheet := pensantusSheet()
	res, err := tb.owner.api.PreviewChoices(t.Context(), connect.NewRequest(&charactersv1.PreviewChoicesRequest{
		CampaignId: tb.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("PreviewChoices() error = %v", err)
	}
	wantNames(t, "the player's cards", res.Msg, false, words...)
}

// The choice list of a feat's options names no hidden entry its options ask for (option prerequisites name no hidden entry).
func TestAPlayersChoiceListNamesNoHiddenEntryAnOptionAsksFor(t *testing.T) {
	t.Parallel()
	tb := newHiddenNamesTable(t)
	m := tb.master
	class := m.addEntry(t, tb.campaign, testClass(forgottenClass)).GetKey()
	style := m.addEntry(t, tb.campaign, &rulesv1.TableOption{NamePt: forgottenOption, Set: "fighting_style", DescPt: []string{"Texto."}}).GetKey()
	spell := m.addEntry(t, tb.campaign, testSpell(forgottenSpell, "class:wizard")).GetKey()
	feat := m.addEntry(t, tb.campaign, &rulesv1.TableFeat{
		NamePt: "Posturas do vale", DescPt: []string{"Texto."},
		Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "feature", Count: 1}},
		Options: []*rulesv1.TableFeature{{NamePt: "Postura exigente", DescPt: []string{"Texto."}, Prerequisite: &rulesv1.OptionPrerequisite{
			ClassLevel: &rulesv1.OptionClassLevel{ClassKey: class, Level: 3}, RequiresOptionKeys: []string{style}, RequiresSpellKey: spell,
		}}},
	}).GetKey()
	m.setOff(t, tb.campaign, true, class, style, spell)

	sheet := pensantusSheet()
	sheet.GetFull().FeatKeys = []string{feat}
	preview := func(u *user, kind charactersv1.CharacterKind) *charactersv1.PreviewChoicesResponse {
		res, err := u.api.PreviewChoices(t.Context(), connect.NewRequest(&charactersv1.PreviewChoicesRequest{CampaignId: tb.campaign, Kind: kind, Sheet: sheet}))
		if err != nil {
			t.Fatalf("PreviewChoices() error = %v", err)
		}
		return res.Msg
	}
	master := preview(m, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY)
	player := preview(tb.owner, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER)
	wantNames(t, "the master's choice list", master, true, "Postura exigente", forgottenClass, forgottenOption, forgottenSpell)
	wantNames(t, "the player's choice list", player, true, "Postura exigente")
	wantNames(t, "the player's choice list", player, false, forgottenClass, forgottenOption, forgottenSpell, class, style, spell)
}
