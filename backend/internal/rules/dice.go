package rules

import (
	"strconv"
	"strings"
)

// DiceFormula is "Count d Sides + Bonus", the shape of every damage and
// healing roll: 1d8+3 is {1, 8, 3}. A flat number such as Aid's "5" is
// {0, 0, 5}. The server never rolls (ADR-0008): the dice are rolled by the
// caller and combat.DamageTotal adds them up.
type DiceFormula struct {
	Count int
	Sides int
	Bonus int
	// AddsModifier says the SRD adds the caster's spellcasting modifier
	// ("1d8 + MOD", Cure Wounds). Bonus does not include it: the caller
	// adds the right modifier.
	AddsModifier bool
}

// ParseDice reads the forms the SRD and the sheet use: "1d10", "1d8+3",
// "1d6-1", "1d8 + MOD", "7d8 + 30" and "5". ok is false for anything else,
// such as Flame Strike's "4d6 OR 5d6".
func ParseDice(s string) (DiceFormula, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	var f DiceFormula
	if rest, ok := strings.CutSuffix(s, "+MOD"); ok {
		f.AddsModifier, s = true, rest
	}
	head, tail := s, ""
	if i := strings.IndexAny(s[min(1, len(s)):], "+-"); i >= 0 {
		head, tail = s[:i+1], s[i+1:]
	}
	if tail != "" {
		b, err := strconv.Atoi(tail)
		if err != nil {
			return DiceFormula{}, false
		}
		f.Bonus = b
	}
	n, sides, isDice := strings.Cut(head, "d")
	if !isDice {
		flat, err := strconv.Atoi(head)
		if err != nil || tail != "" || f.AddsModifier {
			return DiceFormula{}, false
		}
		f.Bonus = flat
		return f, true
	}
	count, err1 := strconv.Atoi(n)
	faces, err2 := strconv.Atoi(sides)
	if err1 != nil || err2 != nil || count < 1 || faces < 1 {
		return DiceFormula{}, false
	}
	f.Count, f.Sides = count, faces
	return f, true
}
