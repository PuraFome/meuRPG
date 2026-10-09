package characters

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The acceptance criteria of MR-025 for feats at the guided level-up: an optional rule of the
// game that the table's rule "Talentos" turns on. Pensantus (Wizard 3, Intelligence 18) is
// the character; level 4 has an Ability Score Improvement.

// featsTable is a level-up table whose master wrote three feats: a half feat (+1 Intelligence
// or Wisdom, asks Intelligence 13), a heavy one (asks Strength 15) and a plain one.
type featsTable struct {
	*levelUpTable
	half, heavy, plain *rulesv1.TableEntry
}

func newFeatsTable(t *testing.T) *featsTable {
	t.Helper()
	ft := &featsTable{levelUpTable: newLevelUpTable(t, 2700)}
	m := ft.master
	ft.half = m.addEntry(t, ft.campaign, &rulesv1.TableFeat{
		NamePt: "Mente afiada", DescPt: []string{"Pensa depressa."},
		Prerequisite: &rulesv1.FeatPrerequisite{Minimums: &rulesv1.AbilityScores{Intelligence: 13}},
		Effects: []*rulesv1.TableEffect{
			{Type: "modifier", Target: "initiative", Mode: "add", Value: "1"},
			{Type: "ability_increase", Count: 1, From: []string{"int", "wis"}, Value: "1"},
		},
	})
	ft.heavy = m.addEntry(t, ft.campaign, &rulesv1.TableFeat{
		NamePt: "Brutamontes", DescPt: []string{"Força bruta."}, Prerequisite: &rulesv1.FeatPrerequisite{Minimums: &rulesv1.AbilityScores{Strength: 15}},
	})
	ft.plain = m.addEntry(t, ft.campaign, &rulesv1.TableFeat{NamePt: "Sortudo", DescPt: []string{"Sorte."}})
	return ft
}

func (ft *featsTable) allowFeats(t *testing.T, allowed bool) {
	t.Helper()
	setRules(t, ft.master, ft.campaign, func(r *campaignsv1.TableRules) { r.FeatsAllowed = allowed })
}

func (ft *featsTable) featChoice(feat string, up *rulesv1.AbilityScores) *charactersv1.LevelUpChoices {
	ch := pensantusLevelUp()
	// A feat replaces the +2: Intelligence stays at 18 (or 19), so Pensantus prepares 8, not 9.
	ch.AbilityIncrease, ch.FeatKey = up, feat
	ch.PreparedSpellKeys = []string{"spell:misty-step"}
	return ch
}

func optionOf(o *charactersv1.LevelUpOptions, key string) *rulesv1.FeatOption {
	for _, f := range o.GetFeats() {
		if f.GetKey() == key {
			return f
		}
	}
	return nil
}

// TestMR025_TheTableRuleTalentosDefaultsToNotUsed: the rule is saved with the others, off
// until the master turns it on; with it off the level-up offers no feat and refuses one.
func TestMR025_TheTableRuleTalentosDefaultsToNotUsed(t *testing.T) {
	t.Parallel()
	ft := newFeatsTable(t)
	got, err := ft.master.campaigns.GetTableRules(t.Context(), connect.NewRequest(&campaignsv1.GetTableRulesRequest{CampaignId: ft.campaign}))
	if err != nil || got.Msg.GetRules().GetFeatsAllowed() {
		t.Fatalf("GetTableRules() = %v, %v; want feats not used by default", got.Msg.GetRules(), err)
	}
	o, err := ft.options(ft.owner, ft.pc)
	if err != nil || !o.GetAbilityScoreImprovement() || len(o.GetFeats()) != 0 {
		t.Fatalf("options with feats not used: ASI %v, %d feats, %v; want the level-up as it was", o.GetAbilityScoreImprovement(), len(o.GetFeats()), err)
	}
	_, err = ft.levelUp(ft.owner, ft.pc, ft.featChoice(ft.plain.GetKey(), nil))
	if r := refusal(t, "LevelUpCharacter", err); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_FEATS_NOT_ALLOWED || r.GetField() != "choices.feat_key" {
		t.Errorf("a feat where feats are not used: refusal = %v", r)
	}
	// Turning it on is saved and read back, and turning it off again too.
	ft.allowFeats(t, true)
	got, _ = ft.master.campaigns.GetTableRules(t.Context(), connect.NewRequest(&campaignsv1.GetTableRulesRequest{CampaignId: ft.campaign}))
	if !got.Msg.GetRules().GetFeatsAllowed() {
		t.Error("the rule Talentos was not saved")
	}
	ft.allowFeats(t, false)
	got, _ = ft.master.campaigns.GetTableRules(t.Context(), connect.NewRequest(&campaignsv1.GetTableRulesRequest{CampaignId: ft.campaign}))
	if got.Msg.GetRules().GetFeatsAllowed() {
		t.Error("the rule Talentos stayed on")
	}
}

