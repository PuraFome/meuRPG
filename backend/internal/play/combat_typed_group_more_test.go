package play

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The second part of the tests of a typed damage-type group: what the group's range counts (and leaves out), the
// master's removal of a part the group covers, what a part hidden from the others does to the group's words and to
// the rows the others read (RN-10), and the small rules of reading the typed parts.

// groupCase is one roll of the parts by typed sums: the extras marked, the critical, the weapon's extra dice, the target.
type groupCase struct {
	parts     []partRecord
	marks     []string
	critical  bool
	extraDice int32
	target    link.Traits
	typed     typedSums
}

func (g groupCase) roll() (partsRoll, error) {
	s := &Service{}
	marks := map[string]*playv1.SelectedExtra{}
	for _, k := range g.marks {
		marks[k] = &playv1.SelectedExtra{Key: k}
		if k == combat.ExtraDivineSmite {
			marks[k].SlotLevel = 1
		}
	}
	return s.rollParts(rollPartsInput{
		p: playdb.PendingDamage{Critical: g.critical, ExtraDice: g.extraDice}, parts: g.parts, rule: combat.CriticalDoubledDice, typed: g.typed, target: g.target,
		choice:   extraChoice{chosen: true, marks: marks},
		selectOK: func(partRecord, *playv1.SelectedExtra) error { return nil },
	})
}

func wantRange(t *testing.T, what string, g groupCase, typ string, lo, hi int) {
	t.Helper()
	with := func(n int) error {
		g.typed.byType = map[string]int{typ: n}
		_, err := g.roll()
		return err
	}
	for _, n := range []int{lo, hi} {
		if err := with(n); err != nil {
			t.Errorf("%s: the sum %d error = %v, want it accepted", what, n, err)
		}
	}
	want := fmt.Sprintf("the sum of %s must be %d to %d", typ, lo, hi)
	for _, n := range []int{lo - 1, hi + 1} {
		if err := with(n); connect.CodeOf(err) != connect.CodeInvalidArgument || err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: the sum %d error = %v, want invalid_argument %q", what, n, err, want)
		}
	}
}

// The extra die of Divine Smite counts against an undead or a fiend and nothing tells the player: it is never in a
// group, whatever the target, so the range of the radiant group is the smite's own dice and the die keeps its key.
func TestTheExtraDieOfDivineSmiteStaysOutOfTheRadiantGroupWhateverTheTarget(t *testing.T) {
	t.Parallel()
	parts := func() []partRecord {
		return []partRecord{
			{Key: partWeapon, Kind: partWeapon, Count: 1, Sides: 8, DamageType: "damage-type:slashing", Available: true},
			{Key: combat.ExtraDivineSmite, Kind: partExtra, Choosable: true, Count: 2, Sides: 8, DamageType: groupRadiant, Available: true, NeedsSlot: true},
			{Key: combat.ExtraDivineSmiteExtra, Kind: partExtra, Count: 1, Sides: 8, DamageType: groupRadiant, Available: true, Conditional: true},
		}
	}
	for _, tt := range []struct {
		name    string
		target  string
		counted bool
	}{{"a humanoid", "humanoid", false}, {"an undead", "undead", true}, {"a fiend", "fiend", true}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := groupCase{parts: parts(), marks: []string{combat.ExtraDivineSmite}, target: link.Traits{CreatureType: tt.target}}
			g.typed = typedSums{byPart: map[string]int{partWeapon: 4, combat.ExtraDivineSmiteExtra: 5}}
			wantRange(t, tt.name, g, groupRadiant, 2, 16)
			g.typed.byType = map[string]int{groupRadiant: 16}
			got, err := g.roll()
			if err != nil {
				t.Fatalf("rollParts() error = %v", err)
			}
			var smite, die *partRoll
			for i := range got.rolls {
				switch got.rolls[i].Key {
				case combat.ExtraDivineSmite:
					smite = &got.rolls[i]
				case combat.ExtraDivineSmiteExtra:
					die = &got.rolls[i]
				}
			}
			if smite == nil || die == nil || smite.Sum != 16 || die.Sum != 5 || die.Counted != tt.counted {
				t.Fatalf("rolls = %+v, want the smite carrying the group's 16 and the die its own 5, counted %v", got.rolls, tt.counted)
			}
			if want := 16 + 5; tt.counted && got.byType[groupRadiant] != want || !tt.counted && got.byType[groupRadiant] != 16 {
				t.Errorf("radiant = %d, want the group alone, or with the die %d against an undead or a fiend", got.byType[groupRadiant], want)
			}
			g.typed.byPart = map[string]int{partWeapon: 4} // the die has no sum of its own: the group does not cover it
			if _, err := g.roll(); err == nil || !strings.Contains(err.Error(), "typed_parts needs a sum for "+combat.ExtraDivineSmiteExtra) {
				t.Errorf("no sum for the die: error = %v, want it asked for by its key", err)
			}
		})
	}
}

