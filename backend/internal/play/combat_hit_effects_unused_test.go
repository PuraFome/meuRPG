package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The ended line of a hit effect says whether its use was still waiting (#577): CombatLogEffect.unused, kept in the
// line's event, read by whoever reads the line.

// endsHow is how the effect of the case ends.
type endsHow int

const (
	byConcentration endsHow = iota
	byDismissal
)

func TestTheEndedLineOfAHitEffectSaysWhenItEndedUnused(t *testing.T) {
	t.Parallel()
	noEffect := &tableMod{Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ATTACK_ADVANTAGE}
	for _, tc := range []struct {
		name  string
		shape func(*rulesv1.TableSpell)
		mod   *tableMod
		cast  []string
		hit   bool // a hit spends the effect before it ends
		ends  endsHow
		want  bool
	}{
		{"next_hit, concentration ends before any hit", nil, hitMod(trigNext), nil, false, byConcentration, true},
		{"next_hit already spent by a hit", nil, hitMod(trigNext), nil, true, byConcentration, false},
		{"next_melee_hit unused", nil, hitMod(trigMelee), nil, false, byConcentration, true},
		{"a non-hit effect", nil, noEffect, nil, false, byConcentration, false},
		{"each_hit unused", nil, hitMod(trigEach), nil, false, byConcentration, true},
		{"marked_target unused", markShape, hitMod(trigMarked), []string{"Goblin"}, false, byConcentration, true},
		{"next_hit ended by the master unused", nil, hitMod(trigNext), nil, false, byDismissal, true},
		{"next_hit ended by the master after a hit", nil, hitMod(trigNext), nil, true, byDismissal, false},
		{"a non-hit effect ended by the master", nil, noEffect, nil, false, byDismissal, false},
	} {
		for _, vis := range []struct {
			name       string
			visibility rulesv1.TableEffectAudience
			othersRead bool
		}{
			{"public", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC, true},
			{"owner", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER, false},
		} {
			t.Run(tc.name+", "+vis.name, func(t *testing.T) {
				t.Parallel()
				a := newHitCasters(t, func(s *rulesv1.TableSpell) {
					if tc.shape != nil {
						tc.shape(s)
					}
					s.Effect.Visibility = vis.visibility
				}, tc.mod)
				e := a.hitFight(t)
				a.castMark(t, e, 1, tc.cast...)
				if tc.hit {
					target := "Capitão Goblin"
					if len(tc.cast) > 0 {
						target = tc.cast[0]
					}
					a.rollDamage(t, a.strike(t, e, battleaxe, target, d20(19)), 3, 3)
				}
				switch tc.ends {
				case byConcentration:
					a.dropConcentration(t, e)
				case byDismissal:
					if err := a.coreEndEffect(t, e, "Pensantus", tableMark, playv1.EffectEndScope_EFFECT_END_SCOPE_THIS); err != nil {
						t.Fatalf("EndLastingEffect() error = %v", err)
					}
				}
				// An effect for its owner alone is read by the players who own a target of the line: the mark's
				// target is not Ana's.
				anaReads := vis.visibility != rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER || len(tc.cast) == 0
				for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
					en := a.endedLine(t, u, e)
					if en == nil {
						if name == "Ana" && !anaReads {
							continue
						}
						t.Fatalf("%s reads no ending line", name)
					}
					if got := en.GetEffect().GetUnused(); got != tc.want {
						t.Errorf("%s reads unused = %v, want %v", name, got, tc.want)
					}
				}
				if tc.ends == byDismissal {
					return // the master's dismissal also writes the action's own line, which has no targets to follow
				}
				if got := a.endedLine(t, a.caio, e) != nil; got != vis.othersRead {
					t.Errorf("another player reads the ending line = %v, want %v", got, vis.othersRead)
				}
				if en := a.endedLine(t, a.caio, e); en != nil && en.GetEffect().GetUnused() != tc.want {
					t.Errorf("another player reads unused = %v, want %v", en.GetEffect().GetUnused(), tc.want)
				}
			})
		}
	}
}
