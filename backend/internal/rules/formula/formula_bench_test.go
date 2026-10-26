package formula

import (
	"strconv"
	"strings"
	"testing"
)

// worstFormulas are the biggest formulas of each shape the limits take: each is
// grown term by term until the next one would be refused (by the bytes or the
// nodes), so the same benchmark measures whatever the limits are.
//
//   - stepped-long-key: the sum of steps of a class table, with the longest class key
//   - stepped-short-key: the same with a one-letter key, which is the most nodes the bytes allow
//   - sum-of-ones: the most operators and numbers (two bytes a node)
//   - nested-min: the deepest tree, a call inside a call
//   - ternary-chain: a table written as a chain of conditions
func worstFormulas(t testing.TB) map[string]string {
	t.Helper()
	grow := func(class string, build func(n int) string) string {
		c := NewCompiler([]string{class})
		best := ""
		for n := 1; n < 2000; n++ {
			s := build(n)
			if _, err := c.Compile(s, Int); err != nil {
				break
			}
			best = s
		}
		if best == "" {
			t.Fatal("not even one term fits")
		}
		return best
	}
	stepped := func(class string) func(int) string {
		return func(n int) string {
			steps := make([]int, n)
			for i := range steps {
				steps[i] = i % 19
			}
			return steppedFormula(class, steps)
		}
	}
	return map[string]string{
		"stepped-long-key":  grow(longClass, stepped(longClass)),
		"stepped-short-key": grow("x", stepped("x")),
		"sum-of-ones":       grow("x", func(n int) string { return strings.Repeat("1+", n) + "1" }),
		"nested-min": grow("x", func(n int) string {
			return strings.Repeat("min(1, ", n) + `classLevel("x")` + strings.Repeat(")", n)
		}),
		"ternary-chain": grow("x", func(n int) string {
			var b strings.Builder
			for i := range n {
				b.WriteString(`classLevel("x") >= ` + strconv.Itoa(20-i%20) + " ? " + strconv.Itoa(i%7) + " : ")
			}
			return b.String() + "0"
		}),
	}
}

// BenchmarkCompileWorst compiles the biggest formulas the limits take. Run it
// with `go test ./internal/rules/formula -run '^$' -bench Worst -benchmem`
// (in backend/, without -race).
func BenchmarkCompileWorst(b *testing.B) {
	for name, source := range worstFormulas(b) {
		c := NewCompiler([]string{"x", longClass})
		b.Run(name+"-"+strconv.Itoa(len(source))+"B", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := c.Compile(source, Int); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkEvalWorst evaluates them: what a sheet read pays for each resource.
func BenchmarkEvalWorst(b *testing.B) {
	for name, source := range worstFormulas(b) {
		p, err := NewCompiler([]string{"x", longClass}).Compile(source, Int)
		if err != nil {
			b.Fatal(err)
		}
		env := wizard3()
		env.ClassLevel = func(string) int { return 9 }
		b.Run(name+"-"+strconv.Itoa(len(source))+"B", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := p.Int(env); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
