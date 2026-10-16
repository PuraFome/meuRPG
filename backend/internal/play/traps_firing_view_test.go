package play

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Characterization of firingProto (MR-035, RN-10, RN-20): what each kind of viewer reads of a
// firing, built from an event that holds every shape at once (a roll with advantage, a die that
// was rolled again, a save that failed without rolling, a damage that landed, one that waits,
// one the master changed, a hidden combatant, an NPC's character). The answer of each viewer is
// a golden file (testdata/trap_firing_*.golden.json). Run with -update-trap-firing to write
// them again after a change of behavior that was meant.

var updateTrapFiring = flag.Bool("update-trap-firing", false, "write the golden files of firingProto again")

// aFiringEvent is a firing that caught a player's character, an NPC and a hidden NPC.
func aFiringEvent() *trapFireEvent {
	srcs := []sourceRec{{Kind: combat.SourceProneTarget, Effect: int(combat.ModeAdvantage)}, {Kind: combat.SourceEffectSave, Effect: int(combat.ModeDisadvantage), Condition: "condition:poisoned"}}
	return &trapFireEvent{
		PointID: "point-1", MapID: "map-1", Manual: true,
		Caught: []trapCaughtEvent{
			{
				Target: "c-toren", Character: "char-toren", Player: true, Conditions: []string{"condition:prone"},
				Attacks: []trapAttackEvent{
					{D20: 14, Modifier: 5, Total: 19, Outcome: outcomeHit, TargetAC: 16, Sources: srcs[:1]},
					{D20: 17, Modifier: 5, Total: 22, Outcome: outcomeCrit, TargetAC: 16, D20B: 3, Counted: 0, Mode: modeAdvantageKey, Sources: srcs[:1]},
					{D20: 2, Modifier: 5, Total: 7, Outcome: outcomeMiss, TargetAC: 16, D20B: 12, Counted: 1, Mode: modeDisadvantageKey},
				},
				Saves: []saveRoll{
					{D20: 9, Bonus: 3, Total: 12, DC: 12, Saved: true},
					{D20: 13, Bonus: 3, Total: 16, DC: 15, D20B: 4, Counted: 1, RollMode: modeAdvantageKey, Extra: []effectDie{{Key: "effect:bless", Faces: 4, Sign: 1, Face: 2}}, Lucky: &luckyReroll{Index: 0, From: 1, To: 13}},
					{Bonus: 3, Total: 3, DC: 15, Auto: true},
					{D20: 6, Bonus: 0, Total: 6, DC: 14, Unknown: true},
				},
				SaveSources: srcs,
				Damages: []trapDamageEvent{
					{damageHit: damageHit{Pending: "p-1", Target: "c-toren", Amount: 7}, Type: "damage-type:piercing", DiceCount: 2, DiceSides: 6, Bonus: 1, Faces: []int32{3, 3}, RollTotal: 7, Critical: true, CriticalMax: 6},
					{damageHit: damageHit{Pending: "p-2", Target: "c-toren", Amount: 3, Half: true}, Type: "damage-type:poison", DiceCount: 1, DiceSides: 8, Faces: []int32{6}, RollTotal: 6},
					{damageHit: damageHit{Pending: "p-3", Target: "c-toren", Amount: 4}, Type: "damage-type:fire", DiceCount: 1, DiceSides: 6, Faces: []int32{4}, RollTotal: 4},
				},
			},
			{
				Target: "c-goblin", Character: "char-bri", Hidden: false,
				Attacks: []trapAttackEvent{{D20: 11, Modifier: 5, Total: 16, Outcome: outcomeHit, TargetAC: 15}},
				Saves:   []saveRoll{{D20: 5, Bonus: 2, Total: 7, DC: 12}},
				Damages: []trapDamageEvent{
					{damageHit: damageHit{Pending: "p-4", Target: "c-goblin", Amount: 5, Applied: true, After: &hpState{HP: 0, Defeated: true}}, Type: "damage-type:piercing", DiceCount: 1, DiceSides: 6, Faces: []int32{5}, RollTotal: 5},
					{damageHit: damageHit{Pending: "p-5", Target: "c-goblin", Amount: 2, Applied: true, After: &hpState{HP: 3}}, Type: "damage-type:piercing", RollTotal: 2},
				},
				Conditions: []string{"condition:poisoned", "condition:prone"},
			},
			{
				Target: "c-secret", Character: "char-secret", Hidden: true,
				Damages: []trapDamageEvent{{damageHit: damageHit{Pending: "p-6", Target: "c-secret", Amount: 9, Applied: true}, Type: "damage-type:fire", RollTotal: 9}},
			},
		},
	}
}

func TestFiringProtoGoldenOfEveryViewer(t *testing.T) {
	t.Parallel()
	labels := map[string]string{"c-toren": "Toren", "c-goblin": "Goblin", "c-secret": "Segredo"}
	names := func(k string) string { return "nome:" + k }
	label := func(id string) string { return labels[id] }
	hidden := func(id string) bool { return id == "c-secret" }
	status := func(id string) (playv1.PendingDamageStatus, bool) {
		if id == "p-3" {
			return playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_DISCARDED, true
		}
		if id == "p-2" {
			return playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED, true
		}
		return 0, false
	}
	amount := func(id string) (int32, bool) { return 2, id == "p-2" }
	for _, tc := range []struct {
		viewer string
		view   trapView
	}{
		{"master", trapView{master: true, label: label, names: names, status: status, amount: amount}},
		{"owner", trapView{owns: func(id string) bool { return id == "c-toren" }, hidden: hidden, label: label, names: names, status: status, amount: amount}},
		{"other", trapView{owns: func(string) bool { return false }, hidden: hidden, label: label, names: names}},
		{"bare", trapView{names: names}},
	} {
		t.Run(tc.viewer, func(t *testing.T) {
			t.Parallel()
			raw, err := protojson.Marshal(firingProto(aFiringEvent(), "firing-1", "Fosso", tc.view))
			if err != nil {
				t.Fatal(err)
			}
			var tree any
			if err := json.Unmarshal(raw, &tree); err != nil {
				t.Fatal(err)
			}
			got, err := json.MarshalIndent(tree, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", "trap_firing_"+tc.viewer+".golden.json")
			if *updateTrapFiring {
				if err := os.WriteFile(path, got, 0o600); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read the golden file: %v (write it with -update-trap-firing)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("firingProto(%s) differs from %s:\n--- got\n%s\n--- want\n%s", tc.viewer, path, got, want)
			}
		})
	}
}
