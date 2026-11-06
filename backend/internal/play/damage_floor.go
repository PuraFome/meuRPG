package play

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The table's roll_minimum on the damage dice (#757): a die of damage that shows less than the minimum counts as the
// minimum, and a die smaller than the minimum (a d4 under 6) counts as its own highest face. The app applies it to every
// die it rolled, whether the part the die belongs to counts toward the total or not (a player reads the dice of a
// conditional part as counted, whatever the target is, so skipping one would tell what the target is), and after the
// rerolls: the reroll runs on the face that came up, the floor on the die that stays. The faces stay as they came up.
// A typed sum has no faces, so nothing is floored.

// countedDie is a die of damage that counted as another number than it showed.
type countedDie struct {
	Index int32 `json:"i"`
	Face  int32 `json:"face"`
	To    int32 `json:"to"`
	// SourceKey is the table feature whose minimum raised the die.
	SourceKey string `json:"source_key,omitempty"`
}

// damageFloorFor is the floor of the dice of a damage of the type: the highest of the damage minimums that cover it.
// A minimum limited to a damage type covers only dice of that type, one without covers all of them. A tie keeps the
// earlier entry, so the source is the same every time.
func damageFloorFor(minimums []link.RollMinimum, damageType string) rollFloor {
	var best rollFloor
	for _, m := range minimums {
		if m.Value > best.Value && slices.Contains(m.Applies, rules.RollDamage) && (m.DamageType == "" || m.DamageType == damageType) {
			best = rollFloor{Value: m.Value, Source: m.Source}
		}
	}
	return best
}

// countedDamageDie is the number a die of the given sides that showed face counts as. The minimum never goes past the
// die's own highest face.
func countedDamageDie(face, sides int, f rollFloor) int {
	return max(face, min(f.Value, sides))
}

// countDamageDice counts the dice by the floor: the sum of what they count as, and the dice the floor changed.
func countDamageDice(faces []int32, sides int32, f rollFloor) (sum int32, changed []countedDie) {
	for i, face := range faces {
		n := clamp32(countedDamageDie(int(face), int(sides), f), 0, 100)
		sum += n
		if n != face {
			changed = append(changed, countedDie{Index: clamp32(i, 0, 200), Face: face, To: n, SourceKey: f.Source})
		}
	}
	return sum, changed
}

// recount counts the dice of the part by the floor again, from the faces as they came up: the part's sum is what they
// count as. Doing it again gives the same, so it follows a reroll too. A part with no faces (a typed sum) is left alone.
func (pr *partRoll) recount(f rollFloor) {
	if len(pr.Faces) == 0 {
		return
	}
	pr.Sum, pr.Floored = countDamageDice(pr.Faces, pr.Sides, f)
}

// countedProto is the dice a floor changed, as the API tells them.
func countedProto(dice []countedDie) []*playv1.CountedDie {
	var out []*playv1.CountedDie
	for _, d := range dice {
		out = append(out, &playv1.CountedDie{Index: d.Index, Face: d.Face, CountedAs: d.To, SourceKey: d.SourceKey})
	}
	return out
}

// readCountedDice reads the dice a floor changed, kept in a column.
func readCountedDice(raw []byte) []countedDie {
	var out []countedDie
	if len(raw) == 0 || json.Unmarshal(raw, &out) != nil {
		return nil
	}
	return out
}

// countedColumn is the column's JSON for the dice: the empty list for none.
func countedColumn(dice []countedDie) []byte {
	if len(dice) == 0 {
		return emptyJSONList
	}
	return mustJSON(dice)
}

// attackerTraits are the traits of the attacker's sheet, read once for the roll.
func (f *damageFlow) attackerTraits(ctx context.Context) (link.Traits, error) {
	if f.traitsRead {
		return f.traits, nil
	}
	sheet, err := f.s.sheetOf(ctx, f.c.tx, f.m.CampaignID, f.attacker)
	if err != nil {
		return link.Traits{}, err
	}
	f.traits, f.traitsRead = sheet.Traits, true
	return f.traits, nil
}

// floorDamage counts the dice just rolled by the attacker's minimums. It runs before a reaction window may hold the
// damage, so the windows decide on what the roll counts, and the replay of a held roll lands it as it was kept.
func (f *damageFlow) floorDamage(ctx context.Context) error {
	if !f.mayCountDice() {
		return nil
	}
	traits, err := f.attackerTraits(ctx)
	if err != nil {
		return err
	}
	if f.minimums = traits.RollMinimums; len(f.minimums) > 0 {
		f.recountDice()
	}
	return nil
}

// recountDice counts every die of the roll by the floor again, from the faces as they came up, and works the totals out
// from what they count as. It runs after the roll and after a reroll.
func (f *damageFlow) recountDice() {
	if f.pr != nil {
		for i := range f.pr.rolls {
			pr := &f.pr.rolls[i]
			pr.recount(damageFloorFor(f.minimums, pr.DamageType))
		}
		f.pr.byType, f.pr.total = map[string]int{}, 0
		f.pr.sumUp()
		f.roll = f.pr.legacy()
		return
	}
	faces := faces32(f.roll.Faces)
	sum, counted := countDamageDice(faces, clamp32(f.expr.Sides, 0, 100), damageFloorFor(f.minimums, f.p.DamageType))
	f.counted = counted
	f.roll.Total = int(sum) + f.roll.Modifier
}
