package play

import (
	"testing"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Guard for the hints of a retried AddLastingEffect: the retry tells the streams again, and its
// log hint is secret when the first call's line was (an effect the players cannot see), so a retry
// does not tell the players there is a new line they cannot read.
func TestAddLastingEffectRetryKeepsTheLogHintSecret(t *testing.T) {
	t.Parallel()
	const (
		bothHeard   = "master[EncounterChanged,CombatLogChanged] caio[EncounterChanged,CombatLogChanged] ana[EncounterChanged,CombatLogChanged] bia[EncounterChanged,CombatLogChanged]"
		masterAlone = "master[EncounterChanged,CombatLogChanged] caio[EncounterChanged] ana[EncounterChanged] bia[EncounterChanged]"
	)
	cases := []struct {
		name       string
		catalogKey string
		hidden     bool // the Goblin is hidden
		hideEffect bool // the master keeps the effect from the players
		want       string
	}{
		{"a condition the master keeps from the players", "condition:blinded", false, true, masterAlone},
		{"a master-only effect on a hidden NPC", holdPerson, true, true, masterAlone},
		{"a public spell on a visible NPC", holdPerson, false, false, bothHeard},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newEffectsParty(t)
			e := a.closeFight(t)
			if tc.hidden {
				a.execSQL(t, `UPDATE combatants SET hidden = true WHERE id = $1`, a.id(t, "Goblin"))
			}
			h := a.listen(t)
			key := newKey()
			edit := func(r *playv1.AddLastingEffectRequest) {
				r.IdempotencyKey, r.CatalogKey, r.TargetIds = key, tc.catalogKey, []string{a.id(t, "Goblin")}
				if tc.hideEffect {
					r.PlayerVisible = new(false)
				}
				if tc.catalogKey == holdPerson {
					r.CasterId = a.id(t, "Pensantus")
				}
			}

			first, err := a.addRaw(t, a.master, e, edit)
			if err != nil {
				t.Fatalf("AddLastingEffect() error = %v", err)
			}
			if got := a.heard(t, h); got != tc.want {
				t.Errorf("the first call told %s, want %s", got, tc.want)
			}

			again, err := a.addRaw(t, a.master, e, edit)
			if err != nil {
				t.Fatalf("the same call again: error = %v", err)
			}
			if got := a.heard(t, h); got != tc.want {
				t.Errorf("the retry told %s, want %s", got, tc.want)
			}
			if !proto.Equal(first, again) {
				t.Errorf("the retry answered %v, want the first answer %v", again, first)
			}
		})
	}
}
