package play

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The save line of a spell's hit effect says what the failed save gave and for how long, and names the spell (#577): the
// master, the attacker's player and the target's player read it, no other player (RN-10).

const (
	untilNextTurnText = "até o fim do próximo turno dele"
	untilSpellEndText = "até o fim da magia"
)

// hitRiderEntries are the entries of the log that are an answered rider, as u reads them.
func (a *armed) hitRiderEntries(t *testing.T, u *user, e *playv1.Encounter) []*playv1.CombatLogEntry {
	t.Helper()
	var out []*playv1.CombatLogEntry
	for _, r := range a.log(t, u, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_HIT_RIDER {
				out = append(out, en)
			}
		}
	}
	return out
}

func TestAHitSpellSaveLineSaysTheConditionItsDurationAndTheSpell(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		edit     func(*tableMod)
		duration string
	}{
		"to the next turn": {func(m *tableMod) { m.Save, m.OnFailConditions = "wis", []string{frightenedKey} }, untilNextTurnText},
		"to the spell's end": {func(m *tableMod) {
			m.Save, m.OnFailConditions, m.ConditionDuration = "wis", []string{frightenedKey}, spellEnd
		}, untilSpellEndText},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := newHitCasters(t, nil, hitMod(trigEach, tc.edit))
			e := a.hitFight(t)
			a.castMark(t, e, 1)
			a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 19) // passes
			e = a.nextTurn(t, e)
			a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 4) // fails
			for who, u := range map[string]*user{"the master": a.master, "Ana": a.ana} {
				lines := a.hitRiderEntries(t, u, e)
				if len(lines) != 2 {
					t.Fatalf("%s reads %d save lines, want 2", who, len(lines))
				}
				for _, en := range lines {
					r := en.GetHitRider()
					if en.GetKeyNamePt() != "Marca de Nanquim" {
						t.Errorf("%s reads the spell %q, want Marca de Nanquim", who, en.GetKeyNamePt())
					}
					if r.GetResult() == playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED {
						if r.GetConditionNamePt() == "" || r.GetDurationText() != tc.duration {
							t.Errorf("%s reads the failed save %v, want the condition and %q", who, r, tc.duration)
						}
					} else if r.GetConditionNamePt() != "" || r.GetDurationText() != "" {
						t.Errorf("%s reads a condition on the save that passed: %v", who, r)
					}
				}
			}
		})
	}
}

// RN-10: the condition and its duration are on the save line, which only the master, the attacker's player and the
// target's player read; every other player reads no word of them. When the effect is its owner's, the target's player
// reads "Outra fonte" and no key; the master and the attacker's player read the spell.
func TestRN10_TheSpellSaveLineKeepsItsTextFromOtherPlayers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		visibility rulesv1.TableEffectAudience
		target     string
		spell      string // what the target's player reads of the spell
	}{
		{"an NPC's save", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC, "Capitão Goblin", ""},
		{"a public effect on a player", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC, "Toren", "Marca de Nanquim"},
		{"an effect of its owner on a player", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER, "Toren", otherSourceTx},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newHitCasters(t, func(s *rulesv1.TableSpell) { s.Effect.Visibility = tc.visibility }, hitMod(trigEach, func(m *tableMod) {
				m.Save, m.OnFailConditions, m.ConditionDuration = "wis", []string{frightenedKey}, spellEnd
			}))
			e := a.hitFight(t)
			a.castMark(t, e, 1)
			a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 4)
			if tc.target == "Toren" {
				e = a.nextTurn(t, e)
				a.rollDamage(t, a.strike(t, e, battleaxe, "Toren", d20(19)), 1, 1, 4)
			}
			canary := untilSpellEndText
			for who, u := range map[string]*user{"the master": a.master, "Ana": a.ana} {
				if !strings.Contains(a.readsOf(t, u, e, ""), canary) {
					t.Errorf("%s does not read the duration: the canary is unreachable", who)
				}
			}
			others := map[string]*user{"Bia": a.bia}
			if tc.target != "Toren" {
				others["Caio"] = a.caio
			}
			for who, u := range others {
				if text := a.readsOf(t, u, e, ""); strings.Contains(text, canary) {
					t.Errorf("%s reads the duration of a save that is not theirs", who)
				}
			}
			if tc.target != "Toren" {
				return
			}
			lines := a.hitRiderEntries(t, a.caio, e)
			if len(lines) != 1 || lines[0].GetKeyNamePt() != tc.spell || lines[0].GetHitRider().GetConditionNamePt() == "" {
				t.Fatalf("the target's player reads %v, want the spell %q and the condition", lines, tc.spell)
			}
			if got := lines[0].GetHitRider().GetDurationText(); tc.spell == otherSourceTx && (got == "" || strings.Contains(got, "magia")) {
				t.Errorf("the target's player reads the duration %q of a hidden effect, want neutral words that do not say it is a spell", got)
			}
			if got := lines[0].GetHitRider().GetDurationText(); tc.spell != otherSourceTx && got != untilSpellEndText {
				t.Errorf("the target's player reads the duration %q of a public effect, want %q", got, untilSpellEndText)
			}
			if tc.spell == otherSourceTx && lines[0].GetKey() != "" {
				t.Errorf("the target's player reads the key %q of a hidden effect", lines[0].GetKey())
			}
			if text := jsonOf(lines[0]); tc.spell == otherSourceTx && strings.Contains(text, "Marca de Nanquim") {
				t.Errorf("the target's player reads the name of a hidden spell")
			}
		})
	}
}
