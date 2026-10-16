package play

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Characterization of tallyHighlights' branches that highlights_test.go leaves
// out: critical hits (attack and spell, friendly fire), Lay on Hands, friendly
// fire on a damage roll, the player behind a character and payloads that do not
// decode. Fabricated events, no database.

func tallyWithUser() ([]playdb.Combatant, string) {
	uid := "user-toren"
	cs := party()
	cs[0].UserID = &uid
	return cs, uid
}

func TestTallyHighlights_CriticalHitsCountOnlyAgainstNPCs(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	f.add(eventAttackRolled, actionEvent{Actor: "c-toren", Target: "c-gob", Outcome: outcomeCrit})
	f.add(eventAttackRolled, actionEvent{Actor: "c-toren", Target: "c-bri", Outcome: outcomeCrit}) // friendly fire
	f.add(eventAttackRolled, actionEvent{Actor: "c-toren", Target: "c-gob", Outcome: "hit"})       // not a crit
	f.add(eventAttackRolled, actionEvent{Actor: "c-gob", Target: "c-toren", Outcome: outcomeCrit}) // an NPC's
	f.add(eventAttackRolled, actionEvent{Actor: "c-gone", Target: "c-gob", Outcome: outcomeCrit})  // left the combat
	f.add(eventSpellCast, actionEvent{Actor: "c-bri", Hits: []castHit{
		{Target: "c-gob", Outcome: outcomeCrit},
		{Target: "c-toren", Outcome: outcomeCrit},
		{Target: "c-gob", Outcome: "hit"},
		{Target: "c-gob", Outcome: outcomeCrit},
	}})
	f.add(eventSpellCast, actionEvent{Actor: "c-gob", Hits: []castHit{{Target: "c-bri", Outcome: outcomeCrit}}})
	got := tallyOf(f)
	if got["Toren"].crits != 1 || got["Brisa"].crits != 2 {
		t.Errorf("crits: Toren %d, Brisa %d; want 1 and 2", got["Toren"].crits, got["Brisa"].crits)
	}
}

func TestTallyHighlights_LayOnHandsAndFriendlyFire(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	f.add(eventActionTaken, actionEvent{Actor: "c-toren", Res: &resourceEvent{Kind: "lay_on_hands", Healed: 5}})
	f.add(eventActionTaken, actionEvent{Actor: "c-toren", Heal: true, Amount: 2, Res: &resourceEvent{Healed: 3}}) // both add
	f.add(eventActionTaken, actionEvent{Actor: "c-gob", Heal: true, Amount: 9})                                   // an NPC's
	// Friendly fire: the party hitting the party counts nowhere.
	f.add(eventDamageRolled, actionEvent{Actor: "c-toren", Target: "c-bri", Applied: true, Amount: 4, Before: &hpState{HP: 9}, After: &hpState{HP: 5}})
	// An applied hit with no hit points snapshot counts nowhere either.
	f.add(eventDamageRolled, actionEvent{Actor: "c-toren", Target: "c-gob", Applied: true, Amount: 4})
	// A pending hit (not applied) waits for damage_applied.
	f.add(eventDamageRolled, actionEvent{Actor: "c-toren", Target: "c-gob", Amount: 4, Before: &hpState{HP: 9}, After: &hpState{HP: 5}})
	// A hit on an NPC already defeated is no final blow.
	f.add(eventDamageRolled, actionEvent{Actor: "c-bri", Target: "c-gob", Applied: true, Amount: 4, Before: &hpState{HP: 0, Defeated: true}, After: &hpState{HP: 0, Defeated: true}})
	got := tallyOf(f)
	if got["Toren"].healing != 10 {
		t.Errorf("Toren healing = %d, want 10 (5, then 2 and 3)", got["Toren"].healing)
	}
	if got["Toren"].damage != 0 || got["Brisa"].damage != 0 || got["Brisa"].finalBlows != 0 || got["Toren"].taken != 0 {
		t.Errorf("tally = %+v, want no damage, no final blow", got)
	}
}

func TestTallyHighlights_SettledHitsAndUserOfTheCharacter(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	f.add(eventDamageRolled, actionEvent{Actor: "c-toren", Settled: []damageHit{
		{Target: "c-gob", Applied: true, Amount: 9, Before: &hpState{HP: 9}, After: &hpState{HP: 0, Defeated: true}},
		{Target: "c-bri", Applied: true, Amount: 9, Before: &hpState{HP: 9}, After: &hpState{HP: 0}}, // friendly fire
		{Target: "c-gob", Amount: 9, Before: &hpState{HP: 9}, After: &hpState{HP: 0}},                // not applied
	}})
	cs, uid := tallyWithUser()
	out := map[string]characterTally{}
	var order []string
	for _, tl := range tallyHighlights(f.rows, cs) {
		out[tl.name] = *tl
		order = append(order, tl.name)
	}
	if out["Toren"].damage != 9 || out["Toren"].finalBlows != 1 {
		t.Errorf("Toren = %+v, want 9 damage and a final blow", out["Toren"])
	}
	if out["Toren"].userID != uid || out["Brisa"].userID != "" {
		t.Errorf("users: Toren %q, Brisa %q; want %q and none", out["Toren"].userID, out["Brisa"].userID, uid)
	}
	if len(order) != 2 || order[0] != "Toren" || order[1] != "Brisa" {
		t.Errorf("order = %v, want the players in the order of the combatants, no NPC", order)
	}
}

func TestTallyHighlights_PayloadsThatDoNotDecodeAreSkipped(t *testing.T) {
	t.Parallel()
	f := &fabric{}
	f.add(eventAttackRolled, actionEvent{Actor: "c-toren", Target: "c-gob", Outcome: outcomeCrit})
	f.rows = append(f.rows,
		playdb.ListEncounterCombatEventsRow{ID: "bad1", Kind: eventAttackRolled, Payload: []byte("not json")},
		playdb.ListEncounterCombatEventsRow{ID: "bad2", Kind: eventActionUndone, Payload: []byte("not json")},
		playdb.ListEncounterCombatEventsRow{ID: "other", Kind: "some_other_kind", Payload: []byte(`{"actor":"c-toren","outcome":"crit"}`)},
	)
	if got := tallyOf(f); got["Toren"].crits != 1 {
		t.Errorf("crits = %d, want 1: the broken payloads and unknown kinds count for nothing", got["Toren"].crits)
	}
}
