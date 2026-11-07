package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A granted_spell with a cost of more than one use and no material components (MR-025). The
// feat is the master's and its names are invented; the spell is the SRD's. These tests need the
// database (MEURPG_TEST_DATABASE_URL).

// newCostParty is newCasters with a feat that grants Bless for two of three uses of a resource,
// with no material components.
func newCostParty(t *testing.T) (a *armed, feat string) {
	t.Helper()
	return newGracePartyWith(t, []*rulesv1.TableEffect{
		{Type: "resource", Resource: graceUses, Max: "3", Recharge: "long_rest"},
		{Type: "granted_spell", Spell: blessKey, Ability: "cha", Uses: graceUses, UsesCost: 2, NoMaterialComponents: true},
	})
}

// The cast spends the cost, a cast the uses left do not cover is refused with nothing spent, and
// the listing says the cost and the waived material.
func TestMR025_AGrantedSpellSpendsItsCostOutsideACombat(t *testing.T) {
	t.Parallel()
	a, feat := newCostParty(t)

	res, err := a.ana.casting.GetCastOptions(t.Context(), connect.NewRequest(&playv1.GetCastOptionsRequest{CampaignId: a.campaignID, CharacterId: a.pens.GetId()}))
	if err != nil {
		t.Fatalf("GetCastOptions() error = %v", err)
	}
	var g *playv1.CastingGrant
	for _, s := range res.Msg.GetSpells() {
		if s.GetSpell().GetKey() == blessKey && len(s.GetGrants()) == 1 {
			g = s.GetGrants()[0]
		}
	}
	if g == nil || g.GetUsesCost() != 2 || g.GetUsesLeft() != 3 || !g.GetReady() || !g.GetNoMaterialComponents() {
		t.Fatalf("grant = %+v, want cost 2, 3 uses left, ready, no material components", g)
	}

	if _, err := a.castOut(t, a.ana, a.pens, blessKey, nil, false, []*charactersv1.Character{a.pens}, grantedBy(feat)); err != nil {
		t.Fatalf("CastSpellOutsideCombat(bless through the grant) error = %v", err)
	}
	if used, total := resourceUsed(a.vitals(t, a.pens), graceUses); used != 2 || total != 3 {
		t.Errorf("grant uses = %d of %d, want 2 of 3", used, total)
	}
	// One use is left and the cast costs two.
	_, err = a.castOut(t, a.ana, a.pens, blessKey, nil, false, []*charactersv1.Character{a.pens}, grantedBy(feat))
	wantCastingBlocked(t, "a cast the uses left do not cover", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_NO_USES)
	if used, _ := resourceUsed(a.vitals(t, a.pens), graceUses); used != 2 {
		t.Errorf("a refused cast spent uses: %d, want 2", used)
	}
}

// In a combat, the option says the cost; the cast spends it and the undo gives all of it back.
func TestMR025_AGrantedSpellCostIsSpentAndGivenBackInCombat(t *testing.T) {
	t.Parallel()
	a, feat := newCostParty(t)
	e := a.castersFight(t, 1)
	granted := func(r *playv1.CastSpellRequest) { r.GrantedBy = feat; noCastRoll(r) }

	a.undoes(t, "the costly grant", func() {
		if _, err := a.cast(t, a.ana, e, "Pensantus", blessKey, nil, a.at(t, "Pensantus", "Toren"), granted); err != nil {
			t.Fatalf("CastSpell(bless through the grant) error = %v", err)
		}
		if used, _ := resourceUsed(a.vitals(t, a.pens), graceUses); used != 2 {
			t.Errorf("grant uses after the cast = %d, want 2", used)
		}
	})
	if used, _ := resourceUsed(a.vitals(t, a.pens), graceUses); used != 0 {
		t.Errorf("grant uses after the undo = %d, want 0", used)
	}
}
