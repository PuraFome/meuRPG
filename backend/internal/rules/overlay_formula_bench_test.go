package rules

import (
	"strconv"
	"strings"
	"testing"
)

// bigFormulaOverlay is the worst the formula budgets allow in bytes: 300
// backgrounds with 6,000 effects whose formulas are the sum of steps of a class
// table of about 950 bytes, `formulas` of them distinct.
func bigFormulaOverlay(formulas int) Overlay {
	_, _, bg := tableMisc()
	o := Overlay{Revision: 3}
	n := 0
	for i := range MaxOverlayEffects / MaxFeatureEffects {
		b := bg
		b.Key = "background:antecedente-" + strconv.Itoa(i) + tableSuffix
		fx := make([]Effect, 0, MaxFeatureEffects)
		for range MaxFeatureEffects {
			d := n % formulas
			terms := make([]string, 0, 24)
			for j := 0; len(strings.Join(terms, " + ")) < 900; j++ {
				terms = append(terms, `min(1, max(0, classLevel("wizard") - `+strconv.Itoa((d+j)%19)+"))")
			}
			terms = append(terms, strconv.Itoa(d+1))
			fx = append(fx, Effect{Type: "modifier", Target: "ac", Mode: "add", Value: strings.Join(terms, " + ")})
			n++
		}
		b.Feature = TableFeature{Key: "background-feature:antecedente-" + strconv.Itoa(i) + tableSuffix, NamePT: "F", Effects: fx}
		o.Backgrounds = append(o.Backgrounds, b)
	}
	return o
}

// BenchmarkWithBigFormulas measures a With of 6,000 effects with formulas of
// about 950 bytes, 100 and 500 of them distinct (the budget of distinct formulas).
func BenchmarkWithBigFormulas(b *testing.B) {
	srd := loadForTest(b)
	for _, formulas := range []int{100, MaxOverlayFormulas} {
		o := bigFormulaOverlay(formulas)
		if _, err := srd.With(o); err != nil {
			b.Fatal(err)
		}
		b.Run(strconv.Itoa(formulas)+"f", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := srd.With(o); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
