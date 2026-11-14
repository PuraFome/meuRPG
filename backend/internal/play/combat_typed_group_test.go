package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// These tests freeze the typed sum of a damage type group: one number for every dice part of a type (the weapon, a
// hit effect, the extras that roll dice), checked against the combined range of the group.

const (
	groupPiercing = "damage-type:piercing"
	groupRadiant  = "damage-type:radiant"
	groupFire     = "damage-type:fire"
)

// groupParts is a weapon of 1d8 + 3 piercing, a hit effect of 1d6 piercing and a radiant extra of 1d4.
func groupParts() []partRecord {
	return []partRecord{
		{Key: partWeapon, Kind: partWeapon, Count: 1, Sides: 8, Flat: 3, DamageType: groupPiercing, Available: true},
		{Key: "spell:hunters-hex", Kind: partAuto, Count: 1, Sides: 6, DamageType: groupPiercing, Available: true, Selected: true},
		{Key: "radiant-edge", Kind: partExtra, Choosable: true, Selected: true, Count: 1, Sides: 4, DamageType: groupRadiant, Available: true},
	}
}

func rollGroups(critical bool, rule combat.CriticalRule, typed typedSums, parts ...partRecord) (partsRoll, error) {
	s := &Service{}
	if len(parts) == 0 {
		parts = groupParts()
	}
	return s.rollParts(rollPartsInput{
		p: playdb.PendingDamage{Critical: critical}, parts: parts, rule: rule, typed: typed,
		choice:   extraChoice{chosen: true, marks: map[string]*playv1.SelectedExtra{"radiant-edge": {Key: "radiant-edge"}}},
		selectOK: func(partRecord, *playv1.SelectedExtra) error { return nil },
	})
}

func byTypeSums(types map[string]int, parts map[string]int) typedSums {
	return typedSums{byType: types, byPart: parts}
}

func TestATypedGroupIsCheckedAgainstTheCombinedRangeOfItsParts(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		sum  int
		ok   bool
	}{{"below", 1, false}, {"lowest", 2, true}, {"middle", 9, true}, {"highest", 14, true}, {"above", 15, false}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := rollGroups(false, combat.CriticalDoubledDice, byTypeSums(map[string]int{groupPiercing: tt.sum}, map[string]int{"radiant-edge": 2}))
			if !tt.ok {
				if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "the sum of damage-type:piercing must be 2 to 14") {
					t.Fatalf("rollParts() error = %v, want invalid_argument naming the range 2 to 14", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("rollParts() error = %v", err)
			}
			if got.byType[groupPiercing] != tt.sum+3 || got.byType[groupRadiant] != 2 || got.total != tt.sum+3+2 {
				t.Errorf("byType = %v total %d, want piercing %d (the group plus the flat 3) and radiant 2", got.byType, got.total, tt.sum+3)
			}
			for _, r := range got.rolls {
				if len(r.Faces) != 0 || len(r.CountedDice) != 0 {
					t.Errorf("the part %s keeps faces %v / %v, want none for a group", r.Key, r.Faces, r.CountedDice)
				}
			}
		})
	}
}

func TestATypedGroupRangeDoublesOnACritical(t *testing.T) {
	t.Parallel()
	// Doubled dice: 2d8 + 2d6 = 4 to 28. Maximum plus roll: one die each (1 to 8 and 1 to 6) with the maxima added without rolling.
	for _, tt := range []struct {
		name   string
		rule   combat.CriticalRule
		lo, hi int
		fixed  int
	}{
		{"doubled dice", combat.CriticalDoubledDice, 4, 28, 0},
		{"maximum plus roll", combat.CriticalMaxPlusRoll, 2, 14, 14},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ok := func(n int) error {
				_, err := rollGroups(true, tt.rule, byTypeSums(map[string]int{groupPiercing: n}, map[string]int{"radiant-edge": 2}))
				return err
			}
			if err := ok(tt.lo); err != nil {
				t.Errorf("sum %d: %v", tt.lo, err)
			}
			if err := ok(tt.hi); err != nil {
				t.Errorf("sum %d: %v", tt.hi, err)
			}
			if err := ok(tt.lo - 1); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("sum %d: error = %v, want invalid_argument", tt.lo-1, err)
			}
			if err := ok(tt.hi + 1); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("sum %d: error = %v, want invalid_argument", tt.hi+1, err)
			}
			got, err := rollGroups(true, tt.rule, byTypeSums(map[string]int{groupPiercing: tt.lo}, map[string]int{"radiant-edge": 2}))
			if err != nil {
				t.Fatal(err)
			}
			if want := tt.lo + 3 + tt.fixed; got.byType[groupPiercing] != want {
				t.Errorf("piercing = %d, want %d (the group, the flat 3 and the fixed %d)", got.byType[groupPiercing], want, tt.fixed)
			}
		})
	}
}

func TestATypedGroupRefusals(t *testing.T) {
	t.Parallel()
	radiant := map[string]int{"radiant-edge": 2}
	for _, tt := range []struct {
		name  string
		typed typedSums
		want  string
	}{
		{"a group and a part of it", byTypeSums(map[string]int{groupPiercing: 5}, map[string]int{partWeapon: 3, "radiant-edge": 2}), "typed_parts names damage-type:piercing and a part of it (weapon)"},
		{"a type with no dice part", byTypeSums(map[string]int{groupPiercing: 5, groupFire: 2}, radiant), "typed_parts names damage-type:fire, which has no dice in this attack"},
		{"a part that is not covered", byTypeSums(map[string]int{groupPiercing: 5}, nil), "typed_parts needs a sum for radiant-edge"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := rollGroups(false, combat.CriticalDoubledDice, tt.typed)
			if connect.CodeOf(err) != connect.CodeInvalidArgument || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("rollParts() error = %v, want invalid_argument %q", err, tt.want)
			}
		})
	}
}

