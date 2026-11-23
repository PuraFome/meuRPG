package play

import (
	"slices"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The line of the master's end of a casting names the targets of that casting, whatever else the scope ends:
// with two castings of one caster in a concentration group, each line holds only its own casting's targets.
func TestLastingRPCTheLineOfAGroupEndNamesTheTargetsOfItsCasting(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.rpcAdd(t, e, holdPerson, []string{"Goblin"}, a.rpcCaster(t, "Pensantus"))
	a.rpcAdd(t, e, "spell:bane", []string{"Toren", "Brisa"}, a.rpcCaster(t, "Pensantus"), func(r *playv1.AddLastingEffectRequest) { r.Concentration = new(false) })
	a.execSQL(t, `UPDATE combatant_states SET concentration = true WHERE source_key = 'spell:bane'`)
	if _, err := a.rpcEnd(t, a.master, e, a.rpcEffectID(t, holdPerson, "Goblin"), playv1.EffectEndScope_EFFECT_END_SCOPE_CONCENTRATION_GROUP); err != nil {
		t.Fatalf("EndLastingEffect() error = %v", err)
	}
	var targets []string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT COALESCE(ARRAY(SELECT jsonb_array_elements_text(payload->'lasting'->'targets')), '{}') FROM session_events WHERE kind = 'lasting_effect_ended' AND payload->'lasting'->>'key' = $1 AND payload->'lasting'->>'reason' = 'concentration' ORDER BY seq DESC LIMIT 1`, holdPerson).Scan(&targets); err != nil {
		t.Fatalf("read the line of the end: %v", err)
	}
	if want := []string{a.id(t, "Goblin")}; !slices.Equal(targets, want) {
		t.Errorf("the line of the end names %v, want only the casting's target %v", targets, want)
	}
}
