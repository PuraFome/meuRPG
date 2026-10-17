package play

import (
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// What the master and each player read of the effects of a combat: the table of "Efeitos em jogo",
// the cards on the combatants, the labels the others read and the conditions a viewer may read
// (RN-10, RN-20). The scenario puts effects of every source, duration and visibility on the close
// fight and writes what each viewer reads of it.

const (
	rpcDurRounds    = playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS
	rpcDurStartTurn = playv1.EffectDurationKind_EFFECT_DURATION_KIND_UNTIL_START_OF_TURN_OF
	rpcDurEndTurn   = playv1.EffectDurationKind_EFFECT_DURATION_KIND_UNTIL_END_OF_TURN_OF
	rpcDurConc      = playv1.EffectDurationKind_EFFECT_DURATION_KIND_CONCENTRATION
	rpcDurLongRest  = playv1.EffectDurationKind_EFFECT_DURATION_KIND_LONG_REST
	rpcDurDismissed = playv1.EffectDurationKind_EFFECT_DURATION_KIND_UNTIL_DISMISSED
)

// rpcVisibleLabel makes the effect visible to the players with a label and an audience.
func rpcVisibleLabel(visible bool, label string, audience playv1.EffectAudience) rpcAddOpts {
	return func(r *playv1.AddLastingEffectRequest) {
		r.PlayerVisible, r.PlayerLabel, r.Audience = &visible, label, audience
	}
}

// rpcAllEffects puts one effect of every kind on the close fight and returns the encounter.
func (a *armed) rpcAllEffects(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := a.closeFight(t)
	// Conditions the master marks by hand come first: the effects add theirs.
	if _, err := a.conditions(t, a.master, e, "Toren", []string{"condition:frightened"}, true, false); err != nil {
		t.Fatalf("SetCombatantConditions(Toren) error = %v", err)
	}
	if _, err := a.conditions(t, a.master, e, "Goblin", []string{"condition:blinded", "condition:charmed"}, true, false); err != nil {
		t.Fatalf("SetCombatantConditions(Goblin) error = %v", err)
	}
	// A spell of the caster Pensantus by concentration, for rounds.
	a.rpcAdd(t, e, blessKey, []string{"Toren"}, a.rpcCaster(t, "Pensantus"), a.rpcDuration(t, rpcDurRounds, 10, "Pensantus"))
	// Hold Person on the goblin: a saving throw at the end of its turns, a DC and a caster the players see.
	a.rpcAdd(t, e, holdPerson, []string{"Goblin"}, a.rpcCaster(t, "Brisa"), func(r *playv1.AddLastingEffectRequest) { r.SaveDc = new(int32(14)) })
	// A spell with a free label, hidden from the players.
	a.rpcAdd(t, e, "spell:bane", []string{"Goblin"}, a.rpcCaster(t, "Goblin"), rpcVisibleLabel(false, "Segredo", playv1.EffectAudience_EFFECT_AUDIENCE_ALL), func(r *playv1.AddLastingEffectRequest) { r.Concentration = new(false) })
	// A condition of the master, with the owner alone reading it.
	a.rpcAdd(t, e, "condition:poisoned", []string{"Toren"})
	// A condition for everyone, with a label of the master's.
	a.rpcAdd(t, e, "condition:prone", []string{"Brisa"}, rpcVisibleLabel(true, "No chão", playv1.EffectAudience_EFFECT_AUDIENCE_ALL))
	// A condition that only the owner reads, though the master made it visible.
	a.rpcAdd(t, e, "condition:deafened", []string{"Pensantus"}, rpcVisibleLabel(true, "", playv1.EffectAudience_EFFECT_AUDIENCE_OWNER))
	// An effect of the master's catalog, with no caster.
	a.rpcAdd(t, e, "effect:web-fire", []string{"Toren"})
	// The clocks that end at a turn, by concentration, at a long rest and never.
	a.rpcAdd(t, e, guidanceKey, []string{"Toren"}, a.rpcDuration(t, rpcDurStartTurn, 0, "Pensantus"))
	a.rpcAdd(t, e, resistanceKey, []string{"Brisa"}, a.rpcDuration(t, rpcDurEndTurn, 0, "Goblin"))
	a.rpcAdd(t, e, longstriderKey, []string{"Pensantus"}, a.rpcDuration(t, rpcDurLongRest, 0, ""))
	a.rpcAdd(t, e, heroismKey, []string{"Brisa"}, a.rpcCaster(t, "Toren"), a.rpcDuration(t, rpcDurConc, 0, ""))
	a.rpcAdd(t, e, longstriderKey, []string{"Toren"}, a.rpcCaster(t, "Goblin"), a.rpcDuration(t, rpcDurDismissed, 0, ""))
	a.rpcAdd(t, e, enhanceAbilityKey, []string{"Toren"}, func(r *playv1.AddLastingEffectRequest) { r.AbilityKey = "con" })
	a.rpcAdd(t, e, "spell:hideous-laughter", []string{"Goblin"}, a.rpcCaster(t, "Goblin"))
	return a.get(t, a.master)
}

// TestLastingRPCViewsOfEveryViewer freezes what each viewer reads of the effects on the combatants.
func TestLastingRPCViewsOfEveryViewer(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	a.rpcAllEffects(t)
	for name, u := range map[string]*user{"master": a.master, "owner": a.caio, "other": a.ana, "bystander": a.bia} {
		wantLastingGolden(t, "lasting_rpc", "view_"+name, a.rpcView(t, u))
	}
	// The goblin hides: its spells have a caster the players do not see.
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: a.get(t, a.master).GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(), Hidden: true,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	for name, u := range map[string]*user{"owner": a.caio, "other": a.ana} {
		wantLastingGolden(t, "lasting_rpc", "view_goblin_hidden_"+name, a.rpcView(t, u))
	}
}

// TestLastingRPCListsTheTableAndTheClock freezes the master's table: the cards, the loose
// conditions, the clock of the next turns, the concentrations and the catalog.
func TestLastingRPCListsTheTableAndTheClock(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.rpcAllEffects(t)
	list := func(mutate func(*playv1.ListLastingEffectsRequest)) *playv1.ListLastingEffectsResponse {
		req := &playv1.ListLastingEffectsRequest{CampaignId: a.campaignID, EncounterId: e.GetId()}
		if mutate != nil {
			mutate(req)
		}
		res, err := a.master.lasting.ListLastingEffects(t.Context(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("ListLastingEffects() error = %v", err)
		}
		return res.Msg
	}
	res := list(nil)
	catalog := res.Catalog
	res.Catalog = nil
	wantLastingGolden(t, "lasting_rpc", "list_table", a.rpcJSON(t, res))
	var lines []string
	for _, c := range catalog {
		lines = append(lines, strings.Join([]string{
			c.GetKey(), c.GetNamePt(), c.GetSourceKind().String(), c.GetDefaultDurationKind().String(),
			fmt.Sprint(c.GetDefaultRounds()), rpcYes(c.GetConcentration()), rpcYes(c.GetHasCaster()),
		}, " | "))
	}
	wantLastingGolden(t, "lasting_rpc", "list_catalog", strings.Join(lines, "\n")+"\n")
	// The same table later in the fight: the round, the turn and the clock move.
	a.turnOf(t, "Toren")
	a.turnOf(t, "Pensantus")
	res = list(nil)
	res.Catalog = nil
	wantLastingGolden(t, "lasting_rpc", "list_table_round_two", a.rpcJSON(t, res))
	// Without a combat only the catalog comes.
	bare := list(func(r *playv1.ListLastingEffectsRequest) { r.EncounterId = "" })
	if len(bare.GetEffects()) != 0 || bare.GetRound() != 0 || len(bare.GetCatalog()) != len(catalog) {
		t.Errorf("the list with no combat = %d effects, round %d, %d catalog entries; want only the %d of the catalog", len(bare.GetEffects()), bare.GetRound(), len(bare.GetCatalog()), len(catalog))
	}
	// The refusals of the call.
	_, err := a.master.lasting.ListLastingEffects(t.Context(), connect.NewRequest(&playv1.ListLastingEffectsRequest{CampaignId: a.campaignID, EncounterId: "not-an-id"}))
	wantRefusal(t, "list with a bad encounter id", err, connect.CodeNotFound, "encounter not found")
	_, err = a.master.lasting.ListLastingEffects(t.Context(), connect.NewRequest(&playv1.ListLastingEffectsRequest{CampaignId: a.campaignID, EncounterId: newKey()}))
	wantCode(t, "list of a combat that is not there", err, connect.CodeNotFound)
	_, err = a.ana.lasting.ListLastingEffects(t.Context(), connect.NewRequest(&playv1.ListLastingEffectsRequest{CampaignId: a.campaignID}))
	wantCode(t, "a player lists the catalog", err, connect.CodePermissionDenied)
}

func rpcYes(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
