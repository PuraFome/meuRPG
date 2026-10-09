package characters

import (
	"encoding/json"
	"fmt"
	"strconv"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The hit dice a character has spent are kept by die size (SRD 5.1,
// "Multiclassing"): character_vitals.hit_dice_used_by_die is a JSON object
// {die size: count}. A row written before the sizes were kept has NULL there, and
// its hit_dice_used (a count with no sizes) is read as the largest dice spent
// first, so an old character never has more healing than the old count allowed.

// hitDiceUsedOf reads the dice a stored row has spent, cut to the dice the
// character has now.
func hitDiceUsedOf(row vitalsRow, m vitalsMax) rules.HitDiceUsed {
	if row.HitDiceUsedByDie == nil {
		return rules.SplitHitDiceUsed(m.hitDice, int(derefInt(row.HitDiceUsed)))
	}
	var stored map[string]int
	if json.Unmarshal(row.HitDiceUsedByDie, &stored) != nil {
		return rules.HitDiceUsed{} // a value that does not read counts as nothing spent
	}
	used := rules.HitDiceUsed{}
	for key, n := range stored {
		if die, err := strconv.Atoi(key); err == nil {
			used[die] = n
		}
	}
	return used.Clean(m.hitDice)
}

// encodeHitDiceUsed is the JSON object a row keeps and the sum it keeps beside it.
func encodeHitDiceUsed(used rules.HitDiceUsed) (body []byte, total int32, err error) {
	out := map[string]int{}
	for die, n := range used {
		if n > 0 {
			out[strconv.Itoa(die)] = n
		}
	}
	body, err = json.Marshal(out)
	if err != nil {
		return nil, 0, fmt.Errorf("encode the hit dice: %w", err)
	}
	return body, i32(used.Total()), nil
}

func hitDiceToProto(used rules.HitDiceUsed) map[int32]int32 {
	out := map[int32]int32{}
	for die, n := range used {
		if n > 0 {
			out[i32(die)] = i32(n)
		}
	}
	return out
}

func hitDiceFromProto(used map[int32]int32) rules.HitDiceUsed {
	out := rules.HitDiceUsed{}
	for die, n := range used {
		if n > 0 {
			out[int(die)] = int(n)
		}
	}
	return out
}

// hitDiceOf is the character's hit dice (its maximums) as the rules read them.
func hitDiceOf(v *playv1.CharacterVitals) []rules.HitDice {
	out := make([]rules.HitDice, 0, len(v.GetHitDice()))
	for _, hd := range v.GetHitDice() {
		out = append(out, rules.HitDice{Die: int(hd.GetFaces()), Count: int(hd.GetCount())})
	}
	return out
}

// setHitDiceUsed puts the spent dice on the vitals, with the sum.
func setHitDiceUsed(v *playv1.CharacterVitals, used rules.HitDiceUsed) {
	v.HitDiceUsedByDie = hitDiceToProto(used)
	v.HitDiceUsed = i32(used.Total())
}

// slotsCreatedOf is the slots Flexible Casting created, as stored (never nil, for
// the column is NOT NULL).
func slotsCreatedOf(row vitalsRow) []int32 {
	out := make([]int32, 0, len(row.SpellSlotsCreated))
	return append(out, row.SpellSlotsCreated...)
}

// hitDiceProto turns the spent dice into the messages a rest preview carries.
func hitDiceProto(dice rules.HitDiceUsed, have []rules.HitDice) []*rulesv1.HitDice {
	var out []*rulesv1.HitDice
	for _, hd := range have { // the character's order: the largest die first
		if n := dice[hd.Die]; n > 0 {
			out = append(out, &rulesv1.HitDice{Faces: i32(hd.Die), Count: i32(n)})
		}
	}
	return out
}
