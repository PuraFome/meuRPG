package play

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A hand-made NPC keeps what it spent of its slots and per-day uses until the table's long rest;
// a monster of a combat starts every fight with all of them. These tests need the database.

// storedUse reads the NPC's own count, "" when the character has no row.
func (a *armed) storedUse(t *testing.T, characterID string) string {
	t.Helper()
	var used string
	err := a.h.pool.QueryRow(t.Context(), `SELECT slots_used::TEXT FROM npc_spell_use WHERE character_id = $1`, characterID).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		return ""
	}
	if err != nil {
		t.Fatalf("npc_spell_use of %s: %v", characterID, err)
	}
	return used
}

// characterOf is the character of a combatant.
func (a *armed) characterOf(t *testing.T, label string) string {
	t.Helper()
	var id string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT character_id FROM combatants WHERE id = $1`, a.id(t, label)).Scan(&id); err != nil {
		t.Fatalf("character of %s: %v", label, err)
	}
	return id
}

// bumpCommitted counts a use on a combatant the way a cast does, and commits.
func (a *armed) bumpCommitted(t *testing.T, label, key string, delta int32) {
	t.Helper()
	cs, err := a.h.svc.queries.ListCombatants(t.Context(), a.get(t, a.master).GetId())
	if err != nil {
		t.Fatalf("ListCombatants() error = %v", err)
	}
	var who playdb.Combatant
	for _, c := range cs {
		if c.ID == a.id(t, label) {
			who = c
		}
	}
	tx, err := a.h.pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer func() { _ = tx.Rollback(t.Context()) }()
	if err := a.h.svc.bumpUsed(t.Context(), &combatTx{tx: tx, q: a.h.svc.queries.WithTx(tx)}, who, key, delta); err != nil {
		t.Fatalf("bumpUsed() error = %v", err)
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
}

// handMadeMage is a table whose Mage was made by hand (not combat_only), after a fight where it
// was a monster; it returns the Mage's character.
func handMadeMage(t *testing.T) (a *armed, mage string) {
	t.Helper()
	a, e := mageFight(t)
	mage = a.characterOf(t, "Mago")
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE characters SET sheet = jsonb_set(sheet, '{basic,combat_only}', 'false') WHERE id = $1`, mage); err != nil {
		t.Fatalf("make the Mage by hand: %v", err)
	}
	a.endEncounter(t, e)
	return a, mage
}

// mageCombat starts a combat of the party and the Mage, the Mage on a square, and ends the setup.
func (a *armed) mageCombat(t *testing.T, mage string, setup bool) *playv1.Encounter {
	t.Helper()
	return a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: mage}}, npcRolls: []int{1},
		players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 5},
		reveal:  []string{"Mago"}, setup: setup,
		at: map[string][2]int32{"Toren": {6, 5}, "Pensantus": {5, 5}, "Brisa": {5, 6}, "Mago": {7, 5}},
	})
}

