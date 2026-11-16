package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// inCombat is what u reads of whether who fights in a running combat, for the entry of the key on who's sheet.
func (s *stocked) inCombat(t *testing.T, u *user, who *charactersv1.Character, key string) bool {
	t.Helper()
	res, err := u.charges.ListConsumableTargets(t.Context(), connect.NewRequest(&playv1.ListConsumableTargetsRequest{
		CampaignId: s.campaignID, CharacterId: who.GetId(), EntryId: s.entryOf(t, who, key),
	}))
	if err != nil {
		t.Fatalf("ListConsumableTargets() error = %v", err)
	}
	return res.Msg.GetInCombat()
}

// The list says in_combat for a user that is a combatant of a running combat, whatever the item, to the player and to the master, and
// the states of the targets come from the same test: they are set exactly when the answer is true and the item is a potion.
func TestTheTargetsSayTheUserIsInARunningCombat(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionCommon, scrollFirst)
	for _, key := range []string{potionCommon, scrollFirst} {
		for _, u := range []*user{f.caio, f.master} {
			if !f.inCombat(t, u, f.toren, key) {
				t.Errorf("%s: in_combat is false for Toren in a running combat", key)
			}
		}
	}
	for _, g := range f.targets(t, f.caio, f.toren, potionCommon) {
		if g.GetState() == playv1.ConsumableTargetState_CONSUMABLE_TARGET_STATE_UNSPECIFIED {
			t.Errorf("target %v has no state in a running combat", g)
		}
	}
}

// A combat in setup has no turn: in_combat is false, and so is it for a character that does not fight in the running combat.
func TestTheTargetsSayNoCombatInSetupAndForAUserWhoDoesNotFight(t *testing.T) {
	t.Parallel()
	s := newStocked(t, potionCommon, scrollFirst)
	if s.inCombat(t, s.caio, s.toren, potionCommon) {
		t.Error("in_combat is true with no combat at all")
	}
	f := &potionFight{stocked: s, e: s.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: s.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Pensantus": {4, 4}, "Brisa": {9, 8}, "Goblin": {12, 8}},
		setup:    true,
	})}
	for _, key := range []string{potionCommon, scrollFirst} {
		if f.inCombat(t, f.caio, f.toren, key) {
			t.Errorf("%s: in_combat is true for a combat in setup", key)
		}
	}
	if f.get(t, f.master).GetStatus() != playv1.EncounterStatus_ENCOUNTER_STATUS_SETUP {
		t.Fatal("the combat is not in setup")
	}
	g := newPotionFight(t, potionCommon)
	out := g.withItems(t, g.fourthHero(t, "Orvalho"), potionCommon)
	if g.inCombat(t, g.master, out, potionCommon) {
		t.Error("in_combat is true for a character that is no combatant of the running combat")
	}
}
