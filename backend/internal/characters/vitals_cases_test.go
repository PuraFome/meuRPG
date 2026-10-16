package characters

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// These tests state, case by case, what applyVitalsChange and planRest do to a
// character's vitals: every refusal (its field and its message, in the order the checks
// run) and, for every accepted change, the fields that moved. No database: the vitals come
// from vitalsToProto on hand-made rows and maximums.

// vitalsGoldenDiff is the first lines that differ between two texts, enough to see what
// moved (the combat reads goldens use it too).
func vitalsGoldenDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	shown := 0
	for i := 0; i < max(len(w), len(g)) && shown < 20; i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			fmt.Fprintf(&b, "line %d:\n  want %s\n  got  %s\n", i+1, wl, gl)
			shown++
		}
	}
	return b.String()
}

// vitalsGoldenRich are the maximums of a character with a bit of everything: 30 hit
// points, slots of levels 1 to 3, two pact slots of level 2, 2d10 and 3d6, a resource of
// each recharge and a wolf's shape (11 hit points).
func vitalsGoldenRich() vitalsMax {
	m := vitalsMax{
		hitPoints: 30, pactLevel: 2, pactSlots: 2,
		hitDice:      []rules.HitDice{{Die: 10, Count: 2}, {Die: 6, Count: 3}},
		hitDiceTotal: 5, wizardLevel: 3, conMod: 2,
		resources: []rules.Resource{
			{Key: "arcane_recovery", NamePT: "Recuperação Arcana", Max: 1, Recharge: rules.RechargeLongRest},
			{Key: "second_wind", NamePT: "Retomar o Fôlego", Max: 1, Recharge: rules.RechargeShortRest},
			{Key: "channel_divinity", NamePT: "Canalizar Divindade", Max: 2, Recharge: rules.RechargeShortRest},
		},
		beast: "monster:wolf", beastNamePT: "Lobo", beastMax: 11,
	}
	m.slots[0], m.slots[1], m.slots[2] = 4, 3, 2
	return m
}

// vitalsGoldenRichRow is a wounded character of vitalsGoldenRich: 12 hit points, 3
// temporary, slots and resources spent, a created slot of level 1, a d10 spent and the
// wolf at 8.
func vitalsGoldenRichRow() vitalsRow {
	return vitalsRow{
		ID: "c-rich", Name: "Rica", PlayerUserID: new("u-rich"),
		HitPointsCurrent: new(int32(12)), HitPointsTemporary: new(int32(3)),
		SpellSlotsUsed: []int32{2, 1, 0}, SpellSlotsCreated: []int32{1, 0, 0},
		PactSlotsUsed: new(int32(1)), HitDiceUsedByDie: []byte(`{"10":1}`),
		ResourcesUsed:  []byte(`{"second_wind":1,"arcane_recovery":1}`),
		WildShapeBeast: new("monster:wolf"), WildShapeHp: new(int32(8)),
		Revision: new(int32(4)),
	}
}

// vitalsGoldenPlain is wizard3 with nothing spent: no pact slots, no resources, no beast.
func vitalsGoldenPlain() *playv1.CharacterVitals {
	return vitalsToProto(vitalsRow{ID: "c-plain", Name: "Simples"}, wizard3())
}

func vitalsGoldenRichView() *playv1.CharacterVitals {
	return vitalsToProto(vitalsGoldenRichRow(), vitalsGoldenRich())
}

