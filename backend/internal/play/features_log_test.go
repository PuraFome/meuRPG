package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The line of a use of a feature in the combat log says what the use cost (#743 part 2, docs/architecture/play/feature-state.md
// "The log line"): the master and the user's player read the part of the turn, the resource and what is left of it; every other
// player reads the plain line. These tests need the database.

// usedLine is the only FEATURE_USED line u reads in the combat.
func (m *featureTable) usedLine(t *testing.T, u *user, e *playv1.Encounter) *playv1.CombatLogFeatureUsed {
	t.Helper()
	lines := logLinesOf(t, m.armed, u, e, playv1.CombatLogKind_COMBAT_LOG_KIND_FEATURE_USED)
	if len(lines) != 1 {
		t.Fatalf("a player reads %d FEATURE_USED lines, want 1", len(lines))
	}
	return lines[0].GetFeatureUsed()
}

// Each economy a feature declares is the line's economy: the action, the bonus action, the reaction, and none for a free one.
func TestTheLogLineOfAUseSaysTheEconomyItSpent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		feature string
		want    playv1.FeatureEconomy
	}{
		{"Postura", playv1.FeatureEconomy_FEATURE_ECONOMY_ACTION},
		{"Brado", playv1.FeatureEconomy_FEATURE_ECONOMY_BONUS_ACTION},
		{"Reflexo", playv1.FeatureEconomy_FEATURE_ECONOMY_REACTION},
		{"Marca", playv1.FeatureEconomy_FEATURE_ECONOMY_NONE},
	} {
		t.Run(tc.feature, func(t *testing.T) {
			t.Parallel()
			m := newFeatureTable(t)
			e := m.fight(t)
			m.mustUse(t, m.ana, e, "Pensantus", tc.feature, m.validArgs(t, tc.feature))
			for name, u := range map[string]*user{"the master": m.master, "Pensantus's player": m.ana} {
				if got := m.usedLine(t, u, e).GetEconomy(); got != tc.want {
					t.Errorf("%s reads the economy %v, want %v", name, got, tc.want)
				}
			}
		})
	}
}

// A use that spends a resource of its own says its name and the uses left of it: Chamado spends one of the two Chamados.
func TestTheLogLineOfAUseSaysTheResourceAndWhatIsLeft(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	m.mustUse(t, m.ana, e, "Pensantus", "Chamado", nil)
	for name, u := range map[string]*user{"the master": m.master, "Pensantus's player": m.ana} {
		line := m.usedLine(t, u, e)
		if line.GetResourceNamePt() != "Chamados" || line.GetResourceLeft() != 1 || line.GetResourceMax() != 2 {
			t.Errorf("%s reads the resource %q with %d of %d, want Chamados with 1 of 2", name, line.GetResourceNamePt(), line.GetResourceLeft(), line.GetResourceMax())
		}
	}
}

// A use limited by a scope only (Brado: 2 per combat, no resource of its own) names no resource and counts the scope: 1 of 2.
func TestTheLogLineOfAScopeLimitedUseCountsTheScope(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	m.h.roller.queue(3)
	m.mustUse(t, m.ana, e, "Pensantus", "Brado", m.validArgs(t, "Brado"))
	for name, u := range map[string]*user{"the master": m.master, "Pensantus's player": m.ana} {
		line := m.usedLine(t, u, e)
		if line.GetResourceNamePt() != "" || line.GetResourceLeft() != 1 || line.GetResourceMax() != 2 {
			t.Errorf("%s reads the resource %q with %d of %d, want none and 1 of 2", name, line.GetResourceNamePt(), line.GetResourceLeft(), line.GetResourceMax())
		}
	}
}

// A use with no limit at all leaves the count at 0 of 0, so the line has no parenthesis.
func TestTheLogLineOfAUseWithNoLimitHasNoCount(t *testing.T) {
	t.Parallel()
	alerta := grantFeature("Alerta", "reaction", "", useOf(rulesv1.UseScope_USE_SCOPE_UNSPECIFIED, "", nil, &rulesv1.FeatureApplies{TempHp: "4"}))
	m := newFeatureTableOf(t, []*rulesv1.TableFeature{alerta})
	e := m.fight(t)
	m.mustUse(t, m.ana, e, "Pensantus", "Alerta", nil)
	if line := m.usedLine(t, m.ana, e); line.GetResourceNamePt() != "" || line.GetResourceLeft() != 0 || line.GetResourceMax() != 0 {
		t.Errorf("the line counts %q %d of %d, want no count", line.GetResourceNamePt(), line.GetResourceLeft(), line.GetResourceMax())
	}
}

// The plain line is what the other players read: the feature, no cost.
func TestTheLogLineOfAUseIsPlainForAnotherPlayer(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	m.mustUse(t, m.ana, e, "Pensantus", "Chamado", nil)
	line := m.usedLine(t, m.caio, e)
	if line.GetNamePt() != "Chamado" {
		t.Fatalf("another player reads the feature %q, want Chamado", line.GetNamePt())
	}
	if line.GetEconomy() != playv1.FeatureEconomy_FEATURE_ECONOMY_UNSPECIFIED || line.GetResourceNamePt() != "" || line.GetResourceLeft() != 0 || line.GetResourceMax() != 0 {
		t.Errorf("another player reads the economy %v and the resource %q with %d of %d, want none of it", line.GetEconomy(), line.GetResourceNamePt(), line.GetResourceLeft(), line.GetResourceMax())
	}
}

// The use that answers a FEATURE window is the same use: the line says the reaction.
func TestTheLogLineOfAUseAnsweringTheFeatureWindowSaysTheReaction(t *testing.T) {
	t.Parallel()
	m := haloTable(t)
	e, w := m.haloWindow(t)
	if _, err := m.answerReaction(t, m.ana, e, w.GetId(), useAnswer(0)); err != nil {
		t.Fatalf("AnswerReaction(USE) error = %v", err)
	}
	for name, u := range map[string]*user{"the master": m.master, "Pensantus's player": m.ana} {
		if got := m.usedLine(t, u, e).GetEconomy(); got != playv1.FeatureEconomy_FEATURE_ECONOMY_REACTION {
			t.Errorf("%s reads the economy %v, want the reaction", name, got)
		}
	}
	if got := m.usedLine(t, m.caio, e).GetEconomy(); got != playv1.FeatureEconomy_FEATURE_ECONOMY_UNSPECIFIED {
		t.Errorf("another player reads the economy %v, want none", got)
	}
}
