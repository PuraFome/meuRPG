package play

import (
	"testing"
)

// featureStateOf reads the feature_state of a combatant.
func (a *armed) featureStateOf(t *testing.T, label string) featureState {
	t.Helper()
	var raw []byte
	if err := a.h.pool.QueryRow(t.Context(), `SELECT feature_state FROM combatants WHERE id = $1`, a.id(t, label)).Scan(&raw); err != nil {
		t.Fatalf("read the feature state of %s: %v", label, err)
	}
	return readFeatureState(raw)
}

func (a *armed) writeFeatureState(t *testing.T, label string, fs featureState) {
	t.Helper()
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE combatants SET feature_state = $2::jsonb WHERE id = $1`, a.id(t, label), string(fs.bytes())); err != nil {
		t.Fatalf("write the feature state of %s: %v", label, err)
	}
}

// TestUndoOfAnEventWithOnceBeforePutsTheOldMarksBack: a damage roll written before the feature state kept the marks of the
// attacker in once_before (Sneak Attack, Colossus Slayer and the table's marks); its undo puts them back as entries of
// the turn, and the marks the old table before did not hold go.
func TestUndoOfAnEventWithOnceBeforePutsTheOldMarksBack(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	turn := extrasTurnKey(e)
	used := featureEntry{S: scopeTurn, At: turn, N: 1}
	for _, tt := range []struct {
		name string
		ev   onceMarks
		want featureState
	}{
		{"the old marks all back", onceMarks{Colossus: turn, Table: map[string]string{"giant": turn}, TableChanged: true},
			featureState{featureColossusSlayer: used, "giant": used}},
		{"no table change leaves the table's marks", onceMarks{},
			featureState{"giant": used, "ember": used}},
		{"an empty table before takes the table's marks away", onceMarks{Sneak: turn, Table: map[string]string{}, TableChanged: true},
			featureState{featureSneakAttack: used}},
	} {
		a.writeFeatureState(t, "Toren", featureState{featureSneakAttack: used, "giant": used, "ember": used})
		ev := tt.ev
		if _, err := a.takeBackAs(t, e, eventDamageRolled, actionEvent{Actor: a.id(t, "Toren"), Target: a.id(t, "Goblin"), Pending: newKey(), OnceBefore: &ev}); err != nil {
			t.Fatalf("%s: takeBack() error = %v", tt.name, err)
		}
		got := a.featureStateOf(t, "Toren")
		if len(got) != len(tt.want) {
			t.Errorf("%s: feature_state = %v, want %v", tt.name, got, tt.want)
			continue
		}
		for k, w := range tt.want {
			if got[k] != w {
				t.Errorf("%s: feature_state[%s] = %v, want %v", tt.name, k, got[k], w)
			}
		}
	}
}

// TestUndoPutsBackTheEntryOfEachKeyTheRollMarked: a nil entry in feature_before is a key the roll found absent (it goes), an
// entry is put back as it was.
func TestUndoPutsBackTheEntryOfEachKeyTheRollMarked(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	turn := extrasTurnKey(e)
	used := featureEntry{S: scopeTurn, At: turn, N: 1}
	before := featureEntry{S: scopeTurn, At: turn, N: 1, C: "fire"}
	a.writeFeatureState(t, "Toren", featureState{featureSneakAttack: used, "giant": used, "other": {S: scopeCombat, N: 2}})
	ev := actionEvent{Actor: a.id(t, "Toren"), Target: a.id(t, "Goblin"), Pending: newKey(), FeatureBefore: map[string]*featureEntry{featureSneakAttack: nil, "giant": &before}}
	if _, err := a.takeBackAs(t, e, eventDamageRolled, ev); err != nil {
		t.Fatalf("takeBack() error = %v", err)
	}
	got := a.featureStateOf(t, "Toren")
	if _, ok := got[featureSneakAttack]; ok || got["giant"] != before || got["other"] != (featureEntry{S: scopeCombat, N: 2}) || len(got) != 2 {
		t.Errorf("feature_state = %v, want giant put back, sneak_attack gone and the other key alone", got)
	}
}