// TestMR025_TheLevelUpOffersTheFeatsTheCharacterQualifiesFor: at the Ability Score Improvement,
// with the rule on, the options list every feat with whether Pensantus qualifies and what he
// lacks, the SRD's Grappler included; a retired feat is not offered.
func TestMR025_TheLevelUpOffersTheFeatsTheCharacterQualifiesFor(t *testing.T) {
	t.Parallel()
	ft := newFeatsTable(t)
	ft.allowFeats(t, true)
	retired := ft.master.addEntry(t, ft.campaign, &rulesv1.TableFeat{NamePt: "Aposentado", DescPt: []string{"x"}})
	if _, err := ft.master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: ft.campaign, Key: retired.GetKey()})); err != nil {
		t.Fatal(err)
	}

	o, err := ft.options(ft.owner, ft.pc)
	if err != nil {
		t.Fatal(err)
	}
	half, heavy, grappler := optionOf(o, ft.half.GetKey()), optionOf(o, ft.heavy.GetKey()), optionOf(o, "feat:grappler")
	if half == nil || heavy == nil || grappler == nil || optionOf(o, ft.plain.GetKey()) == nil {
		t.Fatalf("the options lack a feat: %v", o.GetFeats())
	}
	if optionOf(o, retired.GetKey()) != nil {
		t.Error("a retired feat is offered as a new choice")
	}
	if !half.GetQualifies() || half.GetIncrease().GetCount() != 1 || half.GetIncrease().GetValue() != 1 ||
		!slices.Equal(half.GetIncrease().GetFrom(), []rulesv1.Ability{rulesv1.Ability_ABILITY_INTELLIGENCE, rulesv1.Ability_ABILITY_WISDOM}) ||
		half.GetPrerequisite().GetMinimums().GetIntelligence() != 13 || !half.GetTable() || half.GetNamePt() != "Mente afiada" {
		t.Errorf("the half feat = %v", half)
	}
	if heavy.GetQualifies() || len(heavy.GetUnmet()) != 1 || heavy.GetUnmet()[0].GetKind() != rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_ABILITY_MINIMUM ||
		heavy.GetUnmet()[0].GetAbilities()[0].GetAbility() != rulesv1.Ability_ABILITY_STRENGTH || heavy.GetUnmet()[0].GetAbilities()[0].GetMinimum() != 15 {
		t.Errorf("the heavy feat = %v, want it not met for Strength 15", heavy)
	}
	if grappler.GetQualifies() || grappler.GetNamePt() != "Agarrador" || grappler.GetTable() || grappler.GetPrerequisite().GetMinimums().GetStrength() != 13 {
		t.Errorf("Grappler = %v, want the SRD's feat, not met by Strength 12", grappler)
	}
	// The master reads the same list.
	if mo, err := ft.options(ft.master, ft.pc); err != nil || len(mo.GetFeats()) != len(o.GetFeats()) {
		t.Errorf("the master's options: %d feats, %v; want the player's %d", len(mo.GetFeats()), err, len(o.GetFeats()))
	}
}