// TestApplyVitalsChangeCases: applyVitalsChange, each case is the vitals it
// starts from, the request and either the refusal (field and message) or the fields
// that moved.
func TestApplyVitalsChangeCases(t *testing.T) {
	t.Parallel()
	n := func(v int32) *int32 { return &v }
	slots := func(pairs ...int32) []*playv1.SpellSlotsUsed {
		var out []*playv1.SpellSlotsUsed
		for i := 0; i+1 < len(pairs); i += 2 {
			out = append(out, &playv1.SpellSlotsUsed{Level: pairs[i], Used: pairs[i+1]})
		}
		return out
	}
	res := func(key string, used int32) *playv1.ResourceUsed { return &playv1.ResourceUsed{Key: key, Used: used} }
	type req = playv1.AdjustCharacterVitalsRequest
	cases := []struct {
		name string
		rich bool
		req  *req
		want string // "field: message" of the refusal, or the fields that moved
	}{
		{"nothing to change", true, &req{}, `request: must set at least one value to change`},
		{"nothing to change on a plain character", false, &req{}, `request: must set at least one value to change`},
		{"empty lists are nothing", true, &req{SpellSlotsUsed: []*playv1.SpellSlotsUsed{}, HitDiceUsedByDie: map[int32]int32{}}, `request: must set at least one value to change`},
		// The beast's hit points.
		{"wild shape without a form", false, &req{WildShapeHitPointsCurrent: n(3)}, `wild_shape_hit_points_current: must be unset: the character is not in a beast form`},
		{"wild shape negative", true, &req{WildShapeHitPointsCurrent: n(-1)}, `wild_shape_hit_points_current: must be 0 to 11`},
		{"wild shape above the beast", true, &req{WildShapeHitPointsCurrent: n(12)}, `wild_shape_hit_points_current: must be 0 to 11`},
		{"wild shape at the beast's maximum", true, &req{WildShapeHitPointsCurrent: n(11)}, `wildShape.hitPointsCurrent=11`},
		{"wild shape at 0 ends the form", true, &req{WildShapeHitPointsCurrent: n(0)}, `-wildShape.beastKey -wildShape.beastNamePt -wildShape.hitPointsCurrent -wildShape.hitPointsMax`},
		{"wild shape is checked before the hit points", true, &req{WildShapeHitPointsCurrent: n(99), HitPointsCurrent: n(99)}, `wild_shape_hit_points_current: must be 0 to 11`},
		// Hit points.
		{"hit points at 0", true, &req{HitPointsCurrent: n(0)}, `-hitPointsCurrent`},
		{"hit points at the maximum", true, &req{HitPointsCurrent: n(30)}, `hitPointsCurrent=30`},
		{"hit points above the maximum", true, &req{HitPointsCurrent: n(31)}, `hit_points_current: must be 0 to 30`},
		{"negative hit points", true, &req{HitPointsCurrent: n(-5)}, `hit_points_current: must be 0 to 30`},
		{"hit points are checked before the temporary ones", true, &req{HitPointsCurrent: n(31), HitPointsTemporary: n(-1)}, `hit_points_current: must be 0 to 30`},
		// Temporary hit points.
		{"temporary at 0", true, &req{HitPointsTemporary: n(0)}, `-hitPointsTemporary`},
		{"temporary at the limit", true, &req{HitPointsTemporary: n(MaxTemporaryHitPoints)}, `hitPointsTemporary=999`},
		{"temporary above the limit", true, &req{HitPointsTemporary: n(MaxTemporaryHitPoints + 1)}, `hit_points_temporary: must be 0 to 999`},
		{"negative temporary", true, &req{HitPointsTemporary: n(-1)}, `hit_points_temporary: must be 0 to 999`},
		{"temporary is checked before the slots", true, &req{HitPointsTemporary: n(-1), SpellSlotsUsed: slots(9, 1)}, `hit_points_temporary: must be 0 to 999`},
		// Spell slots.
		{"slots of three levels", true, &req{SpellSlotsUsed: slots(3, 2, 1, 5, 2, 0)}, `-spellSlots[1].used spellSlots[0].used=5 spellSlots[2].used=2`},
		{"a created slot counts in the total", true, &req{SpellSlotsUsed: slots(1, 5)}, `spellSlots[0].used=5`},
		{"a slot above the total", true, &req{SpellSlotsUsed: slots(1, 6)}, `spell_slots_used[0].used: must be 0 to 5`},
		{"a negative slot", true, &req{SpellSlotsUsed: slots(2, -1)}, `spell_slots_used[0].used: must be 0 to 3`},
		{"a level without slots", true, &req{SpellSlotsUsed: slots(4, 0)}, `spell_slots_used[0].level: must be a spell level the character has slots of`},
		{"level 0", true, &req{SpellSlotsUsed: slots(0, 0)}, `spell_slots_used[0].level: must be a spell level the character has slots of`},
		{"a level twice", true, &req{SpellSlotsUsed: slots(1, 1, 1, 2)}, `spell_slots_used[1].level: repeats level 1`},
		{"a level twice is found before a bad used", true, &req{SpellSlotsUsed: slots(2, 1, 2, 99)}, `spell_slots_used[1].level: repeats level 2`},
		{"the first bad entry wins", true, &req{SpellSlotsUsed: slots(1, 1, 7, 0, 2, 99)}, `spell_slots_used[1].level: must be a spell level the character has slots of`},
		{"slots are checked before the pact slots", true, &req{SpellSlotsUsed: slots(7, 0), PactSlotsUsed: n(9)}, `spell_slots_used[0].level: must be a spell level the character has slots of`},
		// Pact slots.
		{"pact slots without pact magic", false, &req{PactSlotsUsed: n(0)}, `pact_slots_used: must be unset: the character has no pact magic slots`},
		{"pact slots at the total", true, &req{PactSlotsUsed: n(2)}, `pactSlots.used=2`},
		{"pact slots at 0", true, &req{PactSlotsUsed: n(0)}, `-pactSlots.used`},
		{"pact slots above the total", true, &req{PactSlotsUsed: n(3)}, `pact_slots_used: must be 0 to 2`},
		{"pact slots are checked before the hit dice", true, &req{PactSlotsUsed: n(3), HitDiceUsed: n(99)}, `pact_slots_used: must be 0 to 2`},
		// Hit dice.
		{"a count and sizes together", true, &req{HitDiceUsed: n(1), HitDiceUsedByDie: map[int32]int32{6: 1}}, `hit_dice_used: must not be set together with hit_dice_used_by_die`},
		{"a count and sizes together, both bad", true, &req{HitDiceUsed: n(99), HitDiceUsedByDie: map[int32]int32{8: 99}}, `hit_dice_used: must not be set together with hit_dice_used_by_die`},
		{"a count spends the largest dice first", true, &req{HitDiceUsed: n(3)}, `hitDiceUsed=3 hitDiceUsedByDie.10=2 hitDiceUsedByDie.6=1`},
		{"every hit die", true, &req{HitDiceUsed: n(5)}, `hitDiceUsed=5 hitDiceUsedByDie.10=2 hitDiceUsedByDie.6=3`},
		{"no hit die", true, &req{HitDiceUsed: n(0)}, `-hitDiceUsed -hitDiceUsedByDie.10`},
		{"more hit dice than the level", true, &req{HitDiceUsed: n(6)}, `hit_dice_used: must be 0 to 5`},
		{"negative hit dice", true, &req{HitDiceUsed: n(-1)}, `hit_dice_used: must be 0 to 5`},
		{"a size set", true, &req{HitDiceUsedByDie: map[int32]int32{6: 2}}, `hitDiceUsed=3 hitDiceUsedByDie.6=2`},
		{"a size cleared", true, &req{HitDiceUsedByDie: map[int32]int32{10: 0}}, `-hitDiceUsed -hitDiceUsedByDie.10`},
		{"two sizes", true, &req{HitDiceUsedByDie: map[int32]int32{10: 2, 6: 3}}, `hitDiceUsed=5 hitDiceUsedByDie.10=2 hitDiceUsedByDie.6=3`},
		{"a size the character has not", true, &req{HitDiceUsedByDie: map[int32]int32{8: 1}}, `hit_dice_used_by_die[8]: must be a die size the character has`},
		{"a size above its count", true, &req{HitDiceUsedByDie: map[int32]int32{10: 3}}, `hit_dice_used_by_die[10]: must be 0 to 2`},
		{"a negative size", true, &req{HitDiceUsedByDie: map[int32]int32{6: -1}}, `hit_dice_used_by_die[6]: must be 0 to 3`},
		{"hit dice are checked before the resources", true, &req{HitDiceUsed: n(9), ResourcesUsed: []*playv1.ResourceUsed{res("nope", 0)}}, `hit_dice_used: must be 0 to 5`},
		// Resources.
		{"a resource spent", true, &req{ResourcesUsed: []*playv1.ResourceUsed{res("channel_divinity", 2)}}, `resources[2].used=2`},
		{"a resource given back", true, &req{ResourcesUsed: []*playv1.ResourceUsed{res("second_wind", 0)}}, `-resources[1].used`},
		{"two resources", true, &req{ResourcesUsed: []*playv1.ResourceUsed{res("arcane_recovery", 0), res("channel_divinity", 1)}}, `-resources[0].used resources[2].used=1`},
		{"a resource the character has not", true, &req{ResourcesUsed: []*playv1.ResourceUsed{res("rage", 1)}}, `resources_used[0].key: must be a resource the character has`},
		{"a resource above its total", true, &req{ResourcesUsed: []*playv1.ResourceUsed{res("second_wind", 2)}}, `resources_used[0].used: must be 0 to 1`},
		{"a negative resource", true, &req{ResourcesUsed: []*playv1.ResourceUsed{res("second_wind", -1)}}, `resources_used[0].used: must be 0 to 1`},
		{"a resource twice", true, &req{ResourcesUsed: []*playv1.ResourceUsed{res("second_wind", 0), res("second_wind", 1)}}, `resources_used[1].key: repeats a resource`},
		{"an unknown key is found before it repeats", true, &req{ResourcesUsed: []*playv1.ResourceUsed{res("rage", 0), res("rage", 0)}}, `resources_used[0].key: must be a resource the character has`},
		{"no resource on a plain character", false, &req{ResourcesUsed: []*playv1.ResourceUsed{res("second_wind", 0)}}, `resources_used[0].key: must be a resource the character has`},
		// Everything at once.
		{"everything at once", true, &req{
			WildShapeHitPointsCurrent: n(4), HitPointsCurrent: n(20), HitPointsTemporary: n(7),
			SpellSlotsUsed: slots(1, 0, 3, 2), PactSlotsUsed: n(0), HitDiceUsedByDie: map[int32]int32{6: 1, 10: 0},
			ResourcesUsed: []*playv1.ResourceUsed{res("arcane_recovery", 0), res("second_wind", 0), res("channel_divinity", 2)},
		}, `-hitDiceUsedByDie.10 -pactSlots.used -resources[0].used -resources[1].used -spellSlots[0].used hitDiceUsedByDie.6=1 hitPointsCurrent=20 hitPointsTemporary=7 resources[2].used=2 spellSlots[2].used=2 wildShape.hitPointsCurrent=4`},
	}
	for _, c := range cases {
		v := vitalsGoldenPlain()
		if c.rich {
			v = vitalsGoldenRichView()
		}
		before := protoFlat(t, v)
		var got string
		err := applyVitalsChange(v, c.req)
		switch fe, ok := errors.AsType[*fieldError](err); {
		case ok:
			got = fe.field + ": " + fe.err.Error()
		case err != nil:
			t.Fatalf("%s: applyVitalsChange() error = %v, want a fieldError or nil", c.name, err)
		default:
			got = fieldsMoved(before, protoFlat(t, v))
		}
		if got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.name, got, c.want)
		}
	}

	// A refused change leaves on v what the checks before it set: AdjustVitals throws v
	// away, so nothing of it is stored.
	v := vitalsGoldenRichView()
	if err := applyVitalsChange(v, &playv1.AdjustCharacterVitalsRequest{HitPointsCurrent: n(1), HitPointsTemporary: n(1000)}); err == nil || v.GetHitPointsCurrent() != 1 {
		t.Errorf("after a refusal = %d hit points, %v; want 1 and the refusal", v.GetHitPointsCurrent(), err)
	}
}

