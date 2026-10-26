package formula

import (
	"strconv"
	"strings"
	"testing"
)

// A table class whose resource follows an irregular level table has no
// conditionals but the ternary and min/max, so the table is a sum of steps:
// one term is 1 once the class level passes its threshold. These are the
// formulas of such a class (an invented key of 60 characters, the longest slug).

const longClass = "classe-inventada-com-um-nome-muito-comprido-para-a-tabela-60"

// steppedFormula is the sum of one step for each threshold.
func steppedFormula(class string, thresholds []int) string {
	terms := make([]string, len(thresholds))
	for i, k := range thresholds {
		terms[i] = `min(1, max(0, classLevel("` + class + `") - ` + strconv.Itoa(k) + `))`
	}
	return strings.Join(terms, " + ")
}

// tenSteps are thresholds of an irregular table: 1 at level 2, 2 at level 3...
var tenSteps = []int{1, 2, 4, 5, 8, 10, 13, 14, 16, 18}

// compileWith compiles source for a content whose only class is class.
func compileWith(t testing.TB, class, source string) (*Program, error) {
	t.Helper()
	return NewCompiler([]string{class}).Compile(source, Int)
}

func TestSteppedFormulaOfAClassTable(t *testing.T) {
	t.Parallel()
	source := steppedFormula(longClass, tenSteps)
	if len(source) <= 256 || len(source) > MaxSourceBytes {
		t.Fatalf("the formula is %d bytes, want over 256 and within the limit of %d", len(source), MaxSourceBytes)
	}
	p, err := compileWith(t, longClass, source)
	if err != nil {
		t.Fatalf("Compile() error = %v (%d bytes)", err, len(source))
	}
	for level := 1; level <= 20; level++ {
		env := wizard3()
		env.ClassLevel = func(string) int { return level }
		want := 0
		for _, k := range tenSteps {
			if level > k {
				want++
			}
		}
		if got, err := p.Int(env); err != nil || got != want {
			t.Errorf("level %d: Int() = %d, %v; want %d", level, got, err, want)
		}
	}
}

// The limits are still limits: a formula over the bytes, and one over the
// nodes within the bytes, are refused.
func TestFormulaLimitsHold(t *testing.T) {
	t.Parallel()
	c := newCompiler()
	over := strings.Repeat("1 + ", MaxSourceBytes/4) + "1"
	if len(over) <= MaxSourceBytes {
		over += strings.Repeat(" ", MaxSourceBytes-len(over)+1)
	}
	if _, err := c.Compile(over, Int); err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Errorf("a formula of %d bytes: error = %v, want the byte limit", len(over), err)
	}
	manyNodes := strings.Repeat("1+", MaxNodes) + "1" // MaxNodes+1 numbers and MaxNodes sums, well under the bytes
	if len(manyNodes) > MaxSourceBytes {
		t.Fatalf("test formula is %d bytes, want within the limit", len(manyNodes))
	}
	if _, err := c.Compile(manyNodes, Int); err == nil {
		t.Errorf("a formula of %d nodes was accepted, want the node limit", 2*MaxNodes+1)
	}
}
