package play

import (
	"encoding/json"
	"slices"
	"testing"
)

// TestACastsSettledDamageFollowsTheCombatOrder: an area cast on three targets opens one pending damage for
// each, all created together; the one roll settles them in the combat's order, whichever pending damage the
// caller rolls, and the settled hits of the event and the answer's cast pendings read in that order.
func TestACastsSettledDamageFollowsTheCombatOrder(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 2)
	a.put(t, "Goblin 1", 7, 5)
	a.put(t, "Capitão Goblin", 8, 6)

	a.h.roller.queue(5, 18, 12)
	cast := a.mustCast(t, a.ana, e, "Pensantus", burningHands, slotOfLevel(1), a.at(t, "Goblin 1", "Goblin 2", "Capitão Goblin"), noCastRoll)
	pendings := cast.GetCast().GetPendingDamages()
	if len(pendings) != 3 {
		t.Fatalf("pending damages = %d, want one for each target", len(pendings))
	}

	var inCombatOrder []string
	targets := []string{a.id(t, "Goblin 1"), a.id(t, "Goblin 2"), a.id(t, "Capitão Goblin")}
	for _, c := range a.get(t, a.master).GetCombatants() {
		if slices.Contains(targets, c.GetId()) {
			inCombatOrder = append(inCombatOrder, c.GetId())
		}
	}
	// Rolling the last one in the combat's order puts the rolled pending damage out of the order of the rest.
	rolled := pendings[0]
	for _, p := range pendings {
		if p.GetTargetId() == inCombatOrder[len(inCombatOrder)-1] {
			rolled = p
		}
	}
	a.h.roller.queue(3, 3, 3)
	res := a.mustDamage(t, a.ana, e, rolled.GetId(), inAppDamage)

	var others []string
	for _, p := range res.GetCastPendingDamages() {
		others = append(others, p.GetTargetId())
	}
	wantOthers := slices.DeleteFunc(slices.Clone(inCombatOrder), func(id string) bool { return id == rolled.GetTargetId() })
	if !slices.Equal(others, wantOthers) {
		t.Errorf("the cast's other pending damages follow %v, want the combat's order %v", others, wantOthers)
	}

	var settled []string
	rows, err := a.h.pool.Query(t.Context(), `SELECT payload::TEXT FROM session_events WHERE kind = $1 ORDER BY seq`, eventDamageRolled)
	if err != nil {
		t.Fatalf("read the events: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			t.Fatalf("scan an event: %v", err)
		}
		var ev actionEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			t.Fatalf("decode an event: %v", err)
		}
		for _, h := range ev.Settled {
			settled = append(settled, h.Target)
		}
	}
	if !slices.Equal(settled, inCombatOrder) {
		t.Errorf("the settled hits follow %v, want the combat's order %v", settled, inCombatOrder)
	}
}
