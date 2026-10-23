package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// An effect belongs to the character it is on: it moves into a combat with the time it has left
// and out of it when the combat ends or the character leaves (RN-22). The record keeps its id.

// rpcInsertCharacterEffect writes a row of character_effects by hand: the cases a call cannot make.
func (a *armed) rpcInsertCharacterEffect(t *testing.T, who *charactersv1.Character, key, kind, duration, seconds, caster string) {
	t.Helper()
	a.execSQL(t, `INSERT INTO character_effects (campaign_id, character_id, group_id, source_key, source_kind, duration_kind, seconds_left, source_character_id)
		VALUES ($1, $2, gen_random_uuid(), $3, $4, $5, NULLIF($6, '')::INT, NULLIF($7, '')::UUID)`, a.campaignID, who.GetId(), key, kind, duration, seconds, caster)
}

// rpcMasterTable is the master's table of the effects, without the catalog.
func (a *armed) rpcMasterTable(t *testing.T, e *playv1.Encounter) string {
	t.Helper()
	res, err := a.master.lasting.ListLastingEffects(t.Context(), connect.NewRequest(&playv1.ListLastingEffectsRequest{CampaignId: a.campaignID, EncounterId: e.GetId()}))
	if err != nil {
		t.Fatalf("ListLastingEffects() error = %v", err)
	}
	res.Msg.Catalog = nil
	return a.rpcRuleJSON(t, res.Msg)
}

// TestLastingRPCEffectsMoveIntoACombatAndBack freezes effectsToCombat and effectsToCharacters:
// what each row becomes on the way in and on the way out.
func TestLastingRPCEffectsMoveIntoACombatAndBack(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	cast := a.mustCastOut(t, a.bia, a.bri, blessKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	a.rpcGive(t, heroismKey, a.pens)
	a.rpcGive(t, longstriderKey, a.toren, rpcCharDuration(playv1.EffectDurationKind_EFFECT_DURATION_KIND_LONG_REST))
	a.rpcGive(t, enhanceAbilityKey, a.pens, func(r *playv1.AddCharacterEffectRequest) { r.AbilityKey = "wis" }, rpcCharDuration(playv1.EffectDurationKind_EFFECT_DURATION_KIND_UNTIL_DISMISSED))
	a.rpcGive(t, guidanceKey, a.bri, rpcSeconds(5))
	a.rpcInsertCharacterEffect(t, a.bri, "spell:bane", "spell", "rounds", "", "")
	a.rpcInsertCharacterEffect(t, a.toren, "spell:faerie-fire", "spell", "rounds", "120", a.capitao.GetId())
	a.rpcInsertCharacterEffect(t, a.pens, "condition:prone", "master", "until_dismissed", "", a.bri.GetId())
	s := a.rpcScript(t)
	s.note("outside", a.rpcCharRows(t))

	e := a.closeFight(t)
	s.note("in the combat: rows", a.rpcClean(t, a.rpcRows(t)))
	s.note("in the combat: characters", a.rpcCharRows(t))
	s.note("in the combat: the table", a.rpcMasterTable(t, e))

	// The fight goes on: effects of its own, and a cast that ends.
	a.rpcAdd(t, e, "spell:bane", []string{"Goblin"}, a.rpcCaster(t, "Pensantus"), a.rpcDuration(t, rpcDurRounds, 4, "Pensantus"))
	a.rpcAdd(t, e, resistanceKey, []string{"Toren"}, a.rpcDuration(t, rpcDurStartTurn, 0, "Pensantus"))
	a.rpcAdd(t, e, "spell:hideous-laughter", []string{"Brisa"}, a.rpcCaster(t, "Toren"))
	a.rpcAdd(t, e, "condition:prone", []string{"Pensantus"}, a.rpcDuration(t, rpcDurEndTurn, 0, "Toren"))
	a.rpcAdd(t, e, "condition:blinded", []string{"Toren"}, a.rpcDuration(t, rpcDurRounds, 1, "Toren"))
	s.note("effects added: rows", a.rpcClean(t, a.rpcRows(t)))
	e = a.turnOf(t, "Toren")
	s.note("round two: rows", a.rpcClean(t, a.rpcRows(t)))
	a.execSQL(t, `UPDATE spell_casts SET status = 'ended', ended_at = now(), end_reason = 'dismissed' WHERE id = $1`, cast.GetId())
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	s.note("after the combat: characters", a.rpcCharRows(t))
	s.note("after the combat: rows", a.rpcClean(t, a.rpcRows(t)))
	s.want("move_combat_round_trip")
}

// TestLastingRPCAPlayerLeavesACombatInSetupKeepsItsEffects pins what happens to the effects of
// a character that the master takes out of a combat in setup: they go back to game time, as they
// do when a combat ends, and the effects on the others stay in the combat.
// Finding play-a-movement-3: a spell or a condition on a character is not the combat's to
// destroy, so removing the combatant must not delete it with the row.
func TestLastingRPCAPlayerLeavesACombatInSetupKeepsItsEffects(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.mustCastOut(t, a.bia, a.bri, blessKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren, a.pens})
	a.rpcGive(t, longstriderKey, a.toren, rpcCharDuration(playv1.EffectDurationKind_EFFECT_DURATION_KIND_LONG_REST))
	a.rpcGive(t, guidanceKey, a.pens, rpcSeconds(48))
	a.rpcInsertCharacterEffect(t, a.toren, "spell:faerie-fire", "spell", "rounds", "120", a.bri.GetId())
	a.rpcInsertCharacterEffect(t, a.toren, "spell:bane", "spell", "until_start_of_turn_of", "", "")
	s := a.rpcScript(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		setup:    true,
	})
	s.note("in the setup: rows", a.rpcClean(t, a.rpcRows(t)))
	s.note("in the setup: characters", a.rpcCharRows(t))
	if _, err := a.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Toren"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("RemoveCombatant() error = %v", err)
	}
	s.note("Toren left: rows", a.rpcClean(t, a.rpcRows(t)))
	s.note("Toren left: characters", a.rpcCharRows(t))
	s.want("move_leaver")
}