// What does not count is not in the group's range: an extra nobody marked and a part the master took out.
func TestATypedGroupLeavesOutAnUnmarkedExtraAndARemovedPart(t *testing.T) {
	t.Parallel()
	g := groupCase{parts: []partRecord{
		{Key: partWeapon, Kind: partWeapon, Count: 1, Sides: 8, Flat: 3, DamageType: groupPiercing, Available: true},
		{Key: "unmarked", Kind: partExtra, Choosable: true, Count: 1, Sides: 6, DamageType: groupPiercing, Available: true},
		{Key: "spell:gone", Kind: partAuto, Count: 1, Sides: 4, DamageType: groupPiercing, Available: true, Removed: true},
	}}
	wantRange(t, "the weapon alone", g, groupPiercing, 1, 8)
	g.typed.byType = map[string]int{groupPiercing: 8}
	got, err := g.roll()
	if err != nil || len(got.rolls) != 1 || got.byType[groupPiercing] != 11 {
		t.Fatalf("rollParts() = %+v, %v, want the weapon's roll alone, 8 and the flat 3", got, err)
	}
}

// The range of a group is the dice the parts roll: the weapon's extra dice (Brutal Critical), and a part that keeps
// its dice on a critical hit.
func TestATypedGroupRangeCountsTheDiceEachPartRolls(t *testing.T) {
	t.Parallel()
	weapon := partRecord{Key: partWeapon, Kind: partWeapon, Count: 1, Sides: 8, Flat: 3, DamageType: groupPiercing, Available: true}
	g := groupCase{parts: []partRecord{weapon}, extraDice: 2}
	wantRange(t, "the weapon with two extra dice", g, groupPiercing, 3, 24)
	keeps := partRecord{Key: "keeps", Kind: partExtra, Choosable: true, Count: 1, Sides: 6, DamageType: groupPiercing, Available: true, NoCrit: true}
	g = groupCase{parts: []partRecord{weapon, keeps}, marks: []string{"keeps"}, critical: true}
	wantRange(t, "a critical hit with a part that keeps its dice", g, groupPiercing, 3, 22) // 2d8 doubled and 1d6 as it is
}

