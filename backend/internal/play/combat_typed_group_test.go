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
// both piercing, against a creature that resists piercing. The group's sum and the same two parts typed one by one are the
// same damage: the same step and the creature loses the same hit points, an odd total halved down as the steps do.
func TestATypedGroupGetsTheResistanceOfItsTypeLikeTheSamePartsRolledOneByOne(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name          string
		weapon, sneak int32
		before, after int32
	}{
		{"an even total", 3, 4, 10, 5},
		{"an odd total", 4, 4, 11, 5},
	} {
		t.Run(tt.name, func(t *testing.T) {
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
			byParts, partSteps := roll(&playv1.TypedPart{PartKey: "weapon", Sum: tt.weapon}, &playv1.TypedPart{PartKey: "sneak-attack", Sum: tt.sneak})
			byGroup, groupSteps := roll(&playv1.TypedPart{DamageType: "damage-type:piercing", Sum: tt.weapon + tt.sneak})
			if byParts != tt.after || byGroup != byParts {
				t.Errorf("hit points lost: %d by parts, %d by group, want %d and %d (%d halved)", byParts, byGroup, tt.after, tt.after, tt.before)
			}
			if len(partSteps) != 1 || len(groupSteps) != 1 || groupSteps[0].GetBefore() != tt.before || groupSteps[0].GetAfter() != tt.after || partSteps[0].GetAfter() != tt.after {
				t.Errorf("steps: %v by parts, %v by group, want one resistance, %d to %d", partSteps, groupSteps, tt.before, tt.after)
			}
		})
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

// TestAHeldTypedGroupLandsAsRolledThroughTheRealRolls: a rogue's rapier and Sneak Attack (both piercing) are typed as one
// group against another rogue, whose Uncanny Dodge holds the damage. The window reads the group's total with the flat
// number, and the answer lands it halved: the replay read the group's sum from the part that carries it.
func TestAHeldTypedGroupLandsAsRolledThroughTheRealRolls(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:rogue", "race:human", 3, abilities(16, 16, 14, 12, 10), []string{rapier}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Reator", "class:rogue", "race:human", 5, abilities(10, 14, 12, 10, 8), []string{rapier}, nil)
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3}, reveal: []string{"Goblin"},
		players: map[string]int32{"Toren": 18, "Reator": 10, "Brisa": 1},
		at:      map[string][2]int32{"Toren": {3, 3}, "Goblin": {8, 8}, "Reator": {4, 4}, "Brisa": {9, 9}},
	})
	a.setConditions(t, e, "Reator", "condition:restrained")
	hit := a.mustAttack(t, a.caio, e, "Toren", rapier, "Reator", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
	held := a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(),
		extrasMarked([]*playv1.SelectedExtra{{Key: "sneak-attack"}}, &playv1.TypedPart{DamageType: groupPiercing, Sum: 9}))
	if held.GetPendingDamage().GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		t.Fatalf("the damage landed before the rogue answered: %v", held.GetPendingDamage())
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE)
	if w == nil || w.GetUncannyDodge().GetDamage() != 12 || w.GetUncannyDodge().GetHalved() != 6 {
		t.Fatalf("the rogue's window = %v, want the group's 9 and the flat 3: 12 halved to 6", w)
	}
	a.mustAnswer(t, a.ana, e, w.GetId(), useAnswer(0))
	if status, amount := a.pendingOf(t, hit.GetPendingDamage().GetId()); status != "rolled" || amount != 6 {
		t.Errorf("the pending damage = %s for %d, want rolled for 6", status, amount)
	}
	var raw []byte
	if err := a.h.pool.QueryRow(t.Context(), `SELECT part_rolls FROM pending_damages WHERE id = $1`, hit.GetPendingDamage().GetId()).Scan(&raw); err != nil {
		t.Fatalf("read the part rolls: %v", err)
	}
	var carrying int
	for _, r := range readPartRolls(raw) {
		if r.Group != groupPiercing {
			t.Errorf("the part %s of the landed damage keeps group %q, want %s", r.Key, r.Group, groupPiercing)
		}
		if r.Sum != 0 {
			carrying++
			if r.Key != partWeapon || r.Sum != 9 {
				t.Errorf("the sum %d sits on %s, want the group's 9 on the weapon", r.Sum, r.Key)
			}
		}
	}
	if carrying != 1 {
		t.Errorf("%d parts carry a sum, want the group's on one", carrying)
	}
}
