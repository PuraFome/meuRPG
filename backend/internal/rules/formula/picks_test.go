package formula

import "testing"

// mod("@name") and score("@name") name the pick of a feat's choice: only a compiler
// told the name accepts it, only in those two calls, and the Env receives the literal.
func TestPicksAreLiteralsOfModAndScore(t *testing.T) {
	t.Parallel()
	plain := newCompiler()
	c := plain.WithPicks([]string{"ability", "other"})

	for _, src := range []string{`mod("@ability")`, `8 + prof() + mod("@ability")`, `score("@other") - 10`, `mod("dex") + mod("@ability")`} {
		if _, err := plain.Compile(src, Int); err == nil {
			t.Errorf("a compiler with no picks accepted %q", src)
		}
		if _, err := c.Compile(src, Int); err != nil {
			t.Errorf("Compile(%q) error = %v", src, err)
		}
		// States and the slot keep the picks.
		if _, err := c.WithStates().WithSlot().Compile(src, Int); err != nil {
			t.Errorf("with states and slot: Compile(%q) error = %v", src, err)
		}
	}
	for _, src := range []string{`mod("@nobody")`, `mod("@")`, `mod("ability")`, `mod("@Ability")`, `classLevel("@ability")`, `armor() == "@ability" ? 1 : 0`} {
		if _, err := c.Compile(src, Int); err == nil {
			t.Errorf("Compile(%q) was accepted", src)
		}
	}

	p, err := c.Compile(`8 + mod("@ability") + score("@other")`, Int)
	if err != nil {
		t.Fatal(err)
	}
	env := wizard3()
	env.Mod = func(a string) int {
		if a == "@ability" {
			return 3
		}
		return 0
	}
	env.Score = func(a string) int {
		if a == "@other" {
			return 14
		}
		return 0
	}
	if got, err := p.Int(env); err != nil || got != 25 {
		t.Errorf("Int() = %d, %v; want 25", got, err)
	}
}
