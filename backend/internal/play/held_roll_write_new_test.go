package play

import "testing"

// Since E4-6 the weapon attack that waits for a question is written in inspiration_holds, owned by its combatant,
// and roll_holds is written no more.

const selectAttackHolds = `SELECT id::TEXT, character_id::TEXT, user_id::TEXT, combatant_id::TEXT, encounter_id::TEXT, window_hold, round, faces[1], total
	FROM inspiration_holds WHERE encounter_id = $1 AND kind = 'weapon_attack' ORDER BY created_at`

type attackHoldRow struct {
	id, characterID, userID, combatantID, encounterID string
	window                                            bool
	round, face, total                                int32
}

// attackHoldsOf reads the weapon attacks of a combat that wait in inspiration_holds.
func attackHoldsOf(t *testing.T, a *armed, encounterID string) []attackHoldRow {
	t.Helper()
	rows, err := a.h.pool.Query(t.Context(), selectAttackHolds, encounterID)
	if err != nil {
		t.Fatalf("read the held attacks: %v", err)
	}
	defer rows.Close()
	var out []attackHoldRow
	for rows.Next() {
		var r attackHoldRow
		if err := rows.Scan(&r.id, &r.characterID, &r.userID, &r.combatantID, &r.encounterID, &r.window, &r.round, &r.face, &r.total); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// TestAHeldWeaponAttackIsWrittenInInspirationHolds: the hold names who rolled (the user), the character and the combatant
// that own it, the combat, the round and the d20 as it came up; roll_holds stays empty.
func TestAHeldWeaponAttackIsWrittenInInspirationHolds(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyInspired(t)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(14))
	if err != nil || held.GetInspirationOffer().GetHoldId() == "" {
		t.Fatalf("Pipo's attack = %v, %v; want the question about the die", held, err)
	}
	rows := attackHoldsOf(t, a, e.GetId())
	if len(rows) != 1 {
		t.Fatalf("%d held attacks in inspiration_holds, want 1", len(rows))
	}
	got := rows[0]
	if got.id != held.GetInspirationOffer().GetHoldId() || got.combatantID != a.id(t, "Pipo") || got.encounterID != e.GetId() ||
		got.window || got.round != 1 || got.face != 14 || got.total != 14+5 {
		t.Errorf("the hold = %+v, want Pipo's, in her combat, round 1, on the d20 of 14 (total 19), no window", got)
	}
	var owner string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT character_id::TEXT FROM combatants WHERE id = $1`, got.combatantID).Scan(&owner); err != nil || got.characterID != owner {
		t.Errorf("the hold's character = %q, want the combatant's, %q (%v)", got.characterID, owner, err)
	}
	if n := countRows(t, a.h, countRollHolds, e.GetId()); n != 0 {
		t.Errorf("%d rows in roll_holds, want none", n)
	}
	if out := a.outsideState(t, a.caio); len(out.GetOffers()) != 0 || len(out.GetLuckyOffers()) != 0 {
		t.Errorf("the list of the rolls out of a combat has %d die and %d Lucky offers, want the attack in the combat view only", len(out.GetOffers()), len(out.GetLuckyOffers()))
	}
}

// TestAHeldWeaponAttackThatAsksLuckyIsNotAnOutsideOffer: the Lucky question of a weapon attack is not on the list of the rolls
// out of a combat either: it is on its combatant.
func TestAHeldWeaponAttackThatAsksLuckyIsNotAnOutsideOffer(t *testing.T) {
	t.Parallel()
	a := newLuckyTable(t)
	e := a.luckyFight(t)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(1))
	if err != nil || held.GetLuckyOffer() == nil {
		t.Fatalf("Pipo's attack = %v, %v; want Lucky's question", held, err)
	}
	if out := a.outsideState(t, a.caio); len(out.GetOffers()) != 0 || len(out.GetLuckyOffers()) != 0 {
		t.Errorf("the list of the rolls out of a combat has %d die and %d Lucky offers, want none", len(out.GetOffers()), len(out.GetLuckyOffers()))
	}
}

// TestACreaturesHeldWeaponAttackKeepsItsOwnersCharacter: a creature is not backed by the character it carries, so the
// roller names no character to the gate, but the row has one (the owner's) and belongs to the creature's combatant.
func TestACreaturesHeldWeaponAttackKeepsItsOwnersCharacter(t *testing.T) {
	t.Parallel()
	a, e := giftedFight(t, 20)
	wolf := a.holdWolfBite(t, e)
	rows := attackHoldsOf(t, a, e.GetId())
	if len(rows) != 1 || rows[0].id != wolf.GetInspirationOffer().GetHoldId() {
		t.Fatalf("held attacks = %+v, want the wolf's", rows)
	}
	if rows[0].combatantID != a.id(t, "Presa") || rows[0].characterID != a.bri.GetId() {
		t.Errorf("the hold names combatant %q and character %q, want Presa and Brisa's character %q", rows[0].combatantID, rows[0].characterID, a.bri.GetId())
	}
}
