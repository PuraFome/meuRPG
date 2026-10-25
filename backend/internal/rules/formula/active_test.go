package formula

import (
	"slices"
	"strings"
	"testing"
)

// active("name") is true while a state of the entry is on. It is a gate, so it
// compiles only for a compiler made with WithStates (the `when` of an effect),
// and its name is a literal that looks like a state name.

func TestActiveReadsTheStatesOfTheEnvironment(t *testing.T) {
	t.Parallel()
	c := newCompiler().WithStates()
	env := wizard3()
	env.Active = func(name string) bool { return name == "postura" }

	tests := []struct {
		source string
		want   bool
	}{
		{`active("postura")`, true},
		{`active("forma")`, false},
		{`not active("forma")`, true},
		{`active("postura") and level() >= 3`, true},
		{`active("postura") and active("forma")`, false},
		{`active("forma") or armor() == "none"`, true},
		{`active("postura") ? level() > 5 : true`, false},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			p, err := c.Compile(tt.source, Bool)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			got, err := p.Bool(env)
			if err != nil {
				t.Fatalf("Bool: %v", err)
			}
			if got != tt.want {
				t.Errorf("= %v, want %v", got, tt.want)
			}
		})
	}
}

func TestActiveIsRefusedWhereNoGateMakesSense(t *testing.T) {
	t.Parallel()
	plain := newCompiler()
	for _, kind := range []Kind{Int, Bool} {
		if _, err := plain.Compile(`active("postura")`, kind); err == nil || !strings.Contains(err.Error(), "active") {
			t.Errorf("kind %d: a plain compiler accepts active(): %v", kind, err)
		}
	}
	// A number never reads a state, even for a compiler that allows the gate.
	if _, err := plain.WithStates().Compile(`active("postura") ? 2 : 1`, Int); err == nil {
		t.Error("an Int formula reads a state")
	}
}

func TestActiveNeedsOneLiteralThatLooksLikeAStateName(t *testing.T) {
	t.Parallel()
	c := newCompiler().WithStates()
	for _, source := range []string{
		`active()`, `active("a", "b")`, `active(1)`, `active(level())`, `active(postura)`,
		`active("")`, `active("Postura")`, `active("com espaço")`, `active("com-hifen")`, `active("` + strings.Repeat("a", 41) + `")`,
		`active("postura") == 1`, `active("postura") == "sim"`,
	} {
		if _, err := c.Compile(source, Bool); err == nil {
			t.Errorf("%s compiles", source)
		}
	}
	for _, source := range []string{`active("a")`, `active("postura_2")`, `active("` + strings.Repeat("a", 40) + `")`} {
		if _, err := c.Compile(source, Bool); err != nil {
			t.Errorf("%s: %v", source, err)
		}
	}
}

func TestProgramListsTheStateNamesItReads(t *testing.T) {
	t.Parallel()
	c := newCompiler().WithStates()
	p, err := c.Compile(`active("b") and (active("a") or active("b")) and level() > 1`, Bool)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.ActiveNames(); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("ActiveNames = %v, want [a b]", got)
	}
	q, err := c.Compile(`level() > 1`, Bool)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.ActiveNames(); len(got) != 0 {
		t.Errorf("ActiveNames = %v, want none", got)
	}
}

func TestActiveWithoutAnEnvironmentFunctionIsAnErrorNotAPanic(t *testing.T) {
	t.Parallel()
	p, err := newCompiler().WithStates().Compile(`active("postura")`, Bool)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Bool(wizard3()); err == nil {
		t.Error("an environment with no Active function answers")
	}
}
