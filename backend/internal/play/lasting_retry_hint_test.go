package play

import (
	"testing"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Guard for the hints of a retried AddLastingEffect: the retry reaches the audience the first call reached. Its
// log hint is secret when the first call's line was (an effect the players cannot see), so a retry
// does not tell the players there is a new line they cannot read. Besides the combat and its log, the retry
// tells the master what the stored change does not say it did not tell (the replay rule of Service.tx): his casts
// and the vitals of the combat's players. A change only the master reads is told to him alone again, and so is one whose
// stored event cannot be read: what the retry cannot tell is for the master to read.
func TestAddLastingEffectRetryKeepsTheLogHintSecret(t *testing.T) {
	t.Parallel()
	const (
		bothHeard   = "master[EncounterChanged,CombatLogChanged] caio[EncounterChanged,CombatLogChanged] ana[EncounterChanged,CombatLogChanged] bia[EncounterChanged,CombatLogChanged]"
		masterAlone = "master[EncounterChanged,CombatLogChanged] caio[EncounterChanged] ana[EncounterChanged] bia[EncounterChanged]"
		// The retry: the replayed hints come first, the combat and its log after, as the handler tells them.
		bothAgain        = "master[SpellCastsChanged,VitalsChanged,VitalsChanged,VitalsChanged,EncounterChanged,CombatLogChanged] caio[VitalsChanged,EncounterChanged,CombatLogChanged] ana[VitalsChanged,EncounterChanged,CombatLogChanged] bia[VitalsChanged,EncounterChanged,CombatLogChanged]"
		masterAloneAgain = "master[SpellCastsChanged,VitalsChanged,VitalsChanged,VitalsChanged,EncounterChanged,CombatLogChanged] caio[] ana[] bia[]"
	)
	cases := []struct {
		name       string
		catalogKey string
		hidden     bool // the Goblin is hidden
		hideEffect bool // the master keeps the effect from the players
		want       string
		again      string // what the retry tells
		unreadable bool   // the stored event of the first call cannot be read when the retry comes
	}{
		{"a condition the master keeps from the players", "condition:blinded", false, true, masterAlone, masterAloneAgain, false},
		{"a master-only effect on a hidden NPC", holdPerson, true, true, masterAlone, masterAloneAgain, false},
		{"a public spell on a visible NPC", holdPerson, false, false, bothHeard, bothAgain, false},
		{"a public spell whose stored event cannot be read", holdPerson, false, false, bothHeard, masterAloneAgain, true},
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

			if tc.unreadable {
				// A field of the wrong type: the event is still an object, but readEvent refuses it.
				a.execSQL(t, `UPDATE session_events SET payload = payload || '{"secret": "unreadable"}'::jsonb WHERE idempotency_key = $1`, key)
			}
			again, err := a.addRaw(t, a.master, e, edit)
			if err != nil {
				t.Fatalf("the same call again: error = %v", err)
			}
			if got := a.heard(t, h); got != tc.again {
				t.Errorf("the retry told %s, want %s", got, tc.again)
			}
			if !tc.unreadable && !proto.Equal(first, again) { // an event that cannot be read has no cards to answer with
				t.Errorf("the retry answered %v, want the first answer %v", again, first)
			}
		})
	}
}