// The master cannot take out a part a typed damage-type sum covers: the sum sits on the group's first part, so taking
// out that one would drop the group and taking out another would take nothing. A part of no group comes out as before.
func TestAPartOfATypedGroupCannotBeTakenOut(t *testing.T) {
	t.Parallel()
	parts := []partRecord{
		{Key: partWeapon, Kind: partWeapon, Count: 1, Sides: 8, DamageType: "damage-type:slashing", Available: true},
		{Key: "radiant-edge", Kind: partExtra, Choosable: true, Count: 1, Sides: 4, DamageType: groupRadiant, Available: true},
		{Key: "radiant-two", Kind: partExtra, Choosable: true, Count: 1, Sides: 6, DamageType: groupRadiant, Available: true},
		{Key: "fire-edge", Kind: partExtra, Choosable: true, Count: 1, Sides: 4, DamageType: groupFire, Available: true},
	}
	roll := func() partsRoll {
		got, err := groupCase{
			parts: parts, marks: []string{"radiant-edge", "radiant-two", "fire-edge"},
			typed: typedSums{byType: map[string]int{groupRadiant: 6}, byPart: map[string]int{partWeapon: 4, "fire-edge": 2}},
		}.roll()
		if err != nil {
			t.Fatalf("rollParts() error = %v", err)
		}
		return got
	}
	for _, tt := range []struct {
		name, key string
		want      bool
	}{
		{"the holder of the group", "radiant-edge", false},
		{"another part of the group", "radiant-two", false},
		{"a part of no group", "fire-edge", true},
	} {
		got := roll()
		if ok := takeOut(got.parts, got.rolls, tt.key); ok != tt.want {
			t.Errorf("%s: takeOut = %v, want %v", tt.name, ok, tt.want)
		}
		for i, r := range got.rolls {
			if r.Key == tt.key && (got.parts[i].Removed == !tt.want || r.Counted != !tt.want) {
				t.Errorf("%s: removed %v, counted %v after takeOut %v", tt.name, got.parts[i].Removed, r.Counted, tt.want)
			}
		}
	}
}

// The master takes Sneak Attack out of a damage rolled with a damage-type sum that covers it: refused with the reason
// of a part that cannot come out, and the damage, the log and the target are as they were. The same Sneak Attack rolled
// with a sum of its own comes out as always.
func TestTheMasterCannotTakeSneakAttackOutOfATypedGroupButCanOutOfItsOwnSum(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		typed []*playv1.TypedPart
		ok    bool
	}{
		{"covered by the piercing group", []*playv1.TypedPart{{DamageType: groupPiercing, Sum: 7}}, false},
		{"with a sum of its own", []*playv1.TypedPart{{PartKey: "weapon", Sum: 3}, {PartKey: "sneak-attack", Sum: 4}}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a, _, shrub := partsFight(t, func(a *armed) {
				a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:rogue", "race:human", 3, abilities(16, 16, 14, 12, 10), []string{rapier}, nil)
			}, "monster:awakened-shrub")
			e := a.passTo(t, a.get(t, a.master), "Toren")
			a.setConditions(t, e, shrub, "condition:restrained")
			hit := a.mustAttack(t, a.caio, e, "Toren", rapier, shrub, func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
			rolled := a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), extrasMarked([]*playv1.SelectedExtra{{Key: "sneak-attack"}}, tt.typed...))
			hp := func() int32 { return byLabel(t, a.get(t, a.master), shrub).GetHitPointsCurrent() }
			before := hp()
			res, err := a.master.combat.RemoveDamagePart(t.Context(), connect.NewRequest(&playv1.RemoveDamagePartRequest{
				CampaignId: a.campaignID, EncounterId: e.GetId(), PendingDamageId: rolled.GetPendingDamage().GetId(), PartKey: "sneak-attack", Reason: "motivo", IdempotencyKey: newKey(),
			}))
			if tt.ok {
				if err != nil {
					t.Fatalf("RemoveDamagePart() error = %v", err)
				}
				if hp() <= before || res.Msg.GetPendingDamage().GetAmount() >= rolled.GetPendingDamage().GetAmount() {
					t.Errorf("the removal left hit points %d (were %d) and the damage %d (was %d), want less damage", hp(), before, res.Msg.GetPendingDamage().GetAmount(), rolled.GetPendingDamage().GetAmount())
				}
				return
			}
			blocked := wantBlockedBy(t, "RemoveDamagePart(a part of a group)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_PART_NOT_REMOVABLE)
			if !strings.Contains(err.Error(), "damage-type sum") || blocked == nil {
				t.Errorf("error = %v, want it to say a damage-type sum covers the part", err)
			}
			if hp() != before {
				t.Errorf("hit points = %d after the refusal, want %d", hp(), before)
			}
		})
	}
}

// hiddenHex is the damage of a spell's hit effect hidden from the other players: 1d6 piercing.
func hiddenHex() partRecord {
	return partRecord{
		Key: "spell:hunters-hex", Label: "Marca de Nanquim", Kind: partAuto, Count: 1, Sides: 6, DamageType: groupPiercing, Source: "spell:hunters-hex",
		Available: true, Table: tableHitEffect, Effect: "row-1", Hide: true,
	}
}