// TestMR025_ALevelUpTakesAFeatInPlaceOfTheIncrease: Pensantus takes the half feat: the feat is on
// the sheet and the sheet lists it with its effects, the feat's +1 is the only increase, the level
// is in the history, and the player can read it back.
func TestMR025_ALevelUpTakesAFeatInPlaceOfTheIncrease(t *testing.T) {
	t.Parallel()
	ft := newFeatsTable(t)
	ft.allowFeats(t, true)
	before := ft.pc.GetDerived().GetInitiative()

	// +2 is the ordinary increase, not the feat's: refused with the feat.
	_, err := ft.levelUp(ft.owner, ft.pc, ft.featChoice(ft.half.GetKey(), &rulesv1.AbilityScores{Intelligence: 2}))
	if r := refusal(t, "LevelUpCharacter", err); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_ABILITY_SHAPE {
		t.Errorf("+2 with a feat of +1: refusal = %v", r)
	}
	// A preview shows what the feat makes, without a write.
	p, err := ft.preview(ft.owner, ft.pc, ft.featChoice(ft.half.GetKey(), &rulesv1.AbilityScores{Wisdom: 1}))
	if err != nil || p.GetRefusal() != nil || p.GetAfter().GetInitiative() != before+1 {
		t.Fatalf("preview: initiative %d, refusal %v, error %v; want %d and no refusal", p.GetAfter().GetInitiative(), p.GetRefusal(), err, before+1)
	}

	up, err := ft.levelUp(ft.owner, ft.pc, ft.featChoice(ft.half.GetKey(), &rulesv1.AbilityScores{Wisdom: 1}))
	if err != nil {
		t.Fatalf("LevelUpCharacter() error = %v", err)
	}
	full := up.GetSheet().GetFull()
	if !slices.Equal(full.GetFeatKeys(), []string{ft.half.GetKey()}) || full.GetExtraAbilityBonuses().GetWisdom() != 1 || full.GetExtraAbilityBonuses().GetIntelligence() != 0 {
		t.Errorf("sheet after = feats %v, bonuses %v; want the feat and +1 Wisdom only", full.GetFeatKeys(), full.GetExtraAbilityBonuses())
	}
	d := up.GetDerived()
	if d.GetInitiative() != before+1 {
		t.Errorf("initiative = %d, want %d: the feat's effect applies", d.GetInitiative(), before+1)
	}
	listed := false
	for _, f := range d.GetFeatures() {
		if f.GetKey() == ft.half.GetKey() {
			listed = f.GetSourcePt() == "Talento · Mago 4" && f.GetNamePt() == "Mente afiada"
		}
	}
	if !listed {
		t.Errorf("the sheet does not list the feat among the features: %v", d.GetFeatures())
	}
	for _, f := range d.GetFeatures() {
		if strings.Contains(f.GetKey(), "ability-score-improvement") {
			t.Errorf("the sheet lists %s as taken although the feat replaced it", f.GetKey())
		}
	}
	history, err := ft.levelUps(ft.master, ft.pc.GetId())
	if err != nil || len(history) != 1 || history[0].GetChoices().GetFeatKey() != ft.half.GetKey() || history[0].GetNamesPt()[ft.half.GetKey()] != "Mente afiada" {
		t.Errorf("history = %v, %v; want the feat recorded and named", history, err)
	}
	// The feat is not offered twice at a later ASI level, and the character is told which feat.
	if got := ft.owner.get(t, ft.campaign, ft.pc.GetId()); !slices.Equal(got.GetSheet().GetFull().GetFeatKeys(), []string{ft.half.GetKey()}) {
		t.Errorf("the owner reads feats %v", got.GetSheet().GetFull().GetFeatKeys())
	}
}

// TestMR025_ALevelUpRefusesAFeatTheCharacterDoesNotQualifyFor: the server checks the
// prerequisite (the app only shows the reason), the ability cap of 20, the key, and the
// switches of the master.
func TestMR025_ALevelUpRefusesAFeatTheCharacterDoesNotQualifyFor(t *testing.T) {
	t.Parallel()
	ft := newFeatsTable(t)
	ft.allowFeats(t, true)
	reasons := func(key string, up *rulesv1.AbilityScores, who *user) *charactersv1.LevelUpRefusal {
		t.Helper()
		_, err := ft.levelUp(who, ft.pc, ft.featChoice(key, up))
		return refusal(t, "LevelUpCharacter", err)
	}
	if r := reasons(ft.heavy.GetKey(), nil, ft.owner); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_FEAT_PREREQUISITE || r.GetField() != "full.feat_keys" {
		t.Errorf("Strength 15 asked, Strength 12 had: refusal = %v", r)
	}
	if r := reasons("feat:grappler", nil, ft.owner); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_FEAT_PREREQUISITE {
		t.Errorf("Grappler without Strength 13: refusal = %v", r)
	}
	if _, err := ft.levelUp(ft.owner, ft.pc, ft.featChoice("feat:nao-existe", nil)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a feat that does not exist: error = %v, want invalid_argument", err)
	}
	// A feat gives its increase and no other.
	if r := reasons(ft.plain.GetKey(), &rulesv1.AbilityScores{Intelligence: 1}, ft.owner); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_ABILITY_SHAPE {
		t.Errorf("a feat with no increase and a raised ability: refusal = %v", r)
	}
	if r := reasons(ft.half.GetKey(), &rulesv1.AbilityScores{Charisma: 1}, ft.owner); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_ABILITY_SHAPE {
		t.Errorf("the half feat raising an ability it does not list: refusal = %v", r)
	}

	// An ability never goes above 20: with Intelligence at 20 the feat's +1 on it is refused,
	// and the other ability it lists is still fine.
	sheet := proto.Clone(ft.pc.GetSheet()).(*charactersv1.CharacterSheet)
	sheet.GetFull().ExtraAbilityBonuses = &rulesv1.AbilityScores{Intelligence: 2}
	updated, err := ft.master.update(t, ft.pc, ft.pc.GetName(), sheet)
	if err != nil {
		t.Fatalf("the master edits the locked sheet: %v", err)
	}
	ft.pc = updated
	if r := reasons(ft.half.GetKey(), &rulesv1.AbilityScores{Intelligence: 1}, ft.owner); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_ABILITY_ABOVE_20 {
		t.Errorf("+1 on Intelligence 20: refusal = %v, want ability_above_20", r)
	}
	if _, err := ft.levelUp(ft.owner, ft.pc, ft.featChoice(ft.half.GetKey(), &rulesv1.AbilityScores{Wisdom: 1})); err != nil {
		t.Errorf("+1 on Wisdom 13 with Intelligence at 20: error = %v", err)
	}
}