// TestApplyVitalsChangeHitDiceSizesTwoBadOrder: with two bad sizes in the map, which one
// is named depends on Go's map order. Finding: the refusal of hit_dice_used_by_die is not
// deterministic when more than one size is wrong; the test accepts either.
func TestApplyVitalsChangeHitDiceSizesTwoBadOrder(t *testing.T) {
	t.Parallel()
	for range 20 {
		v := vitalsGoldenRichView()
		err := applyVitalsChange(v, &playv1.AdjustCharacterVitalsRequest{HitDiceUsedByDie: map[int32]int32{8: 1, 12: 1}})
		fe, ok := errors.AsType[*fieldError](err)
		if !ok || (fe.field != "hit_dice_used_by_die[8]" && fe.field != "hit_dice_used_by_die[12]") || fe.err.Error() != "must be a die size the character has" {
			t.Fatalf("error = %v, want a die size the character has, on [8] or [12]", err)
		}
	}
}

// TestPlanRestCases: planRest, for each character and rest, what the master's
// preview says and the vitals after, or the refusal.
func TestPlanRestCases(t *testing.T) {
	t.Parallel()
	rich := vitalsGoldenRich()
	aided := vitalsGoldenRichRow()
	aided.HitPointsMaxBonus = new(int32(5))
	aided.HitPointsCurrent = new(int32(33))
	down := vitalsGoldenRichRow()
	down.HitPointsCurrent = new(int32(0))
	spentAll := vitalsGoldenRichRow()
	spentAll.HitDiceUsedByDie = []byte(`{"10":2,"6":3}`)
	createdOnly := vitalsGoldenRichRow()
	createdOnly.SpellSlotsCreated = []int32{0, 0, 0, 2} // two 4th-level slots the sheet has none of
	createdOnly.SpellSlotsUsed = []int32{0, 0, 0, 1}
	level1 := vitalsMax{hitPoints: 8, hitDice: []rules.HitDice{{Die: 8, Count: 1}}, hitDiceTotal: 1}
	level1Row := vitalsRow{ID: "c-l1", Name: "Novata", HitPointsCurrent: new(int32(3)), HitDiceUsedByDie: []byte(`{"8":1}`)}
	cases := []struct {
		name   string
		kind   string
		row    vitalsRow
		m      vitalsMax
		choice rules.HitDiceUsed
		want   string // "error: ..." or "preview <fields> / after <fields moved>"
	}{
		{"short rest", rules.RestShort, vitalsGoldenRichRow(), rich, nil, `preview hitDiceBackLimit=2 hitDiceSpent[0].count=1 hitDiceSpent[0].faces=10 pactSlotsBack=1 resources[0].key="second_wind" resources[0].spent=1 resources[0].total=1 / after -pactSlots.used -resources[1].used`},
		{"short rest ignores a hit dice choice", rules.RestShort, vitalsGoldenRichRow(), rich, rules.HitDiceUsed{8: 9}, `preview hitDiceBackLimit=2 hitDiceSpent[0].count=1 hitDiceSpent[0].faces=10 pactSlotsBack=1 resources[0].key="second_wind" resources[0].spent=1 resources[0].total=1 / after -pactSlots.used -resources[1].used`},
		{"long rest, largest dice first", rules.RestLong, vitalsGoldenRichRow(), rich, nil, `preview createdSlotsLost=1 hitDiceBackLimit=2 hitDiceBack[0].count=1 hitDiceBack[0].faces=10 hitDiceSpent[0].count=1 hitDiceSpent[0].faces=10 pactSlotsBack=1 resources[0].key="arcane_recovery" resources[0].spent=1 resources[0].total=1 resources[1].key="second_wind" resources[1].spent=1 resources[1].total=1 spellSlotsBack[0].count=2 spellSlotsBack[0].level=1 spellSlotsBack[1].count=1 spellSlotsBack[1].level=2 temporaryHitPointsLost=3 / after -hitDiceUsed -hitDiceUsedByDie.10 -hitPointsTemporary -pactSlots.used -resources[0].used -resources[1].used -spellSlots[0].created -spellSlots[0].used -spellSlots[1].used hitPointsCurrent=30 spellSlots[0].total=4`},
		{"long rest, every die spent", rules.RestLong, spentAll, rich, nil, `preview createdSlotsLost=1 hitDiceBackLimit=2 hitDiceBack[0].count=2 hitDiceBack[0].faces=10 hitDiceSpent[0].count=2 hitDiceSpent[0].faces=10 hitDiceSpent[1].count=3 hitDiceSpent[1].faces=6 pactSlotsBack=1 resources[0].key="arcane_recovery" resources[0].spent=1 resources[0].total=1 resources[1].key="second_wind" resources[1].spent=1 resources[1].total=1 spellSlotsBack[0].count=2 spellSlotsBack[0].level=1 spellSlotsBack[1].count=1 spellSlotsBack[1].level=2 temporaryHitPointsLost=3 / after -hitDiceUsedByDie.10 -hitPointsTemporary -pactSlots.used -resources[0].used -resources[1].used -spellSlots[0].created -spellSlots[0].used -spellSlots[1].used hitDiceUsed=3 hitPointsCurrent=30 spellSlots[0].total=4`},
		{"long rest, a chosen d6", rules.RestLong, spentAll, rich, rules.HitDiceUsed{6: 2}, `preview createdSlotsLost=1 hitDiceBackLimit=2 hitDiceBack[0].count=2 hitDiceBack[0].faces=6 hitDiceSpent[0].count=2 hitDiceSpent[0].faces=10 hitDiceSpent[1].count=3 hitDiceSpent[1].faces=6 pactSlotsBack=1 resources[0].key="arcane_recovery" resources[0].spent=1 resources[0].total=1 resources[1].key="second_wind" resources[1].spent=1 resources[1].total=1 spellSlotsBack[0].count=2 spellSlotsBack[0].level=1 spellSlotsBack[1].count=1 spellSlotsBack[1].level=2 temporaryHitPointsLost=3 / after -hitPointsTemporary -pactSlots.used -resources[0].used -resources[1].used -spellSlots[0].created -spellSlots[0].used -spellSlots[1].used hitDiceUsed=3 hitDiceUsedByDie.6=1 hitPointsCurrent=30 spellSlots[0].total=4`},
		{"long rest, an empty choice gives no die back", rules.RestLong, spentAll, rich, rules.HitDiceUsed{}, `preview createdSlotsLost=1 hitDiceBackLimit=2 hitDiceSpent[0].count=2 hitDiceSpent[0].faces=10 hitDiceSpent[1].count=3 hitDiceSpent[1].faces=6 pactSlotsBack=1 resources[0].key="arcane_recovery" resources[0].spent=1 resources[0].total=1 resources[1].key="second_wind" resources[1].spent=1 resources[1].total=1 spellSlotsBack[0].count=2 spellSlotsBack[0].level=1 spellSlotsBack[1].count=1 spellSlotsBack[1].level=2 temporaryHitPointsLost=3 / after -hitPointsTemporary -pactSlots.used -resources[0].used -resources[1].used -spellSlots[0].created -spellSlots[0].used -spellSlots[1].used hitPointsCurrent=30 spellSlots[0].total=4`},
		{"long rest, a choice above the limit", rules.RestLong, spentAll, rich, rules.HitDiceUsed{6: 3}, `error: the hit dice choice is not the character's`},
		{"long rest, a size the character has not", rules.RestLong, spentAll, rich, rules.HitDiceUsed{8: 1}, `error: the hit dice choice is not the character's`},
		{"long rest, a size not spent", rules.RestLong, vitalsGoldenRichRow(), rich, rules.HitDiceUsed{6: 1}, `error: the hit dice choice is not the character's`},
		{"long rest, a zero count", rules.RestLong, spentAll, rich, rules.HitDiceUsed{6: 0}, `error: the hit dice choice is not the character's`},
		{"long rest under Aid", rules.RestLong, aided, rich, nil, `preview createdSlotsLost=1 hitDiceBackLimit=2 hitDiceBack[0].count=1 hitDiceBack[0].faces=10 hitDiceSpent[0].count=1 hitDiceSpent[0].faces=10 pactSlotsBack=1 resources[0].key="arcane_recovery" resources[0].spent=1 resources[0].total=1 resources[1].key="second_wind" resources[1].spent=1 resources[1].total=1 spellSlotsBack[0].count=2 spellSlotsBack[0].level=1 spellSlotsBack[1].count=1 spellSlotsBack[1].level=2 temporaryHitPointsLost=3 / after -hitDiceUsed -hitDiceUsedByDie.10 -hitPointsMaxBonus -hitPointsTemporary -pactSlots.used -resources[0].used -resources[1].used -spellSlots[0].created -spellSlots[0].used -spellSlots[1].used hitPointsCurrent=30 hitPointsMax=30 spellSlots[0].total=4`},
		{"short rest under Aid", rules.RestShort, aided, rich, nil, `preview hitDiceBackLimit=2 hitDiceSpent[0].count=1 hitDiceSpent[0].faces=10 pactSlotsBack=1 resources[0].key="second_wind" resources[0].spent=1 resources[0].total=1 / after -pactSlots.used -resources[1].used`},
		{"long rest at 0 hit points gives nothing", rules.RestLong, down, rich, nil, `preview hitDiceBackLimit=2 hitDiceSpent[0].count=1 hitDiceSpent[0].faces=10 noBenefit=true / after unchanged`},
		{"short rest at 0 hit points still recharges", rules.RestShort, down, rich, nil, `preview hitDiceBackLimit=2 hitDiceSpent[0].count=1 hitDiceSpent[0].faces=10 pactSlotsBack=1 resources[0].key="second_wind" resources[0].spent=1 resources[0].total=1 / after -pactSlots.used -resources[1].used`},
		{"long rest drops a level of created slots only", rules.RestLong, createdOnly, rich, nil, `preview createdSlotsLost=2 hitDiceBackLimit=2 hitDiceBack[0].count=1 hitDiceBack[0].faces=10 hitDiceSpent[0].count=1 hitDiceSpent[0].faces=10 pactSlotsBack=1 resources[0].key="arcane_recovery" resources[0].spent=1 resources[0].total=1 resources[1].key="second_wind" resources[1].spent=1 resources[1].total=1 temporaryHitPointsLost=3 / after -hitDiceUsed -hitDiceUsedByDie.10 -hitPointsTemporary -pactSlots.used -resources[0].used -resources[1].used -spellSlots[3].created -spellSlots[3].level -spellSlots[3].total -spellSlots[3].used hitPointsCurrent=30`},
		{"long rest of a fresh character", rules.RestLong, vitalsRow{ID: "c-fresh", Name: "Nova"}, wizard3(), nil, `preview hitDiceBackLimit=1 / after unchanged`},
		{"short rest of a fresh character", rules.RestShort, vitalsRow{ID: "c-fresh", Name: "Nova"}, wizard3(), nil, `preview hitDiceBackLimit=1 / after unchanged`},
		{"long rest of a level 1 character gives back one die", rules.RestLong, level1Row, level1, nil, `preview hitDiceBackLimit=1 hitDiceBack[0].count=1 hitDiceBack[0].faces=8 hitDiceSpent[0].count=1 hitDiceSpent[0].faces=8 / after -hitDiceUsed -hitDiceUsedByDie.8 hitPointsCurrent=8`},
		{"an unknown kind is a short rest without the short-rest gains", "nap", vitalsGoldenRichRow(), rich, nil, `preview hitDiceBackLimit=2 hitDiceSpent[0].count=1 hitDiceSpent[0].faces=10 / after unchanged`},
	}
	for _, c := range cases {
		v := vitalsToProto(c.row, c.m)
		was := proto.CloneOf(v)
		plan, err := planRest(c.kind, v, c.m, c.choice)
		if !proto.Equal(v, was) {
			t.Errorf("%s: planRest changed the vitals it was given", c.name)
		}
		var got string
		if err != nil {
			if !errors.Is(err, link.ErrBadHitDiceChoice) {
				t.Errorf("%s: error = %v, want link.ErrBadHitDiceChoice", c.name, err)
			}
			got = "error: " + err.Error()
		} else {
			got = "preview " + vitalsPreview(protoFlat(t, plan.preview)) + " / after " + fieldsMoved(protoFlat(t, was), protoFlat(t, plan.after))
		}
		if got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.name, got, c.want)
		}
	}
}

// vitalsPreview is what a rest's preview tells the master beyond the character's
// own name, id and hit points (those are the vitals' own).
func vitalsPreview(flat map[string]string) string {
	for k := range flat {
		if strings.HasSuffix(k, ".namePt") {
			delete(flat, k)
		}
	}
	for _, k := range []string{"characterId", "name", "hitPointsCurrent", "hitPointsMax"} {
		delete(flat, k)
	}
	return fieldsMoved(map[string]string{}, flat)
}