func weaponPiercing() partRecord {
	return partRecord{Key: partWeapon, Label: "Rapieira", Kind: partWeapon, Count: 1, Sides: 8, Flat: 3, DamageType: groupPiercing, Available: true}
}

// "Hidden" is hidden from the OTHER players: the roller is the owner or the master, who read every part whole. For the
// roller a group that holds a hidden part is a group like any other: its range shows its bounds, a part_key typed for the
// hidden part works, and a type whose dice are all hidden has dice.
func TestAGroupWithAHiddenPartIsAGroupLikeAnyOtherForTheRoller(t *testing.T) {
	t.Parallel()
	for _, order := range []string{"hidden first", "hidden last"} {
		parts := []partRecord{hiddenHex(), weaponPiercing()}
		if order == "hidden last" {
			parts = []partRecord{weaponPiercing(), hiddenHex()}
		}
		for _, n := range []int{1, 99} { // the group is 2 to 14
			_, err := groupCase{parts: parts, typed: typedSums{byType: map[string]int{groupPiercing: n}}}.roll()
			if connect.CodeOf(err) != connect.CodeInvalidArgument || err == nil || !strings.Contains(err.Error(), "the sum of damage-type:piercing must be 2 to 14") {
				t.Errorf("%s, sum %d: error = %v, want invalid_argument with the bounds 2 to 14", order, n, err)
			}
		}
		// A part_key typed for the hidden part beside its type's sum is the same refusal as for any part of the group.
		_, err := groupCase{parts: parts, typed: typedSums{byType: map[string]int{groupPiercing: 9}, byPart: map[string]int{"spell:hunters-hex": 5}}}.roll()
		if connect.CodeOf(err) != connect.CodeInvalidArgument || err == nil || !strings.Contains(err.Error(), "names damage-type:piercing and a part of it (spell:hunters-hex)") {
			t.Errorf("%s: error = %v, want the refusal naming the type and the part", order, err)
		}
		// A key of no part is still passed over.
		if got, err := (groupCase{parts: parts, typed: typedSums{byType: map[string]int{groupPiercing: 9}, byPart: map[string]int{"spell:not-here": 5}}}.roll()); err != nil || got.byType[groupPiercing] != 12 {
			t.Errorf("%s, a key of no part: byType = %v, error = %v, want the group's 9 and the flat 3", order, got.byType, err)
		}
		// With no group typed, each dice part takes the sum typed for its key, the hidden one too.
		got, err := groupCase{parts: parts, typed: typedSums{byPart: map[string]int{partWeapon: 4, "spell:hunters-hex": 5}}}.roll()
		if err != nil || got.byType[groupPiercing] != 12 {
			t.Errorf("%s, part keys: byType = %v, error = %v, want 4 + 5 and the flat 3", order, got.byType, err)
		}
	}
	// A type whose only dice part is hidden has dice: its bounds are shown and a sum in them is taken.
	slashing := partRecord{Key: partWeapon, Kind: partWeapon, Count: 1, Sides: 8, DamageType: "damage-type:slashing", Available: true}
	_, err := groupCase{parts: []partRecord{slashing, hiddenHex()}, typed: typedSums{byType: map[string]int{groupPiercing: 99}, byPart: map[string]int{partWeapon: 4}}}.roll()
	if err == nil || !strings.Contains(err.Error(), "the sum of damage-type:piercing must be 1 to 6") {
		t.Errorf("a type of hidden dice: error = %v, want the bounds 1 to 6", err)
	}
	if got, err := (groupCase{parts: []partRecord{slashing, hiddenHex()}, typed: typedSums{byType: map[string]int{groupPiercing: 4}, byPart: map[string]int{partWeapon: 4}}}.roll()); err != nil || got.byType[groupPiercing] != 4 {
		t.Errorf("a type of hidden dice, sum 4: byType = %v, error = %v, want it accepted", got.byType, err)
	}
	// A type with no dice part at all is still refused as such.
	if _, err := (groupCase{parts: []partRecord{slashing}, typed: typedSums{byType: map[string]int{groupFire: 3}, byPart: map[string]int{partWeapon: 4}}}.roll()); err == nil || !strings.Contains(err.Error(), "has no dice in this attack") {
		t.Errorf("a type with no dice: error = %v", err)
	}
	// And a group with no part hidden keeps its numbers.
	_, err = groupCase{parts: []partRecord{weaponPiercing()}, typed: typedSums{byType: map[string]int{groupPiercing: 99}}}.roll()
	if err == nil || !strings.Contains(err.Error(), "must be 1 to 8") {
		t.Errorf("a visible group: error = %v, want its range", err)
	}
}

