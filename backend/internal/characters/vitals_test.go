package characters

import (
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// These tests need no database: they check how stored vitals and the
// sheet's maximums combine (RN-02).

// wizard3 are the maximums of a level 3 wizard: 4 slots of level 1, 2 of
// level 2, 3d6.
func wizard3() vitalsMax {
	m := vitalsMax{hitPoints: 20, hitDice: []rules.HitDice{{Die: 6, Count: 3}}, hitDiceTotal: 3}
	m.slots[0], m.slots[1] = 4, 2
	return m
}

// TestFreshVitals: a character nobody has adjusted has full hit points and
// nothing used.
func TestFreshVitals(t *testing.T) {
	t.Parallel()
	v := vitalsToProto(vitalsRow{ID: "c1", Name: "Pensantus"}, wizard3())
	want := &playv1.CharacterVitals{
		CharacterId: "c1", Name: "Pensantus",
		HitPointsCurrent: 20, HitPointsMax: 20,
		SpellSlots: []*playv1.SpellSlotUsage{{Level: 1, Total: 4}, {Level: 2, Total: 2}},
		HitDice:    v.GetHitDice(), HitDiceTotal: 3,
	}
	if !proto.Equal(v, want) {
		t.Errorf("fresh vitals = %v, want %v", v, want)
	}
	if len(v.GetHitDice()) != 1 || v.GetHitDice()[0].GetFaces() != 6 || v.GetHitDice()[0].GetCount() != 3 {
		t.Errorf("hit dice = %v, want 3d6", v.GetHitDice())
	}
}

// TestVitalsAreClampedToTheSheet: stored values above the sheet's current
// maximums (the character lost a level, or a class) never show.
func TestVitalsAreClampedToTheSheet(t *testing.T) {
	t.Parallel()
	now := time.Now()
	row := vitalsRow{
		ID:                 "c1",
		HitPointsCurrent:   new(int32(35)),
		HitPointsTemporary: new(int32(4)),
		SpellSlotsUsed:     []int32{3, 3, 2}, // level 3 slots are gone
		PactSlotsUsed:      new(int32(2)),    // no pact magic now
		HitDiceUsed:        new(int32(5)),
		Revision:           new(int32(7)),
		UpdatedAt:          &now,
	}
	v := vitalsToProto(row, wizard3())
	if v.GetHitPointsCurrent() != 20 || v.GetHitPointsTemporary() != 4 || v.GetHitDiceUsed() != 3 || v.GetRevision() != 7 {
		t.Errorf("vitals = %v, want hit points 20 (the maximum), 4 temporary, 3 hit dice used, revision 7", v)
	}
	if len(v.GetSpellSlots()) != 2 || v.GetSpellSlots()[0].GetUsed() != 3 || v.GetSpellSlots()[1].GetUsed() != 2 {
		t.Errorf("spell slots = %v, want 3 of 4 and 2 of 2, and no level 3", v.GetSpellSlots())
	}
	if v.GetPactSlots() != nil {
		t.Errorf("pact slots = %v, want none", v.GetPactSlots())
	}
}

func TestApplyVitalsChange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		req       *playv1.AdjustCharacterVitalsRequest
		wantField string // "" means accepted
	}{
		{"nothing to change", &playv1.AdjustCharacterVitalsRequest{}, "request"},
		{"hit points at 0", &playv1.AdjustCharacterVitalsRequest{HitPointsCurrent: new(int32(0))}, ""},
		{"hit points at the maximum", &playv1.AdjustCharacterVitalsRequest{HitPointsCurrent: new(int32(20))}, ""},
		{"hit points above the maximum", &playv1.AdjustCharacterVitalsRequest{HitPointsCurrent: new(int32(21))}, "hit_points_current"},
		{"negative hit points", &playv1.AdjustCharacterVitalsRequest{HitPointsCurrent: new(int32(-1))}, "hit_points_current"},
		{"temporary hit points at the limit", &playv1.AdjustCharacterVitalsRequest{HitPointsTemporary: new(int32(MaxTemporaryHitPoints))}, ""},
		{"too many temporary hit points", &playv1.AdjustCharacterVitalsRequest{HitPointsTemporary: new(int32(MaxTemporaryHitPoints + 1))}, "hit_points_temporary"},
		{"slots of two levels", &playv1.AdjustCharacterVitalsRequest{SpellSlotsUsed: []*playv1.SpellSlotsUsed{{Level: 2, Used: 2}, {Level: 1, Used: 0}}}, ""},
		{"too many slots used", &playv1.AdjustCharacterVitalsRequest{SpellSlotsUsed: []*playv1.SpellSlotsUsed{{Level: 1, Used: 1}, {Level: 2, Used: 3}}}, "spell_slots_used[1].used"},
		{"a level without slots", &playv1.AdjustCharacterVitalsRequest{SpellSlotsUsed: []*playv1.SpellSlotsUsed{{Level: 3, Used: 0}}}, "spell_slots_used[0].level"},
		{"a level twice", &playv1.AdjustCharacterVitalsRequest{SpellSlotsUsed: []*playv1.SpellSlotsUsed{{Level: 1, Used: 1}, {Level: 1, Used: 2}}}, "spell_slots_used[1].level"},
		{"pact slots without pact magic", &playv1.AdjustCharacterVitalsRequest{PactSlotsUsed: new(int32(0))}, "pact_slots_used"},
		{"every hit die", &playv1.AdjustCharacterVitalsRequest{HitDiceUsed: new(int32(3))}, ""},
		{"more hit dice than the level", &playv1.AdjustCharacterVitalsRequest{HitDiceUsed: new(int32(4))}, "hit_dice_used"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := vitalsToProto(vitalsRow{ID: "c1"}, wizard3())
			err := applyVitalsChange(v, tt.req)
			fe, isField := errors.AsType[*fieldError](err)
			switch {
			case tt.wantField == "" && err != nil:
				t.Errorf("error = %v, want accepted", err)
			case tt.wantField != "" && (!isField || fe.field != tt.wantField):
				t.Errorf("error = %v, want a field error on %s", err, tt.wantField)
			}
		})
	}

	// The accepted values land where they belong; the rest stay.
	v := vitalsToProto(vitalsRow{ID: "c1", HitPointsTemporary: new(int32(2))}, wizard3())
	if err := applyVitalsChange(v, &playv1.AdjustCharacterVitalsRequest{
		HitPointsCurrent: new(int32(7)),
		SpellSlotsUsed:   []*playv1.SpellSlotsUsed{{Level: 2, Used: 1}},
	}); err != nil {
		t.Fatalf("applyVitalsChange() error = %v", err)
	}
	if v.GetHitPointsCurrent() != 7 || v.GetHitPointsTemporary() != 2 || v.GetSpellSlots()[0].GetUsed() != 0 || v.GetSpellSlots()[1].GetUsed() != 1 {
		t.Errorf("after the change = %v, want 7 hit points, 2 temporary kept, 1 slot of level 2 used", v)
	}

	// A warlock's pact slots.
	warlock := vitalsMax{hitPoints: 10, pactLevel: 1, pactSlots: 1, hitDiceTotal: 1}
	v = vitalsToProto(vitalsRow{ID: "c2"}, warlock)
	if err := applyVitalsChange(v, &playv1.AdjustCharacterVitalsRequest{PactSlotsUsed: new(int32(1))}); err != nil || v.GetPactSlots().GetUsed() != 1 {
		t.Errorf("pact slots = %v, %v; want 1 used", v.GetPactSlots(), err)
	}
	if err := applyVitalsChange(v, &playv1.AdjustCharacterVitalsRequest{PactSlotsUsed: new(int32(2))}); err == nil {
		t.Error("2 pact slots of 1 were accepted")
	}
}