func (a *armed) magicMissile(t *testing.T, e *playv1.Encounter) *playv1.Encounter {
	t.Helper()
	e = a.passTo(t, e, "Mago")
	a.mustCast(t, a.master, e, "Mago", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Toren", 3)}, noCastRoll)
	return e
}

func (a *armed) rest(t *testing.T, kind playv1.RestKind) {
	t.Helper()
	if _, err := a.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{CampaignId: a.campaignID, Kind: kind, IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("TakeRest(%v) error = %v", kind, err)
	}
}

// TestAHandMadeNPCsSpentSlotAndUseCarryToTheNextCombat: the Mage spends a 1st-level slot and an
// innate use in one combat; the next combat of the same NPC starts with both spent.
func TestAHandMadeNPCsSpentSlotAndUseCarryToTheNextCombat(t *testing.T) {
	t.Parallel()
	a, mage := handMadeMage(t)
	e := a.magicMissile(t, a.mageCombat(t, mage, false))
	a.bumpCommitted(t, "Mago", usedResource+"innate_3_day_bless", 1)
	const want = `{"1": 1, "res:innate_3_day_bless": 1}`
	if got := a.storedUse(t, mage); got != want {
		t.Fatalf("the NPC's count after combat A = %s, want %s", got, want)
	}
	a.endEncounter(t, e)

	a.mageCombat(t, mage, false)
	if got := a.usedOf(t, "Mago"); got != want {
		t.Errorf("the Mage's count at the start of combat B = %s, want %s", got, want)
	}
}

// TestTheLongRestClearsAHandMadeNPCsUseAndTheShortRestDoesNot: a short rest changes nothing; a
// long rest clears the NPC's count and the one a combat in setup copied.
func TestTheLongRestClearsAHandMadeNPCsUseAndTheShortRestDoesNot(t *testing.T) {
	t.Parallel()
	a, mage := handMadeMage(t)
	e := a.magicMissile(t, a.mageCombat(t, mage, false))
	a.endEncounter(t, e)
	a.mageCombat(t, mage, true)
	if got := a.usedOf(t, "Mago"); got != `{"1": 1}` {
		t.Fatalf("the Mage's count in setup = %s, want one 1st-level slot", got)
	}
	a.rest(t, playv1.RestKind_REST_KIND_SHORT)
	if got := a.storedUse(t, mage); got != `{"1": 1}` {
		t.Errorf("the NPC's count after a short rest = %s, want it kept", got)
	}
	if got := a.usedOf(t, "Mago"); got != `{"1": 1}` {
		t.Errorf("the Mage's count in setup after a short rest = %s, want it kept", got)
	}
	a.rest(t, playv1.RestKind_REST_KIND_LONG)
	if got := a.storedUse(t, mage); got != "" {
		t.Errorf("the NPC's count after a long rest = %s, want none", got)
	}
	if got := a.usedOf(t, "Mago"); got != `{}` {
		t.Errorf("the Mage's count in setup after a long rest = %s, want none", got)
	}
}

// TestAMonstersCombatStartsFreshAndItsCopiesShareNothing: a monster (combat_only) spends a slot; its
// copy in the same fight and the next fight start with nothing, and nothing is kept for it.
func TestAMonstersCombatStartsFreshAndItsCopiesShareNothing(t *testing.T) {
	t.Parallel()
	a, e := mageFight(t)
	mage := a.characterOf(t, "Mago")
	e = a.magicMissile(t, e)
	a.h.roller.queue(1)
	res := a.mustAddMonsters(t, a.get(t, a.master), func(r *playv1.AddMonstersRequest) { r.CreatureKey, r.Count = mageKey, 1 })
	copyLabel := ""
	for _, c := range res.GetEncounter().GetCombatants() {
		if c.GetCharacterId() == mage && c.GetId() != a.id(t, "Mago") {
			copyLabel = c.GetLabel()
		}
	}
	if copyLabel == "" {
		t.Fatal("the second Mage did not join")
	}
	if got := a.usedOf(t, copyLabel); got != `{}` {
		t.Errorf("the copy's count = %s, want none", got)
	}
	if got := a.storedUse(t, mage); got != "" {
		t.Errorf("the monster's NPC count = %s, want none kept", got)
	}
	a.endEncounter(t, e)
	next := a.monsterSetup(t, false)
	a.h.roller.queue(1)
	a.mustAddMonsters(t, next, func(r *playv1.AddMonstersRequest) { r.CreatureKey, r.Count = mageKey, 1 })
	if got := a.usedOf(t, "Mago"); got != `{}` {
		t.Errorf("the monster's count in the next fight = %s, want none", got)
	}
}

// TestTheUndoOfAHandMadeNPCsCastGivesBackOnTheCombatantAndTheNPC.
func TestTheUndoOfAHandMadeNPCsCastGivesBackOnTheCombatantAndTheNPC(t *testing.T) {
	t.Parallel()
	a, mage := handMadeMage(t)
	e := a.magicMissile(t, a.mageCombat(t, mage, false))
	if got := a.storedUse(t, mage); got != `{"1": 1}` {
		t.Fatalf("the NPC's count after the cast = %s, want one slot", got)
	}
	a.undoLast(t, e)
	if got := a.usedOf(t, "Mago"); got != `{"1": 0}` {
		t.Errorf("the combatant's count after the undo = %s, want the slot back", got)
	}
	if got := a.storedUse(t, mage); got != `{"1": 0}` {
		t.Errorf("the NPC's count after the undo = %s, want the slot back", got)
	}
}

// TestAHandMadeNPCAddedMidCombatBringsItsCount: the Mage joins a running combat with the count it
// kept.
func TestAHandMadeNPCAddedMidCombatBringsItsCount(t *testing.T) {
	t.Parallel()
	a, mage := handMadeMage(t)
	a.endEncounter(t, a.magicMissile(t, a.mageCombat(t, mage, false)))
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{1},
		players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 5},
	})
	a.h.roller.queue(1)
	if _, err := a.master.combat.AddCombatants(t.Context(), connect.NewRequest(&playv1.AddCombatantsRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(),
		Participants: []*playv1.Participant{{CharacterId: mage}},
	})); err != nil {
		t.Fatalf("AddCombatants() error = %v", err)
	}
	if got := a.usedOf(t, "Mago"); got != `{"1": 1}` {
		t.Errorf("the Mage's count after joining = %s, want the slot it spent", got)
	}
}