// The group's sum sits on the first part the others can read; every part hidden, on the first of them.
func TestTheGroupSumIsKeptOnTheFirstPartTheOthersCanRead(t *testing.T) {
	t.Parallel()
	sums := func(parts ...partRecord) map[string]int32 {
		got, err := groupCase{parts: parts, typed: typedSums{byType: map[string]int{groupPiercing: 9}, byPart: map[string]int{"other": 1}}}.roll()
		if err != nil {
			t.Fatalf("rollParts() error = %v", err)
		}
		out := map[string]int32{}
		for _, r := range got.rolls {
			out[r.Key] = r.Sum
		}
		return out
	}
	hex2 := hiddenHex()
	hex2.Key, hex2.Effect = "spell:second", "row-2"
	if got := sums(hiddenHex(), weaponPiercing()); got[partWeapon] != 9 || got["spell:hunters-hex"] != 0 {
		t.Errorf("hidden first: sums %v, want the weapon carrying 9", got)
	}
	if got := sums(hiddenHex(), hex2); got["spell:hunters-hex"] != 9 || got["spell:second"] != 0 {
		t.Errorf("all hidden: sums %v, want the first carrying 9", got)
	}
}

// RN-10: a group that holds a hidden part is one row for the others: the visible parts' names and "Outra fonte", the damage
// type and the group's sum (13: 1d8 and 1d6 and a flat 3 make 16 of it), never the hidden effect's key or name and never a
// 0 that was not rolled. The owner and the master read the parts as before.
func TestRN10_TheOthersReadAGroupThatHoldsAHiddenPartAsOneRow(t *testing.T) {
	t.Parallel()
	read := func(parts []partRecord, viewer string) string {
		got, err := groupCase{parts: parts, typed: typedSums{byType: map[string]int{groupPiercing: 13}}}.roll()
		if err != nil {
			t.Fatalf("rollParts() error = %v", err)
		}
		p := playdb.PendingDamage{Parts: mustJSON(got.parts), PartRolls: mustJSON(got.rolls)}
		return jsonOf(&playv1.PendingDamage{PartRolls: partRollsProto(p, viewer == "master", viewer != "other", dtPT)})
	}
	canary := regexp.MustCompile(`"sum":\s*13\b`)
	for _, order := range [][]partRecord{{weaponPiercing(), hiddenHex()}, {hiddenHex(), weaponPiercing()}} {
		for _, viewer := range []string{"owner", "master"} {
			if text := read(order, viewer); !canary.MatchString(text) {
				t.Errorf("the %s reads %s: the canary is unreachable", viewer, text)
			}
		}
		text := read(order, "other")
		if !canary.MatchString(text) || !strings.Contains(text, "Rapieira + Outra fonte") || strings.Contains(text, "hunters-hex") || strings.Contains(text, "Marca de Nanquim") || strings.Contains(text, "row-1") {
			t.Errorf("the others read %s of a group that holds a hidden part, want one row with the sum and no word of the spell", text)
		}
	}
	visible := hiddenHex()
	visible.Hide = false
	if text := read([]partRecord{weaponPiercing(), visible}, "other"); !canary.MatchString(text) {
		t.Errorf("the others read %s of a group with no hidden part, want its sum", text)
	}
}

