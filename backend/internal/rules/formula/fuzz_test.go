package formula

import (
	"testing"
)

// FuzzCompileFormula feeds random text to Compile and runs whatever
// compiles. Homebrew content will be untrusted input (ADR-0008), so no
// formula may panic, hang or return a number out of range.
//
// Run it with: cd backend && go test ./internal/rules/formula -run '^$' -fuzz FuzzCompileFormula -fuzztime 30s
func FuzzCompileFormula(f *testing.F) {
	for _, seed := range []string{
		`8 + prof() + mod("int")`,
		`ceil(classLevel("wizard") / 2)`,
		`max(1, mod("wis") + classLevel("wizard"))`,
		`armor() == "none" and not shield()`,
		`classLevel("monk") >= 18 ? 30 : classLevel("monk") >= 14 ? 25 : 10`,
		`level() % classLevel("monk")`,
		`floor(level() / 0)`,
		`1..1000000`,
		`map(1..10, #)`,
		`let x = 1; x`,
		`"a" matches "b"`,
		`min(max(1, 2), floor(3.5))`,
		`-(-(-level()))`,
	} {
		f.Add(seed)
	}
	c := newCompiler()
	env := wizard3()
	f.Fuzz(func(t *testing.T, source string) {
		for _, kind := range []Kind{Int, Bool} {
			p, err := c.Compile(source, kind)
			if err != nil {
				continue
			}
			if kind == Int {
				n, err := p.Int(env)
				if err == nil && (n > MaxResult || n < -MaxResult) {
					t.Fatalf("Int(%q) = %d, out of range", source, n)
				}
				continue
			}
			_, _ = p.Bool(env)
		}
	})
}