// TestMR025_ASwitchedOffOrRetiredFeatIsNeverAPlayersChoice: RN-23. The feat the master switched off
// is not offered to the player, nor accepted from them, but the master sees it; a retired one is
// not a new choice for anyone. The text of such a feat never reaches the player (the positive
// control: the live feat's text does).
func TestMR025_ASwitchedOffOrRetiredFeatIsNeverAPlayersChoice(t *testing.T) {
	t.Parallel()
	ft := newFeatsTable(t)
	ft.allowFeats(t, true)
	const live, off, retired = "LEAKCANARY-feat-live-1", "LEAKCANARY-feat-off-1", "LEAKCANARY-feat-retired-1"
	mk := func(name, marker string) *rulesv1.TableEntry {
		return ft.master.addEntry(t, ft.campaign, &rulesv1.TableFeat{NamePt: name, DescPt: []string{marker}})
	}
	liveFeat, offFeat, retiredFeat := mk("Visível", live), mk("Escondido", off), mk("Velho", retired)
	if _, err := ft.master.table.SetOptionSwitches(t.Context(), connect.NewRequest(&rulesv1.SetOptionSwitchesRequest{
		CampaignId: ft.campaign, Switches: []*rulesv1.OptionSwitch{{Key: offFeat.GetKey(), Off: true}},
	})); err != nil {
		t.Fatal(err)
	}
	if _, err := ft.master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: ft.campaign, Key: retiredFeat.GetKey()})); err != nil {
		t.Fatal(err)
	}

	po, err := ft.options(ft.owner, ft.pc)
	if err != nil {
		t.Fatal(err)
	}
	text := po.String()
	if !strings.Contains(text, live) {
		t.Fatalf("the player does not read the live feat: %s", text) // the positive control
	}
	if strings.Contains(text, off) || strings.Contains(text, retired) || strings.Contains(text, "Escondido") || strings.Contains(text, "Velho") || optionOf(po, offFeat.GetKey()) != nil {
		t.Errorf("the player's options carry a feat the master hides: %s", text)
	}
	mo, _ := ft.options(ft.master, ft.pc)
	if optionOf(mo, offFeat.GetKey()) == nil || optionOf(mo, retiredFeat.GetKey()) != nil {
		t.Errorf("the master's options: off feat %v, retired feat %v; want the off one and not the retired one", optionOf(mo, offFeat.GetKey()), optionOf(mo, retiredFeat.GetKey()))
	}
	for _, key := range []string{offFeat.GetKey(), retiredFeat.GetKey()} {
		_, err := ft.levelUp(ft.owner, ft.pc, ft.featChoice(key, nil))
		r := refusal(t, "LevelUpCharacter", err)
		if r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_SWITCHED_OFF_CHOICE && r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_ARCHIVED_CHOICE {
			t.Errorf("%s: refusal = %v", key, r)
		}
	}
	if p, err := ft.preview(ft.owner, ft.pc, ft.featChoice(offFeat.GetKey(), nil)); err != nil || strings.Contains(p.String(), off) {
		t.Errorf("a preview with a hidden feat showed it: %v, %v", p, err)
	}
	if _, err := ft.levelUp(ft.owner, ft.pc, ft.featChoice(liveFeat.GetKey(), nil)); err != nil {
		t.Errorf("the live feat: error = %v", err)
	}
}