// twoMages starts a combat of two copies of the hand-made Mage, in setup.
func (a *armed) twoMages(t *testing.T, mage string) {
	t.Helper()
	a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: mage, Count: 2}}, npcRolls: []int{1, 2},
		players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 5}, setup: true,
	})
}

// TestCopiesOfAHandMadeNPCKeepTheHighestCountNotTheSum: two copies each spend one of the Mage's two
// uses, so the NPC keeps 1 and each copy starts the next combat at 1; when one copy spends two and
// the other one, the NPC keeps 2; the undo of the higher copy lowers it to the other's count.
func TestCopiesOfAHandMadeNPCKeepTheHighestCountNotTheSum(t *testing.T) {
	t.Parallel()
	a, mage := handMadeMage(t)
	const key = usedResource + "innate_2_day_bless"
	a.twoMages(t, mage)
	a.bumpCommitted(t, "Mago 1", key, 1)
	a.bumpCommitted(t, "Mago 2", key, 1)
	if got := a.storedUse(t, mage); got != `{"res:innate_2_day_bless": 1}` {
		t.Fatalf("the NPC's count after each copy spent one = %s, want 1", got)
	}
	a.bumpCommitted(t, "Mago 1", key, 1)
	if got := a.storedUse(t, mage); got != `{"res:innate_2_day_bless": 2}` {
		t.Fatalf("the NPC's count after one copy spent two = %s, want 2", got)
	}
	a.bumpCommitted(t, "Mago 1", key, -1)
	if got := a.storedUse(t, mage); got != `{"res:innate_2_day_bless": 1}` {
		t.Errorf("the NPC's count after the undo of the higher copy = %s, want the other copy's 1", got)
	}
	a.endEncounter(t, a.get(t, a.master))
	a.twoMages(t, mage)
	for _, label := range []string{"Mago 1", "Mago 2"} {
		if got := a.usedOf(t, label); got != `{"res:innate_2_day_bless": 1}` {
			t.Errorf("%s starts the next combat at %s, want 1", label, got)
		}
	}
}

// TestAPlayersBumpLeavesNoNPCRow: a player's combatant counts nothing for an NPC.
func TestAPlayersBumpLeavesNoNPCRow(t *testing.T) {
	t.Parallel()
	a, mage := handMadeMage(t)
	a.mageCombat(t, mage, true)
	a.bumpCommitted(t, "Toren", "1", 1)
	if got := a.storedUse(t, a.characterOf(t, "Toren")); got != "" {
		t.Errorf("the player's NPC row = %s, want none", got)
	}
}

// TestTheUndoRestoresTheNPCCountAfterACarriedOverCount: a combat starts with the count the NPC
// kept, a cast raises it and the undo brings it back to what it carried.
func TestTheUndoRestoresTheNPCCountAfterACarriedOverCount(t *testing.T) {
	t.Parallel()
	a, mage := handMadeMage(t)
	a.endEncounter(t, a.magicMissile(t, a.mageCombat(t, mage, false)))
	e := a.magicMissile(t, a.mageCombat(t, mage, false))
	if got := a.storedUse(t, mage); got != `{"1": 2}` {
		t.Fatalf("the NPC's count after the second cast = %s, want 2", got)
	}
	a.undoLast(t, e)
	if got := a.storedUse(t, mage); got != `{"1": 1}` {
		t.Errorf("the NPC's count after the undo = %s, want the carried 1", got)
	}
	if got := a.usedOf(t, "Mago"); got != `{"1": 1}` {
		t.Errorf("the combatant's count after the undo = %s, want the carried 1", got)
	}
}

// TestDeletingTheNPCRemovesItsCount: the count is kept for the character (ON DELETE CASCADE), so deleting
// the character takes its row.
func TestDeletingTheNPCRemovesItsCount(t *testing.T) {
	t.Parallel()
	a, mage := handMadeMage(t)
	e := a.magicMissile(t, a.mageCombat(t, mage, false))
	a.endEncounter(t, e)
	if got := a.storedUse(t, mage); got != `{"1": 1}` { // positive control: the row is there
		t.Fatalf("the NPC's count before the delete = %s, want one slot", got)
	}
	if _, err := a.h.pool.Exec(t.Context(), `DELETE FROM characters WHERE id = $1`, mage); err != nil {
		t.Fatalf("delete the NPC: %v", err)
	}
	if got := a.storedUse(t, mage); got != "" {
		t.Errorf("the NPC's count after the delete = %s, want none", got)
	}
}
