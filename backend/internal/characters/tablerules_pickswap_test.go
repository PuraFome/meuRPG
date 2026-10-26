package characters

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// The table rule "Trocar uma escolha no aumento de atributo" (MR-025): at an Ability Score
// Improvement level the level-up also swaps one earlier pick of a group. Pensantus (Wizard 3)
// reaches level 4, the level of an Ability Score Improvement and a new cantrip.

func swapGroupOf(o *charactersv1.LevelUpOptions, group string) *charactersv1.LevelUpSwapGroup {
	for _, g := range o.GetSwapGroups() {
		if g.GetGroup() == group {
			return g
		}
	}
	return nil
}

func TestMR025_ALevelUpSwapsACantripOnlyWhereTheTableUsesTheRule(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2710)
	o, err := tb.options(tb.owner, tb.pc)
	if err != nil || len(o.GetSwapGroups()) != 0 {
		t.Fatalf("options with the rule off: %d groups, %v; want the level-up as it was", len(o.GetSwapGroups()), err)
	}
	ch := pensantusLevelUp()
	ch.SwapPick = &charactersv1.LevelUpPickSwap{Group: "cantrip", FromKey: "spell:light", ToKey: "spell:prestidigitation"}
	_, err = tb.levelUp(tb.owner, tb.pc, ch)
	if r := refusal(t, "LevelUpCharacter", err); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_SWAP_NOT_ALLOWED || r.GetField() != "choices.swap_pick" {
		t.Errorf("a swap where the rule is off: refusal = %v", r)
	}

	setRules(t, tb.master, tb.campaign, func(r *campaignsv1.TableRules) { r.PickSwapAllowed = true })
	got, _ := tb.master.campaigns.GetTableRules(t.Context(), connect.NewRequest(&campaignsv1.GetTableRulesRequest{CampaignId: tb.campaign}))
	if !got.Msg.GetRules().GetPickSwapAllowed() {
		t.Fatal("the rule was not saved")
	}
	o, err = tb.options(tb.owner, tb.pc)
	if err != nil {
		t.Fatal(err)
	}
	g := swapGroupOf(o, "cantrip")
	if g == nil || len(g.GetPicks()) == 0 || len(g.GetPool()) == 0 {
		t.Fatalf("options with the rule on: %v, want the cantrip group", o.GetSwapGroups())
	}
	from := g.GetPicks()[0].GetKey()
	pool := slices.IndexFunc(g.GetPool(), func(k *charactersv1.LevelUpNamedKey) bool { return k.GetKey() != "spell:mage-hand" })
	to := g.GetPool()[pool].GetKey()

	ch.SwapPick = &charactersv1.LevelUpPickSwap{Group: "cantrip", FromKey: from, ToKey: to}
	res, err := tb.levelUp(tb.owner, tb.pc, ch)
	if err != nil {
		t.Fatalf("a swap of %s for %s: %v", from, to, err)
	}
	if cantrips := res.GetSheet().GetFull().GetCantripKeys(); slices.Contains(cantrips, from) || !slices.Contains(cantrips, to) || !slices.Contains(cantrips, "spell:mage-hand") {
		t.Errorf("cantrips after the swap = %v", cantrips)
	}
}

func TestMR025_ASwapNeedsAPickAndAPoolOptionOfTheGroup(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2711)
	setRules(t, tb.master, tb.campaign, func(r *campaignsv1.TableRules) { r.PickSwapAllowed = true })
	for name, sw := range map[string]*charactersv1.LevelUpPickSwap{
		"a pick the sheet does not have": {Group: "cantrip", FromKey: "spell:fire-bolt-nope", ToKey: "spell:prestidigitation"},
		"a group nothing offers":         {Group: "fighting_style", FromKey: "feature:fighter-fighting-style-defense", ToKey: "feature:fighter-fighting-style-dueling"},
	} {
		ch := pensantusLevelUp()
		ch.SwapPick = sw
		_, err := tb.levelUp(tb.owner, tb.pc, ch)
		if r := refusal(t, "LevelUpCharacter", err); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_SWAP_NOT_ALLOWED {
			t.Errorf("%s: refusal = %v", name, r)
		}
	}
}