func TestTypedPartsOfReadsGroupsAndRefusesWhatIsNotOne(t *testing.T) {
	t.Parallel()
	part := func(key, typ string, sum int32) *playv1.TypedPart {
		return &playv1.TypedPart{PartKey: key, DamageType: typ, Sum: sum}
	}
	got, err := typedPartsOf([]*playv1.TypedPart{part(partWeapon, "", 3), part("", groupPiercing, 9)})
	if err != nil || got.byPart[partWeapon] != 3 || got.byType[groupPiercing] != 9 {
		t.Fatalf("typedPartsOf() = %+v, %v", got, err)
	}
	for name, raw := range map[string][]*playv1.TypedPart{
		"the same type twice":              {part("", groupPiercing, 3), part("", groupPiercing, 4)},
		"both a key and a type":            {part(partWeapon, groupPiercing, 3)},
		"neither":                          {part("", "", 3)},
		"a type that is not a damage type": {part("", "condition:blinded", 3)},
	} {
		if _, err := typedPartsOf(raw); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: error = %v, want invalid_argument", name, err)
		}
	}
}

func TestAGroupAndAPartKeyOfAnotherTypeRollTogether(t *testing.T) {
	t.Parallel()
	got, err := rollGroups(false, combat.CriticalDoubledDice, byTypeSums(map[string]int{groupPiercing: 8}, map[string]int{"radiant-edge": 4}))
	if err != nil {
		t.Fatalf("rollParts() error = %v", err)
	}
	if got.byType[groupPiercing] != 11 || got.byType[groupRadiant] != 4 || got.total != 15 {
		t.Errorf("byType = %v total %d, want piercing 11 and radiant 4", got.byType, got.total)
	}
}

// TestATypedGroupGetsTheResistanceOfItsTypeLikeTheSamePartsRolledOneByOne: a rapier (1d8 + 3) and a Sneak Attack (2d6),
// both piercing, against a creature that resists piercing. The group's 7 and the parts 3 and 4 are the same damage:
// the step is 10 to 5 and the creature loses the same hit points.
func TestATypedGroupGetsTheResistanceOfItsTypeLikeTheSamePartsRolledOneByOne(t *testing.T) {
	t.Parallel()
	a, e, shrub := partsFight(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:rogue", "race:human", 3, abilities(16, 16, 14, 12, 10), []string{rapier}, nil)
	}, "monster:awakened-shrub")
	hp := func() int32 { return byLabel(t, a.get(t, a.master), shrub).GetHitPointsCurrent() }
	sneak := []*playv1.SelectedExtra{{Key: "sneak-attack"}}
	roll := func(typed ...*playv1.TypedPart) (lost int32, steps []*playv1.DamageStep) {
		t.Helper()
		e = a.passTo(t, a.get(t, a.master), "Toren")
		a.setConditions(t, e, shrub, "condition:restrained")
		hit := a.mustAttack(t, a.caio, e, "Toren", rapier, shrub, func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
		before := hp()
		a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), extrasMarked(sneak, typed...))
		for _, r := range a.log(t, a.master, e).GetRounds() {
			for _, en := range r.GetEntries() {
				if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && en.GetDamage() != nil {
					steps = en.GetDamage().GetSteps()
				}
			}
		}
		a.mustEndTurn(t, a.caio, a.get(t, a.caio))
		return before - hp(), steps
	}
	byParts, partSteps := roll(&playv1.TypedPart{PartKey: "weapon", Sum: 3}, &playv1.TypedPart{PartKey: "sneak-attack", Sum: 4})
	byGroup, groupSteps := roll(&playv1.TypedPart{DamageType: "damage-type:piercing", Sum: 7})
	if byParts != 5 || byGroup != byParts {
		t.Errorf("hit points lost: %d by parts, %d by group, want 5 and 5 (10 halved)", byParts, byGroup)
	}
	if len(partSteps) != 1 || len(groupSteps) != 1 || groupSteps[0].GetBefore() != 10 || groupSteps[0].GetAfter() != 5 {
		t.Errorf("steps: %v by parts, %v by group, want one resistance, 10 to 5", partSteps, groupSteps)
	}
}

// TestAHeldRollOfATypedGroupLandsAsRolled: the replay of a damage a reaction window held rebuilds the type's total
// from the stored parts, the group's sum on its first part.
func TestAHeldRollOfATypedGroupLandsAsRolled(t *testing.T) {
	t.Parallel()
	got, err := rollGroups(false, combat.CriticalDoubledDice, byTypeSums(map[string]int{groupPiercing: 9}, map[string]int{"radiant-edge": 3}))
	if err != nil {
		t.Fatal(err)
	}
	held := heldPartsRoll(got.parts, got.rolls, true)
	if held.byType[groupPiercing] != 12 || held.byType[groupRadiant] != 3 || held.total != 15 {
		t.Errorf("held byType = %v total %d, want piercing 12 and radiant 3", held.byType, held.total)
	}
}
