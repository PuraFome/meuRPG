package characters

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

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

// hiddenKnowledge is the table of the hidden-names fixture plus a hidden class, pact, spell and option, and a feat that
// asks for each.
type hiddenKnowledge struct {
	*hiddenNamesTable
	class, pact, spell, style          string
	byClass, byPact, bySpell, byOption string
}

func newHiddenKnowledge(t *testing.T) *hiddenKnowledge {
	t.Helper()
	tb := newHiddenNamesTable(t)
	m := tb.master
	class := m.addEntry(t, tb.campaign, testClass(forgottenClass)).GetKey()
	pact := m.addEntry(t, tb.campaign, &rulesv1.TableOption{NamePt: forgottenPact, Set: "pact_boon", DescPt: []string{"Texto."}}).GetKey()
	spell := m.addEntry(t, tb.campaign, testSpell(forgottenSpell, "class:wizard")).GetKey()
	style := m.addEntry(t, tb.campaign, &rulesv1.TableOption{NamePt: forgottenOption, Set: "fighting_style", DescPt: []string{"Texto."}}).GetKey()
	feat := func(name string, p *rulesv1.FeatPrerequisite) string {
		return m.addEntry(t, tb.campaign, &rulesv1.TableFeat{NamePt: name, DescPt: []string{"Texto."}, Prerequisite: p}).GetKey()
	}
	hk := &hiddenKnowledge{hiddenNamesTable: tb, class: class, pact: pact, spell: spell, style: style}
	hk.byClass = feat("Pede a ordem", &rulesv1.FeatPrerequisite{ClassLevel: &rulesv1.FeatClassLevel{ClassKey: class, Level: 3}})
	hk.byPact = feat("Pede o pacto", &rulesv1.FeatPrerequisite{RequiresPact: pact})
	hk.bySpell = feat("Pede a magia", &rulesv1.FeatPrerequisite{RequiresSpellKey: spell})
	hk.byOption = feat("Pede o estilo", &rulesv1.FeatPrerequisite{RequiresOptionKeys: []string{style}})
	m.setOff(t, tb.campaign, true, class, pact, spell, style)
	return hk
}

func TestAPlayersLevelUpNamesNoHiddenClassPactSpellOrOption(t *testing.T) {
	t.Parallel()
	hk := newHiddenKnowledge(t)
	tb, m := hk.hiddenNamesTable, hk.master
	class, pact, spell, style := hk.class, hk.pact, hk.spell, hk.style
	byClass, byPact, bySpell, byOption := hk.byClass, hk.byPact, hk.bySpell, hk.byOption

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
		if got.GetUnmet()[0].GetValue() != 0 {
			t.Errorf("%s: the hidden condition still sends the number %d", key, got.GetUnmet()[0].GetValue())
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

// The refusal a player gets for a feat that asks for a hidden entry names no hidden key and no hidden name.
func TestAPlayersLevelUpRefusalNamesNoHiddenEntry(t *testing.T) {
	t.Parallel()
	hk := newHiddenKnowledge(t)
	for name, feat := range map[string]string{"class": hk.byClass, "pact": hk.byPact, "spell": hk.bySpell, "option": hk.byOption} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ch := pensantusLevelUp()
			ch.FeatKey, ch.AbilityIncrease = feat, nil
			_, err := hk.levelUp(hk.owner, hk.pc, ch)
			if err == nil {
				t.Fatal("a player took a feat whose prerequisite they do not meet")
			}
			for _, w := range []string{forgottenClass, forgottenPact, forgottenSpell, forgottenOption, hk.class, hk.pact, hk.spell, hk.style} {
				if strings.Contains(err.Error(), w) {
					t.Errorf("the refusal names %q: %v", w, err)
				}
			}
		})
	}
}

// The sheet's issue of a lost option prerequisite reaches the owner through GetCharacter, and a condition on an entry
// the player may not read names nothing in it.
func TestTheOwnerReadsTheLostOptionPrerequisiteIssueWithoutHiddenNames(t *testing.T) {
	t.Parallel()
	hk := newHiddenKnowledge(t)
	m := hk.master
	feat := m.addEntry(t, hk.campaign, &rulesv1.TableFeat{
		NamePt: "Posturas do vale", DescPt: []string{"Texto."},
		Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "feature", Count: 1}},
		Options: []*rulesv1.TableFeature{{NamePt: "Postura exigente", DescPt: []string{"Texto."}, Prerequisite: &rulesv1.OptionPrerequisite{
			ClassLevel: &rulesv1.OptionClassLevel{ClassKey: hk.class, Level: 3},
		}}},
	})
	option := feat.GetTableFeat().GetOptions()[0].GetKey()
	current := m.get(t, hk.campaign, hk.pc.GetId())
	sheet := proto.CloneOf(current.GetSheet())
	sheet.GetFull().FeatKeys = []string{feat.GetKey()}
	sheet.GetFull().FeatureChoiceKeys = []string{option}
	if _, err := m.update(t, current, current.GetName(), sheet); err != nil {
		t.Fatalf("the master's save of a sheet whose option asks for a class it lacks: %v", err)
	}
	owned := hk.owner.get(t, hk.campaign, hk.pc.GetId())
	var message string
	for _, i := range owned.GetDerived().GetIssues() {
		if i.GetCode() == "option_prerequisite" {
			message = i.GetMessage()
		}
	}
	if want := "O personagem não cumpre mais o pré-requisito de Postura exigente: pede uma condição que a mesa não mostra."; message != want {
		t.Errorf("the owner's issue = %q, want %q", message, want)
	}
	wantNames(t, "the owner's sheet", owned, false, forgottenClass, hk.class)
}
