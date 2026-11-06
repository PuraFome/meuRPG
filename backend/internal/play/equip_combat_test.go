package play

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Equipping and stowing gear in a combat (#562, SRD 5.1 "Other Activity on Your Turn"): the
// first weapon swap of the turn is the free object interaction, the next one and a shield
// cost the action, body armor is refused, the master is unrestricted.

const shieldItem = "equipment:shield"

// equipFight is reactorFight with Ana's Reator in chain mail and a shield, with a longsword
// and a dagger; the combat is on the Goblin's turn.
func equipFight(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	return reactorFight(t, func(t *testing.T, a *armed) *charactersv1.Character {
		t.Helper()
		return a.shieldBearer(t, "Reator", nil, []string{longswordKey, daggerKey})
	})
}

// swap calls CharacterService.SetEquipped as u for Reator.
func (a *armed) swap(u *user, c *charactersv1.Character, key string, equipped bool) (*charactersv1.SetEquippedResponse, error) {
	res, err := u.characters.SetEquipped(context.Background(), connect.NewRequest(&charactersv1.SetEquippedRequest{
		CampaignId: a.campaignID, CharacterId: c.GetId(), Key: key, Equipped: equipped,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) mustSwap(t *testing.T, u *user, c *charactersv1.Character, key string, equipped bool) *charactersv1.SetEquippedResponse {
	t.Helper()
	res, err := a.swap(u, c, key, equipped)
	if err != nil {
		t.Fatalf("SetEquipped(%s, %v) error = %v", key, equipped, err)
	}
	return res
}

// turnFlags reads what the combat remembers of the combatant's turn.
func (a *armed) turnFlags(t *testing.T, label string) (objectInteraction, action bool) {
	t.Helper()
	if err := a.h.pool.QueryRow(t.Context(), `SELECT object_interaction_used, action_used FROM combatants WHERE id = $1`, a.id(t, label)).Scan(&objectInteraction, &action); err != nil {
		t.Fatalf("read the turn of %s: %v", label, err)
	}
	return objectInteraction, action
}

func (a *armed) wantTurn(t *testing.T, step string, wantObject, wantAction bool) {
	t.Helper()
	if o, ac := a.turnFlags(t, "Reator"); o != wantObject || ac != wantAction {
		t.Errorf("%s: object interaction used %v, action used %v; want %v, %v", step, o, ac, wantObject, wantAction)
	}
}

func (a *armed) carriedOf(t *testing.T, c *charactersv1.Character) []string {
	t.Helper()
	res, err := a.master.characters.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: a.campaignID, CharacterId: c.GetId()}))
	if err != nil {
		t.Fatalf("GetCharacter() error = %v", err)
	}
	return res.Msg.GetCharacter().GetSheet().GetFull().GetCarriedKeys()
}

// Outside a combat gear is free: nothing is spent and nothing is refused, whoever asks.
func TestEquipOutsideACombatIsFree(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.pens = a.shieldBearer(t, "Reator", nil, []string{longswordKey, daggerKey})
	})
	for _, key := range []string{chainMailKey, shieldItem, longswordKey, daggerKey} {
		a.mustSwap(t, a.ana, a.pens, key, false)
	}
	if got := a.carriedOf(t, a.pens); len(got) != 4 {
		t.Errorf("carried = %v, want the four pieces", got)
	}
}

// The first weapon of the turn is the free object interaction; the second costs the action;
// a third with the action spent is refused and changes nothing.
func TestEquipWeaponSwapsFreeOnceThenCostTheAction(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	a.passTo(t, e, "Reator")
	a.wantTurn(t, "at the start of the turn", false, false)

	a.mustSwap(t, a.ana, a.pens, longswordKey, false)
	a.wantTurn(t, "after the first swap", true, false)
	a.mustSwap(t, a.ana, a.pens, daggerKey, false)
	a.wantTurn(t, "after the second swap", true, true)

	before := a.carriedOf(t, a.pens)
	_, err := a.swap(a.ana, a.pens, longswordKey, true)
	wantBlockedBy(t, "a third swap with the action spent", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED)
	if got := a.carriedOf(t, a.pens); len(got) != len(before) {
		t.Errorf("a refused swap changed the sheet: %v -> %v", before, got)
	}
}

// Asking for the state the piece already has spends nothing.
func TestEquipTheSameStateSpendsNothing(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	a.passTo(t, e, "Reator")
	a.mustSwap(t, a.ana, a.pens, longswordKey, true)
	a.wantTurn(t, "equipping an equipped weapon", false, false)
}

// The shield costs the action, and is refused once it is spent.
func TestEquipShieldCostsTheAction(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	a.passTo(t, e, "Reator")
	a.mustSwap(t, a.ana, a.pens, shieldItem, false)
	a.wantTurn(t, "after doffing the shield", false, true)
	_, err := a.swap(a.ana, a.pens, shieldItem, true)
	wantBlockedBy(t, "donning it again with the action spent", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED)
	if got := a.carriedOf(t, a.pens); len(got) != 1 || got[0] != shieldItem {
		t.Errorf("carried = %v, want the shield only", got)
	}
}

