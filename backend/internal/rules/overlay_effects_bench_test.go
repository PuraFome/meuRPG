package rules

import (
	"os"
	"runtime"
	"strconv"
	"testing"
)

// effectMix is the idx-th effect of a book's mix: modifiers (whose formulas
// come from a pool of `formulas` distinct texts), proficiencies, resistances,
// notes, choices and resources, each one different from the next.
func effectMix(idx, formulas int) Effect {
	switch idx % 6 {
	case 0:
		return Effect{Type: "modifier", Target: []string{"ac", "initiative", "speed.walk", "save.con"}[idx/6%4], Mode: "add", Value: `max(1, mod("str") + ` + strconv.Itoa(idx/6%formulas+1) + ")"}
	case 1:
		return Effect{Type: "proficiency", Proficiency: []string{"skill:survival", "skill:arcana", "skill:history", "skill:nature"}[idx/6%4]}
	case 2:
		return Effect{Type: "resistance", Except: "damage-type:psychic", While: ResistanceWhileRage}
	case 3:
		return Effect{Type: "note", TextPT: "Nota do livro " + strconv.Itoa(idx) + ": o que esta característica faz na mesa."}
	case 4:
		return Effect{Type: "choice", Choice: "skill", Count: 1}
	default:
		return Effect{Type: "resource", Resource: "recurso_" + strconv.Itoa(idx), Max: "prof()", Recharge: "short_rest"}
	}
}

// budgetEffects counts the effects the budget counts in o.
func budgetEffects(t testing.TB, o Overlay) int {
	t.Helper()
	bg := &budget{formulas: map[string]bool{}}
	for _, part := range []func(*Overlay) error{bg.classes, bg.subclasses, bg.races, bg.subraces, bg.backgrounds, bg.feats} {
		if err := part(&o); err != nil {
			t.Fatal(err)
		}
	}
	return bg.effects
}

// bookOverlayEffects is bookOverlay with effects added to the features of its
// subclasses, races, backgrounds and feats until the overlay carries `effects`
// of them: some features get few, some many (up to the 20 a feature may have),
// with `formulas` distinct formula texts among them.
func bookOverlayEffects(t testing.TB, c *Content, n, effects, formulas int) Overlay {
	t.Helper()
	o := bookOverlay(t, c, n)
	total := budgetEffects(t, o)
	var targets []*[]Effect
	for i := range o.Subclasses {
		for l := range o.Subclasses[i].Levels {
			for f := range o.Subclasses[i].Levels[l].Features {
				targets = append(targets, &o.Subclasses[i].Levels[l].Features[f].Effects)
			}
		}
	}
	for i := range o.Races {
		for f := range o.Races[i].Traits {
			targets = append(targets, &o.Races[i].Traits[f].Effects)
		}
	}
	for i := range o.Backgrounds {
		targets = append(targets, &o.Backgrounds[i].Feature.Effects)
	}
	for i := range o.Feats {
		targets = append(targets, &o.Feats[i].Effects)
	}
	for pass := 0; total < effects && pass < MaxFeatureEffects; pass++ {
		for ti, e := range targets {
			// A target takes effects in 1 + ti%5 groups of three: the features are not alike.
			if total >= effects || pass >= 3*(1+ti%5) || len(*e) >= MaxFeatureEffects {
				continue
			}
			*e = append(*e, effectMix(total+ti, formulas))
			total++
		}
	}
	if total < effects {
		t.Fatalf("only %d of %d effects fit in the targets", total, effects)
	}
	return o
}

// TestBookOverlayEffectsIsValid: the overlays the effect benchmarks run are
// valid and have the effects they say.
func TestBookOverlayEffectsIsValid(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	for _, tc := range fxBenchCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := bookOverlayEffects(t, srd, 1000, tc.effects, tc.formulas)
			if got := budgetEffects(t, o); got != tc.effects {
				t.Fatalf("%d effects, want %d", got, tc.effects)
			}
			if _, err := srd.With(o); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// fxBenchCases are the overlays the effect benchmarks run: 1,000 entries with
// the effects the books carry, up to the budget, and with many distinct
// formulas.
var fxBenchCases = []struct {
	name              string
	effects, formulas int
}{{"2000fx-100f", 2000, 100}, {"4000fx-100f", 4000, 100}, {"6000fx-100f", 6000, 100}, {"6000fx-480f", 6000, 480}}

// BenchmarkWithEffects measures one With of 1,000 entries at each effect count.
// Run it with
// `go test -run '^$' -bench 'BenchmarkWithEffects' -benchmem -benchtime 2s -count 5 ./internal/rules`
// (in backend/, without -race).
func BenchmarkWithEffects(b *testing.B) {
	srd := loadForTest(b)
	for _, tc := range fxBenchCases {
		o := bookOverlayEffects(b, srd, 1000, tc.effects, tc.formulas)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := srd.With(o); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestWithMemoryByEffects measures what a With of 1,000 entries keeps alive at
// each effect count, with MEURPG_MEASURE=1 and -v.
func TestWithMemoryByEffects(t *testing.T) {
	if os.Getenv("MEURPG_MEASURE") == "" {
		t.Skip("set MEURPG_MEASURE=1 to measure")
	}
	srd := loadForTest(t)
	heap := func() uint64 {
		runtime.GC()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	const copies = 4
	for _, tc := range fxBenchCases {
		o := bookOverlayEffects(t, srd, 1000, tc.effects, tc.formulas)
		keep := make([]*Content, 0, copies)
		before := heap()
		for range copies {
			c, err := srd.With(o)
			if err != nil {
				t.Fatal(err)
			}
			keep = append(keep, c)
		}
		after := heap()
		t.Logf("1000 entries, %s: a content keeps %.2f MB", tc.name, float64(after-before)/copies/1e6)
		runtime.KeepAlive(keep)
	}
}
