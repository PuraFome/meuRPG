package play

import (
	"fmt"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// These tests freeze what the roll of a damage by parts refuses: an extra that does not
// hold when the damage is rolled (the conditions are read again), a Divine Smite with no
// slot, more than one maneuver, no use left, and typed sums that do not fit the dice.
// They need the database (MEURPG_TEST_DATABASE_URL).

var (
	blockedExtraNotAvailable = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_EXTRA_NOT_AVAILABLE
	blockedNoSlot            = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_SLOT
)

const (
	extraNotAvailableMsg = "that extra does not hold now"
	extraWeaponDamage    = "weapon"
)

// extrasMarked marks the extras of the damage roll, with the sums typed for the dice.
func extrasMarked(marks []*playv1.SelectedExtra, typed ...*playv1.TypedPart) func(*playv1.RollDamageRequest) {
	return func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen = true
		r.SelectedExtras = marks
		r.TypedParts = typed
	}
}

// setCombatantColumn writes a column of a combatant's row, as a past turn would have left it.
func (a *armed) setCombatantColumn(t *testing.T, label, column, value string) {
	t.Helper()
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE combatants SET `+column+` = NULLIF($2, '') WHERE id = $1`, a.id(t, label), value); err != nil {
		t.Fatalf("set %s of %s: %v", column, label, err)
	}
}

// extrasTurnKey is the key of the turn in progress, as the once-per-turn marks keep it.
func extrasTurnKey(e *playv1.Encounter) string {
	return fmt.Sprintf("%d:%s", e.GetRound(), e.GetCurrentCombatantId())
}

// TestRollDamageRefusesASneakAttackThatDoesNotHold: the part the hit stored as not
// available, and the one that was available but whose once-per-turn mark was set before
// the roll, are both refused with the same reason.
func TestRollDamageRefusesASneakAttackThatDoesNotHold(t *testing.T) {
	t.Parallel()
	sneak := []*playv1.SelectedExtra{{Key: "sneak-attack"}}
	typed := []*playv1.TypedPart{{PartKey: extraWeaponDamage, Sum: 3}, {PartKey: "sneak-attack", Sum: 5}}
	for _, tt := range []struct {
		name      string
		markFirst bool // the mark is set before the hit (the part is stored as not available)
	}{
		{"stored as not available", true},
		{"read again at the roll", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a, e := rollsTable(t)
			e = a.passTo(t, e, "Toren")
			a.setConditions(t, e, "Goblin", "condition:restrained")
			if tt.markFirst {
				a.setCombatantColumn(t, "Toren", "sneak_attack_turn", extrasTurnKey(e))
			}
			res := a.mustAttack(t, a.caio, a.get(t, a.caio), "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
			if !tt.markFirst {
				a.setCombatantColumn(t, "Toren", "sneak_attack_turn", extrasTurnKey(e))
			}
			_, err := a.damage(t, a.caio, e, res.GetPendingDamage().GetId(), extrasMarked(sneak, typed...))
			wantBlockedBy(t, "RollDamage(a Sneak Attack that does not hold)", err, blockedExtraNotAvailable)
			wantPartsRefusal(t, "RollDamage(a Sneak Attack that does not hold)", err, connect.CodeFailedPrecondition, extraNotAvailableMsg)
			// Not marking it is fine: the weapon alone rolls.
			if _, err := a.damage(t, a.caio, e, res.GetPendingDamage().GetId(), extrasMarked(nil, &playv1.TypedPart{PartKey: extraWeaponDamage, Sum: 3})); err != nil {
				t.Errorf("RollDamage(the weapon alone) error = %v", err)
			}
		})
	}
}

// TestRollDamageRefusesTypedSumsThatDoNotFit: with physical dice a sum is typed for each
// group of dice, once, inside the dice's range.
func TestRollDamageRefusesTypedSumsThatDoNotFit(t *testing.T) {
	t.Parallel()
	a, e, pendingID := partEditTable(t)
	sneak := []*playv1.SelectedExtra{{Key: "sneak-attack"}}
	roll := func(typed ...*playv1.TypedPart) error {
		_, err := a.damage(t, a.caio, e, pendingID, extrasMarked(sneak, typed...))
		return err
	}
	part := func(key string, sum int32) *playv1.TypedPart { return &playv1.TypedPart{PartKey: key, Sum: sum} }
	wantPartsRefusal(t, "RollDamage(no sum for Sneak Attack)", roll(part(extraWeaponDamage, 3)),
		connect.CodeInvalidArgument, "typed_parts needs a sum for sneak-attack")
	wantPartsRefusal(t, "RollDamage(no sum for the weapon)", roll(part("sneak-attack", 5)),
		connect.CodeInvalidArgument, "typed_parts needs a sum for weapon")
	wantPartsRefusal(t, "RollDamage(the weapon above its dice)", roll(part(extraWeaponDamage, 99), part("sneak-attack", 5)),
		connect.CodeInvalidArgument, "the sum of weapon must be 1 to 8")
	wantPartsRefusal(t, "RollDamage(Sneak Attack below its dice)", roll(part(extraWeaponDamage, 3), part("sneak-attack", 1)),
		connect.CodeInvalidArgument, "the sum of sneak-attack must be 2 to 12")
	wantPartsRefusal(t, "RollDamage(a part twice)", roll(part(extraWeaponDamage, 3), part(extraWeaponDamage, 4)),
		connect.CodeInvalidArgument, "typed_parts must name each part once")
	wantPartsRefusal(t, "RollDamage(a part with no key)", roll(part("", 3)),
		connect.CodeInvalidArgument, "typed_parts must name each part once")
	if err := roll(part(extraWeaponDamage, 3), part("sneak-attack", 5)); err != nil {
		t.Errorf("RollDamage(the sums that fit) error = %v", err)
	}
}

// TestRollDamageRefusesAColossusSlayerAlreadyUsed: the Hunter's once-per-turn mark is
// read again when the damage is rolled.
func TestRollDamageRefusesAColossusSlayerAlreadyUsed(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:ranger", 3, "subclass:hunter"), abilities(16, 14, 14, 10, 10), []string{rapier}, nil,
			[]string{"feature:hunters-prey-colossus-slayer"})
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
	})
	e := a.threeAndAGoblin(t)
	if _, err := a.adjustHP(t, e, "Goblin", damageHP(1)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	res := a.mustAttack(t, a.caio, e, "Toren", rapier, "Goblin", d20(15))
	a.setCombatantColumn(t, "Toren", "colossus_slayer_turn", extrasTurnKey(e))
	_, err := a.damage(t, a.caio, e, res.GetPendingDamage().GetId(),
		extrasMarked([]*playv1.SelectedExtra{{Key: "colossus-slayer"}}, &playv1.TypedPart{PartKey: extraWeaponDamage, Sum: 1}, &playv1.TypedPart{PartKey: "colossus-slayer", Sum: 2}))
	wantBlockedBy(t, "RollDamage(a Colossus Slayer already used)", err, blockedExtraNotAvailable)
	wantPartsRefusal(t, "RollDamage(a Colossus Slayer already used)", err, connect.CodeFailedPrecondition, extraNotAvailableMsg)
}

// TestSelectionCheckerHoldsTheExtrasThatDoNotNeedTheDatabase: the once-per-turn marks, the
// concentration of Hunter's Mark and the maneuvers of a creature that is no player's character
// are read from the attacker alone.
func TestSelectionCheckerHoldsTheExtrasThatDoNotNeedTheDatabase(t *testing.T) {
	t.Parallel()
	const turn = "2:combatant"
	s := &Service{}
	c := &combatTx{enc: playdb.Encounter{Round: 2, CurrentCombatantID: new("combatant")}}
	hunters := "spell:hunters-mark"
	other := "spell:bless"
	mark := turn
	earlier := "1:combatant"
	tests := []struct {
		name     string
		attacker playdb.Combatant
		part     partRecord
		holds    bool
	}{
		{"a part that is not available", playdb.Combatant{}, partRecord{Key: "sneak-attack"}, false},
		{"Sneak Attack not used this turn", playdb.Combatant{}, partRecord{Key: "sneak-attack", Available: true}, true},
		{"Sneak Attack used this turn", playdb.Combatant{SneakAttackTurn: &mark}, partRecord{Key: "sneak-attack", Available: true}, false},
		{"Sneak Attack used in another turn", playdb.Combatant{SneakAttackTurn: &earlier}, partRecord{Key: "sneak-attack", Available: true}, true},
		{"Colossus Slayer not used this turn", playdb.Combatant{}, partRecord{Key: "colossus-slayer", Available: true}, true},
		{"Colossus Slayer used this turn", playdb.Combatant{ColossusSlayerTurn: &mark}, partRecord{Key: "colossus-slayer", Available: true}, false},
		{"Hunter's Mark while concentrating on it", playdb.Combatant{ConcentrationSpell: &hunters}, partRecord{Key: "hunters-mark", Available: true}, true},
		{"Hunter's Mark with no concentration", playdb.Combatant{}, partRecord{Key: "hunters-mark", Available: true}, false},
		{"Hunter's Mark while concentrating on another spell", playdb.Combatant{ConcentrationSpell: &other}, partRecord{Key: "hunters-mark", Available: true}, false},
		{"a maneuver of a creature that is no player's", playdb.Combatant{Kind: kindNPC}, partRecord{Key: "maneuver", Resource: "dice", Available: true}, false},
		{"an extra the rules have no condition for", playdb.Combatant{}, partRecord{Key: "something-else", Available: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := s.selectionChecker(t.Context(), c, "campaign", tt.attacker)(tt.part, nil)
			if tt.holds {
				if err != nil {
					t.Errorf("the check = %v, want the extra to hold", err)
				}
				return
			}
			wantBlockedBy(t, "the check", err, blockedExtraNotAvailable)
			wantPartsRefusal(t, "the check", err, connect.CodeFailedPrecondition, extraNotAvailableMsg)
		})
	}
}

// TestRollDamageRefusesADivineSmiteWithoutASlot: the smite names the slot it spends, a
// level from 1 to 9 that the paladin has free.
func TestRollDamageRefusesADivineSmiteWithoutASlot(t *testing.T) {
	t.Parallel()
	a, _ := smiteTable(t)
	res := a.mustAttack(t, a.caio, a.get(t, a.caio), "Toren", battleaxe, "Esqueleto", d20(15))
	e := a.get(t, a.caio)
	roll := func(marks ...*playv1.SelectedExtra) error {
		_, err := a.damage(t, a.caio, e, res.GetPendingDamage().GetId(), extrasMarked(marks,
			&playv1.TypedPart{PartKey: extraWeaponDamage, Sum: 1}, &playv1.TypedPart{PartKey: "divine-smite", Sum: 2}, &playv1.TypedPart{PartKey: "divine-smite-extra", Sum: 1}))
		return err
	}
	const names = "a Divine Smite names the slot it spends"
	wantPartsRefusal(t, "RollDamage(a smite with no slot)", roll(&playv1.SelectedExtra{Key: "divine-smite"}), connect.CodeInvalidArgument, names)
	wantPartsRefusal(t, "RollDamage(a smite with slot 10)", roll(&playv1.SelectedExtra{Key: "divine-smite", SlotLevel: 10}), connect.CodeInvalidArgument, names)
	err := roll(&playv1.SelectedExtra{Key: "divine-smite", SlotLevel: 3})
	b := wantBlockedBy(t, "RollDamage(a smite with a slot the paladin has not)", err, blockedNoSlot)
	wantPartsRefusal(t, "RollDamage(a smite with a slot the paladin has not)", err, connect.CodeFailedPrecondition, "there is no free spell slot of that level")
	if b.GetMinLevel() != 3 {
		t.Errorf("the refusal names level %d, want the 3 that was asked", b.GetMinLevel())
	}
	err = roll(&playv1.SelectedExtra{Key: "divine-smite", SlotLevel: 1, Pact: true})
	wantBlockedBy(t, "RollDamage(a smite with a pact slot the paladin has not)", err, blockedNoSlot)
	if err := roll(&playv1.SelectedExtra{Key: "divine-smite", SlotLevel: 1}); err != nil {
		t.Errorf("RollDamage(a smite with a free slot) error = %v", err)
	}
}

// TestRollDamageRefusesTwoManeuversAndOneWithNoUseLeft: one maneuver for each attack, and
// the uses are read again when the damage is rolled.
func TestRollDamageRefusesTwoManeuversAndOneWithNoUseLeft(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "4")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")
	pending := m.hit(t, e)
	_, err := m.damage(t, m.ana, e, pending.GetId(), marking(m.heron, m.bull))
	wantBlockedBy(t, "RollDamage(two maneuvers)", err, blockedExtraNotAvailable)
	wantPartsRefusal(t, "RollDamage(two maneuvers)", err, connect.CodeFailedPrecondition, "only one maneuver for each attack")

	one := newManeuverTable(t, "1")
	e = one.passTo(t, one.castersFight(t, 1), "Pensantus")
	pending = one.hit(t, e)
	// The only die is spent between the hit and the roll (the master's correction).
	one.correct(t, one.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.ResourcesUsed = []*playv1.ResourceUsed{{Key: maneuverDiceKey, Used: 1}}
	})
	if got := diceLeft(t, one); got != 0 {
		t.Fatalf("dice left = %d, want 0", got)
	}
	_, err = one.damage(t, one.ana, e, pending.GetId(), marking(one.heron))
	wantBlockedBy(t, "RollDamage(a maneuver with no die left)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES)
	wantPartsRefusal(t, "RollDamage(a maneuver with no die left)", err, connect.CodeFailedPrecondition, "no uses left")
}
