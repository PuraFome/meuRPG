package leaktest

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Whether a player's free object interaction is spent this turn (Combatant.object_interaction_used) is the owner's and the
// master's. Another player reads no word of it: the owner's and the master's reads are the positive controls.
func TestAPlayersSpentObjectInteractionIsTheOwnersAndTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()
	m := w.master
	sheet := w.fullSheet("class:fighter", "race:human", 3)
	sheet.GetFull().WeaponKeys = []string{"equipment:dagger"}
	must(m.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), Revision: w.toren.GetRevision(), Name: w.toren.GetName(), Sheet: sheet,
	})))
	toren := w.combatant(w.toren)
	for range 40 {
		cur := must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() == toren.GetId() {
			w.encounter = cur
			break
		}
		must(m.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: w.campaign, EncounterId: cur.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound()})))
	}
	if w.encounter.GetCurrentCombatantId() != toren.GetId() {
		t.Fatal("it never became Toren's turn")
	}
	// Caio stows the dagger: the first weapon of the turn is free, and the free interaction is spent.
	must(w.caio.characters.SetEquipped(ctx, rq(&charactersv1.SetEquippedRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), Key: "equipment:dagger", Equipped: false,
	})))
	w.secrets.add(&canary{needle: `"objectInteractionUsed"`, kind: "object-interaction-used"})

	read := func(p *person) *playv1.Combatant {
		t.Helper()
		e := must(p.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		return w.combatantOf(e, w.toren.GetId())
	}
	for _, p := range []*person{w.caio, m} {
		if !read(p).GetObjectInteractionUsed() {
			t.Errorf("%s reads the free object interaction as unspent, want spent", p.name)
		}
	}
	if read(w.ana).GetObjectInteractionUsed() {
		t.Error("Ana reads Toren's spent object interaction")
	}
	for _, p := range []*person{w.ana, w.pending} {
		for _, r := range w.reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	found := false
	for _, r := range w.reads(m) {
		found = found || contains(r.body, `"objectInteractionUsed"`)
	}
	if !found {
		t.Error("the master's reads do not hold the field: the canary is unreachable")
	}
}
