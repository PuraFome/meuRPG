package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// What a creature of a stat block has spent, kept on the combatant in
// combatants.creature_spent as JSON {"<key>": N}: only what was spent, a key that
// is absent is 0. The keys:
//
//   - "legendary_actions": legendary actions taken since the creature's last turn;
//     ResetCombatantTurn clears it when the creature's own turn starts.
//   - "legendary_resistance": uses of Legendary Resistance spent in this combat (the
//     "day" is the table's rest between combats).
//   - "use:<action key>" and "recharge:<action key>": reserved for the N/day and
//     Recharge actions (issue #556); nothing here writes them, and every key this
//     file does not know is kept as it is.
//
// The column is new in every combat (a combatant starts with '{}'). The monster's
// reaction stays in combatants.reaction_used (one per round, RN-31).
const (
	spentLegendaryActions    = "legendary_actions"
	spentLegendaryResistance = "legendary_resistance"
)

// creatureSpentLimit is the most bytes the column's text has (the table's CHECK).
const creatureSpentLimit = 4096

// errCreatureSpentFull is the refusal of a spend that would take the column over its limit.
var errCreatureSpentFull = errors.New("what the creature spent does not fit in 4 KiB")

// creatureSpentOf reads what a combatant spent: every key, the ones this package
// does not know too.
func creatureSpentOf(raw []byte) map[string]int {
	out := map[string]int{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out) // a value this package wrote
	}
	return out
}

// bumpCreatureSpent adds delta to what the combatant spent under key, never below 0,
// and returns what it spent now. A key that reaches 0 is dropped (only what was
// spent is kept). A result over 4 KiB is refused with errCreatureSpentFull and
// nothing changes.
func bumpCreatureSpent(ctx context.Context, q *playdb.Queries, id, key string, delta int) (map[string]int, error) {
	spent, err := readCreatureSpent(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if next := max(spent[key]+delta, 0); next > 0 {
		spent[key] = next
	} else {
		delete(spent, key)
	}
	return spent, writeCreatureSpent(ctx, q, id, spent)
}

// clearCreatureSpent forgets key (the other keys stay) and returns what is left.
func clearCreatureSpent(ctx context.Context, q *playdb.Queries, id, key string) (map[string]int, error) {
	spent, err := readCreatureSpent(ctx, q, id)
	if err != nil {
		return nil, err
	}
	delete(spent, key)
	return spent, writeCreatureSpent(ctx, q, id, spent)
}

// jsonbTextLen is the length of the text form of a JSON object of pairs pairs that
// the database checks: JSONB writes ": " and ", " where compact JSON writes ":" and
// ",", one byte more for each.
func jsonbTextLen(compact []byte, pairs int) int {
	if pairs == 0 {
		return len(compact)
	}
	return len(compact) + 2*pairs - 1
}

// readCreatureSpent reads the column now.
func readCreatureSpent(ctx context.Context, q *playdb.Queries, id string) (map[string]int, error) {
	raw, err := q.GetCombatantCreatureSpent(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("read what the creature spent: %w", err)
	}
	return creatureSpentOf(raw), nil
}

// writeCreatureSpent saves spent, refusing what the column cannot hold.
func writeCreatureSpent(ctx context.Context, q *playdb.Queries, id string, spent map[string]int) error {
	body, err := json.Marshal(spent)
	if err != nil {
		return fmt.Errorf("encode what the creature spent: %w", err)
	}
	if n := jsonbTextLen(body, len(spent)); n > creatureSpentLimit {
		return fmt.Errorf("%w: %d bytes, the limit is %d", errCreatureSpentFull, n, creatureSpentLimit)
	}
	if err := q.SetCombatantCreatureSpent(ctx, playdb.SetCombatantCreatureSpentParams{ID: id, CreatureSpent: body}); err != nil {
		return fmt.Errorf("save what the creature spent: %w", err)
	}
	return nil
}
