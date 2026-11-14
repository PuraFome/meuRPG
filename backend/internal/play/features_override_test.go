package play

import (
	"testing"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The master's economy override in the options (#743 part 2): UseFeature lets the master use a feature whose economy the turn
// spent, and the row says so, in the master's own read only. These tests need the database.

// Brado is a bonus action with two uses in the combat: after one use the bonus action is spent and a use is left. The master reads
// the row marked, a player reads it disabled and not marked, and an enabled row is not marked for anybody.
func TestTheMastersRowBlockedByTheEconomyAloneMayBeOverridden(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	m.h.roller.queue(3)
	m.mustUse(t, m.ana, e, "Pensantus", "Brado", m.validArgs(t, "Brado"))
	if row := m.row(t, m.master, e, "Pensantus", "Brado"); row.GetEnabled() || !row.GetMasterMayOverride() {
		t.Errorf("the master's row of Brado with the bonus action spent = enabled %v, override %v, want disabled and overridable", row.GetEnabled(), row.GetMasterMayOverride())
	}
	if row := m.row(t, m.ana, e, "Pensantus", "Brado"); row.GetEnabled() || row.GetMasterMayOverride() {
		t.Errorf("Pensantus's player's row of Brado = enabled %v, override %v, want disabled and not overridable", row.GetEnabled(), row.GetMasterMayOverride())
	}
	for name, u := range map[string]*user{"the master": m.master, "Pensantus's player": m.ana} {
		if row := m.row(t, u, e, "Pensantus", "Postura"); !row.GetEnabled() || row.GetMasterMayOverride() {
			t.Errorf("%s reads the row of Postura = enabled %v, override %v, want enabled and not marked", name, row.GetEnabled(), row.GetMasterMayOverride())
		}
	}
}

// A reaction the combatant spent is the economy too: Alerta is a reaction with no limit in a scope, so nothing but the reaction
// stops it.
func TestTheMastersRowOfASpentReactionMayBeOverridden(t *testing.T) {
	t.Parallel()
	alerta := grantFeature("Alerta", "reaction", "", useOf(rulesv1.UseScope_USE_SCOPE_UNSPECIFIED, "", nil, &rulesv1.FeatureApplies{TempHp: "4"}))
	m := newFeatureTableOf(t, []*rulesv1.TableFeature{alerta})
	e := m.passTo(t, m.fight(t), "Toren")
	m.mustUse(t, m.ana, e, "Pensantus", "Alerta", nil)
	if row := m.row(t, m.master, e, "Pensantus", "Alerta"); row.GetEnabled() || !row.GetMasterMayOverride() {
		t.Errorf("the master's row of Alerta with the reaction spent = enabled %v, override %v, want disabled and overridable", row.GetEnabled(), row.GetMasterMayOverride())
	}
	if row := m.row(t, m.ana, e, "Pensantus", "Alerta"); row.GetMasterMayOverride() {
		t.Error("Pensantus's player's row of Alerta is overridable")
	}
	if _, err := m.use(t, m.master, e, "Pensantus", "Alerta", nil); err != nil {
		t.Errorf("the master's second Alerta, the row said overridable: %v", err)
	}
}

// What is not the economy is not the master's to override: the uses of the scope spent, another creature's turn, and a question
// that waits for him. His row is disabled and not marked.
func TestTheMastersRowBlockedForAnotherReasonIsNotOverridable(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	m.h.roller.queue(3, 3)
	m.mustUse(t, m.master, e, "Pensantus", "Brado", m.validArgs(t, "Brado"))
	m.mustUse(t, m.master, e, "Pensantus", "Brado", m.validArgs(t, "Brado")) // his override: the bonus action is spent, the second use is not
	if row := m.row(t, m.master, e, "Pensantus", "Brado"); row.GetEnabled() || row.GetMasterMayOverride() {
		t.Errorf("the master's row of Brado with the uses spent = enabled %v (%v), override %v, want disabled and not overridable", row.GetEnabled(), row.GetReason().GetCode(), row.GetMasterMayOverride())
	}
	e = m.passTo(t, e, "Toren")
	if row := m.row(t, m.master, e, "Pensantus", "Chamado"); row.GetEnabled() || row.GetMasterMayOverride() {
		t.Errorf("the master's row of Chamado on Toren's turn = enabled %v (%v), override %v, want disabled and not overridable", row.GetEnabled(), row.GetReason().GetCode(), row.GetMasterMayOverride())
	}
}

// While a question waits for the master, a spent economy does not make the row overridable: nothing is used.
func TestTheMastersRowWhileAQuestionWaitsIsNotOverridable(t *testing.T) {
	t.Parallel()
	m := newFeatureTable(t)
	e := m.fight(t)
	m.h.roller.queue(3)
	m.mustUse(t, m.ana, e, "Pensantus", "Brado", m.validArgs(t, "Brado"))
	m.execSQL(t, `INSERT INTO hidden_reveals (encounter_id, caster_id, spell_key, combatant_ids, origin_col, origin_row, seq, created_at)
		SELECT encounter_id, id, 'spell:fire-bolt', ARRAY[]::TEXT[], 4, 3, 1, now() FROM combatants WHERE label = 'Pensantus'`)
	if row := m.row(t, m.master, e, "Pensantus", "Brado"); row.GetEnabled() || row.GetMasterMayOverride() {
		t.Errorf("the master's row of Brado while a question waits = enabled %v, override %v, want disabled and not overridable", row.GetEnabled(), row.GetMasterMayOverride())
	}
}