// Reading the typed parts: the two ways of naming nothing and naming both have their own words, the repeated key keeps
// its own, and a refusal reads nothing.
func TestTypedPartsOfWordsEachRefusalItsOwnWay(t *testing.T) {
	t.Parallel()
	part := func(key, typ string) *playv1.TypedPart {
		return &playv1.TypedPart{PartKey: key, DamageType: typ, Sum: 3}
	}
	const once = "typed_parts must name each part once"
	for name, tt := range map[string]struct {
		raw       []*playv1.TypedPart
		contains  string
		repeating bool
	}{
		"neither":               {[]*playv1.TypedPart{part("", "")}, "must name a part_key or a damage_type", false},
		"both":                  {[]*playv1.TypedPart{part(partWeapon, groupPiercing)}, "not both", false},
		"the same key twice":    {[]*playv1.TypedPart{part(partWeapon, ""), part(partWeapon, "")}, once, true},
		"the same type twice":   {[]*playv1.TypedPart{part("", groupPiercing), part("", groupPiercing)}, once, true},
		"a type not a damage":   {[]*playv1.TypedPart{part("", "condition:blinded")}, "damage_type must be a damage-type: key", false},
		"a refusal after a key": {[]*playv1.TypedPart{part(partWeapon, ""), part("", "")}, "must name a part_key or a damage_type", false},
	} {
		got, err := typedPartsOf(tt.raw)
		if connect.CodeOf(err) != connect.CodeInvalidArgument || err == nil || !strings.Contains(err.Error(), tt.contains) {
			t.Errorf("%s: error = %v, want invalid_argument with %q", name, err, tt.contains)
		}
		if !tt.repeating && err != nil && strings.Contains(err.Error(), once) {
			t.Errorf("%s: error = %v, want words of its own, not the repeated key's", name, err)
		}
		if got.byPart != nil || got.byType != nil {
			t.Errorf("%s: typedPartsOf() kept %+v with an error, want nothing", name, got)
		}
	}
}

// Two groups that are both wrong are refused in the order of their keys, the same every time.
func TestTypedGroupsAreCheckedInTheOrderOfTheirTypes(t *testing.T) {
	t.Parallel()
	parts := []partRecord{weaponPiercing(), {Key: "fire-edge", Kind: partExtra, Choosable: true, Count: 1, Sides: 4, DamageType: groupFire, Available: true}}
	for i := range 60 {
		_, err := groupCase{parts: parts, marks: []string{"fire-edge"}, typed: typedSums{byType: map[string]int{groupPiercing: 99, groupFire: 99}}}.roll()
		if err == nil || !strings.Contains(err.Error(), "the sum of "+groupFire+" must be 1 to 4") {
			t.Fatalf("round %d: error = %v, want the first type by key (fire) named", i, err)
		}
	}
}

// Two casters of the same spell give one attacker two parts with the same key: each is its own row (the effect's), so the
// group's sum sits on one part only and is counted once, and the parts keep apart in the others' view.
func TestTwoCastersOfOneSpellAreTwoPartsOfOneGroup(t *testing.T) {
	t.Parallel()
	first, second := hiddenHex(), hiddenHex()
	second.Effect, second.Hide = "row-2", false
	first.Hide = false
	got, err := groupCase{
		parts: []partRecord{first, second, weaponPiercing()},
		typed: typedSums{byType: map[string]int{groupPiercing: 13}},
	}.roll()
	if err != nil {
		t.Fatalf("rollParts() error = %v", err)
	}
	if got.byType[groupPiercing] != 16 {
		t.Errorf("byType = %v, want the group's 13 and the flat 3 counted once", got.byType)
	}
	carriers := 0
	for _, r := range got.rolls {
		if r.Sum != 0 {
			carriers++
		}
		if r.Group != groupPiercing {
			t.Errorf("%s (%s) has group %q, want the group of its type", r.Key, r.Effect, r.Group)
		}
	}
	if carriers != 1 {
		t.Errorf("%d parts carry the group's sum, want one", carriers)
	}
}