// Body armor takes minutes: a player cannot change it in a fight, even on their turn.
func TestEquipArmorIsRefusedInACombat(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	a.passTo(t, e, "Reator")
	_, err := a.swap(a.ana, a.pens, chainMailKey, false)
	wantBlockedBy(t, "doffing armor on the turn", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ARMOR_IN_COMBAT)
	a.wantTurn(t, "after the refusal", false, false)
	if got := a.carriedOf(t, a.pens); len(got) != 0 {
		t.Errorf("carried = %v after a refused change", got)
	}
}

// Off their turn the player is refused, with the refusal the attacks give.
func TestEquipNotYourTurnIsRefused(t *testing.T) {
	t.Parallel()
	a, _ := equipFight(t) // the Goblin's turn
	_, err := a.swap(a.ana, a.pens, daggerKey, false)
	wantBlockedBy(t, "a swap off turn", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN)
}

// The master has the final word: any turn, no economy, armor included.
func TestEquipMasterIsUnrestricted(t *testing.T) {
	t.Parallel()
	a, _ := equipFight(t) // the Goblin's turn
	for _, key := range []string{chainMailKey, shieldItem, longswordKey, daggerKey} {
		a.mustSwap(t, a.master, a.pens, key, false)
	}
	a.wantTurn(t, "after the master's changes", false, false)
	if got := a.carriedOf(t, a.pens); len(got) != 4 {
		t.Errorf("carried = %v, want the four pieces", got)
	}
}

// The free object interaction and the action come back at the start of the next turn.
func TestEquipTurnStartGivesTheInteractionBack(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustSwap(t, a.ana, a.pens, longswordKey, false)
	a.mustSwap(t, a.ana, a.pens, shieldItem, false)
	a.wantTurn(t, "end of the turn", true, true)
	a.passTo(t, e, "Goblin")
	a.passTo(t, e, "Reator")
	a.wantTurn(t, "next turn of Reator", false, false)
	a.mustSwap(t, a.ana, a.pens, daggerKey, false)
	a.wantTurn(t, "a swap in the new turn", true, false)
}

// A weapon stowed before the combat is no attack; drawing it in the combat offers it at once.
func TestEquipCarriedWeaponIsNoAttackUntilDrawn(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil)
		a.pens = a.shieldBearer(t, "Reator", nil, []string{longswordKey, daggerKey})
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3, abilities(10, 16, 14, 16, 8), []string{maceKey}, []string{sacredFlame}, nil, []string{cureWounds})
	})
	a.mustSwap(t, a.ana, a.pens, daggerKey, false) // outside the combat: free
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Reator": 18, "Toren": 10, "Brisa": 1}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Reator": {4, 4}, "Brisa": {8, 8}},
	})
	if attackOption(a.mustOptions(t, a.ana, e, "Reator"), daggerKey) != nil {
		t.Fatalf("the stowed dagger is offered as an attack")
	}
	if attackOption(a.mustOptions(t, a.ana, e, "Reator"), longswordKey) == nil {
		t.Fatalf("the equipped longsword is not offered")
	}
	a.mustSwap(t, a.ana, a.pens, daggerKey, true)
	opt := attackOption(a.mustOptions(t, a.ana, e, "Reator"), daggerKey)
	if opt == nil || !opt.GetEnabled() {
		t.Fatalf("the drawn dagger = %v, want an enabled attack", opt)
	}
	a.wantTurn(t, "after drawing it", true, false)
	a.mustAttack(t, a.ana, e, "Reator", daggerKey, "Goblin", d20(15))
}

// Mage Armor asks for a creature that wears no armor: the armor is stowed outside the combat
// (the cast is refused before), and the combat then reads Mage Armor's 13 + Dexterity.
func TestEquipMageArmorWithStowedArmorInACombat(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	dani := a.h.newUser("Dani")
	a.h.join(a.master, a.campaignID, dani)
	ilha := dani.castingHero(t, a.campaignID, "Ilha", classLevel("class:fighter", 2, ""),
		&rulesv1.AbilityScores{Strength: 14, Dexterity: 14, Constitution: 12, Intelligence: 10, Wisdom: 10, Charisma: 8}, nil, nil, nil, leatherArmorKey)
	_, err := a.castOut(t, a.ana, a.pens, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{ilha})
	wantCastingBlocked(t, "Mage Armor on a creature in armor", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_TARGET_WEARS_ARMOR)
	a.mustSwap(t, dani, ilha, leatherArmorKey, false)
	a.mustCastOut(t, a.ana, a.pens, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{ilha})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Ilha": 18, "Pensantus": 15, "Toren": 10, "Brisa": 1}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Ilha": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Toren": {5, 5}, "Brisa": {8, 8}},
	})
	if got := byLabel(t, e, "Ilha").GetArmorClass(); got != 15 {
		t.Errorf("Ilha's armor class in the combat = %d, want 15 (Mage Armor: 13 + Dexterity 2)", got)
	}
	// And in the combat the armor stays off the table for her.
	_, err = a.swap(dani, ilha, leatherArmorKey, true)
	wantBlockedBy(t, "donning armor in the combat", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ARMOR_IN_COMBAT)
}
