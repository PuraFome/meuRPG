package play

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Characterization of Metamagic on a casting, of the placed-area preview and of the hit
// point spells, written before the functions behind them were split into phases: each
// refusal with its code, reason and exact message. These tests need the database
// (MEURPG_TEST_DATABASE_URL).

// wantCombatCastMessage fails unless err is a Connect error with this code and this message.
func wantCombatCastMessage(t *testing.T, call string, err error, code connect.Code, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s succeeded, want %v %q", call, code, msg)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != code || ce.Message() != msg {
		t.Fatalf("%s error = %v, want %v %q", call, err, code, msg)
	}
}

// TestMetamagicRefusalsSayWhy: every refusal of a Metamagic choice, in the order the checks run,
// with its message, and that none of them spends a slot or a point.
func TestMetamagicRefusalsSayWhy(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.sorcererFight(t, map[string][2]int32{"Nael": {4, 9}, "Tavo": {9, 3}, "Orla": {10, 3}, "Goblin": {4, 4}, "Capitão Goblin": {9, 9}})
	goblin, captain, tavo := a.id(t, "Goblin"), a.id(t, "Capitão Goblin"), a.id(t, "Tavo")
	const unknown = "the Metamagic is not one the character knows, or it is more than one option (only Empowered Spell joins another)"
	const carefulCount = "Careful Spell protects from one creature up to the Charisma modifier"
	for name, tc := range map[string]struct {
		spell string
		slot  int32
		at    []string
		meta  []*playv1.MetamagicChoice
		msg   string
	}{
		"an option the sorcerer does not know": {fireballSpell, 3, []string{"Goblin"}, []*playv1.MetamagicChoice{choice(rules.MetamagicSubtle)}, unknown},
		"an option that is not one":            {fireballSpell, 3, []string{"Goblin"}, []*playv1.MetamagicChoice{choice("feature:metamagic-nothing")}, unknown},
		"two options that do not join": {
			fireballSpell, 3,
			[]string{"Goblin"},
			[]*playv1.MetamagicChoice{{Key: rules.MetamagicCareful, CarefulIds: []string{goblin}}, {Key: rules.MetamagicTwinned, TargetIds: []string{captain}}},
			unknown,
		},
		"the same option twice": {
			fireballSpell, 3,
			[]string{"Goblin"},
			[]*playv1.MetamagicChoice{{Key: rules.MetamagicCareful, CarefulIds: []string{goblin}}, {Key: rules.MetamagicCareful, CarefulIds: []string{goblin}}},
			unknown,
		},
		"Twinned Spell on an area": {
			fireballSpell, 3,
			[]string{"Goblin"},
			[]*playv1.MetamagicChoice{{Key: rules.MetamagicTwinned, TargetIds: []string{captain}}},
			"the spell does not take this Metamagic option",
		},
		"Twinned Spell with no second target": {
			holdPerson, 2,
			[]string{"Goblin"},
			[]*playv1.MetamagicChoice{choice(rules.MetamagicTwinned)},
			"Twinned Spell takes the spell's target and one more",
		},
		"Twinned Spell with the same target": {
			holdPerson, 2,
			[]string{"Goblin"},
			[]*playv1.MetamagicChoice{{Key: rules.MetamagicTwinned, TargetIds: []string{goblin}}},
			"the second target must be another creature",
		},
		"Careful Spell with nobody": {fireballSpell, 3, []string{"Goblin"}, []*playv1.MetamagicChoice{choice(rules.MetamagicCareful)}, carefulCount},
		"Careful Spell for more than Charisma": {
			fireballSpell, 3,
			[]string{"Goblin"},
			[]*playv1.MetamagicChoice{{Key: rules.MetamagicCareful, CarefulIds: []string{goblin, captain, tavo, a.id(t, "Orla")}}},
			carefulCount,
		},
		"Careful Spell for a creature protected twice": {
			fireballSpell, 3,
			[]string{"Goblin"},
			[]*playv1.MetamagicChoice{{Key: rules.MetamagicCareful, CarefulIds: []string{goblin, goblin}}},
			"a creature is protected twice",
		},
		"Careful Spell for a creature not hit": {
			fireballSpell, 3,
			[]string{"Goblin"},
			[]*playv1.MetamagicChoice{{Key: rules.MetamagicCareful, CarefulIds: []string{tavo}}},
			"a protected creature must be one of the spell's targets",
		},
	} {
		_, err := a.castWith(t, a.ana, e, "Nael", tc.spell, slotOfLevel(tc.slot), a.at(t, tc.at...), tc.meta...)
		wantCombatCastMessage(t, name, err, connect.CodeInvalidArgument, tc.msg)
	}
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.pens), rules.SorceryPointsKey); left != 5 {
		t.Errorf("the refusals spent points: %d left, want 5", left)
	}
	if _, used, _ := slotsOf(a.vitalsOfCharacter(t, a.pens), 3); used != 0 {
		t.Errorf("the refusals spent %d slots of the 3rd level", used)
	}
}

// TestMetamagicTwinnedRefusesADefeatedSecondTarget: the second target of Twinned Spell is
// checked like a first one: a creature that is already down is refused.
func TestMetamagicTwinnedRefusesADefeatedSecondTarget(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.sorcererFight(t, nil)
	a.execSQL(t, `UPDATE combatants SET defeated = true WHERE id = $1`, a.id(t, "Capitão Goblin"))
	_, err := a.castWith(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"),
		&playv1.MetamagicChoice{Key: rules.MetamagicTwinned, TargetIds: []string{a.id(t, "Capitão Goblin")}})
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_DEFEATED)
	wantCombatCastMessage(t, "a defeated second target", err, connect.CodeFailedPrecondition, "a target is defeated")
}

// TestMetamagicHeightenedNeedsASpellTarget: Heightened Spell burdens a creature the spell reaches.
func TestMetamagicHeightenedNeedsASpellTarget(t *testing.T) {
	t.Parallel()
	a := newResourceTableWith(t, []string{rules.MetamagicHeightened, rules.MetamagicSubtle})
	a.sorcererFight(t, map[string][2]int32{"Nael": {4, 9}, "Tavo": {9, 3}, "Orla": {10, 3}, "Goblin": {4, 4}, "Capitão Goblin": {9, 9}})
	_, err := a.castWith(t, a.ana, a.get(t, a.master), "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"),
		&playv1.MetamagicChoice{Key: rules.MetamagicHeightened, HeightenedId: a.id(t, "Tavo")})
	wantCombatCastMessage(t, "Heightened Spell on a creature not hit", err, connect.CodeInvalidArgument, "the creature with disadvantage must be one of the spell's targets")
	_, err = a.castWith(t, a.ana, a.get(t, a.master), "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"), choice(rules.MetamagicHeightened))
	// An empty id is parsed like any combatant's: not found.
	wantCombatCastMessage(t, "Heightened Spell with no creature", err, connect.CodeNotFound, "combatant not found")
}

// TestMetamagicQuickenedNeedsAFreeBonusActionAndRespectsTheSpellLimit: the bonus action must be
// free, and a leveled spell cast with the action leaves no leveled bonus action spell.
func TestMetamagicQuickenedNeedsAFreeBonusActionAndRespectsTheSpellLimit(t *testing.T) {
	t.Parallel()
	a := newResourceTableWith(t, []string{rules.MetamagicQuickened, rules.MetamagicSubtle})
	e := a.sorcererFight(t, nil)
	a.execSQL(t, `UPDATE combatants SET bonus_action_used = true WHERE id = $1`, a.id(t, "Nael"))
	_, err := a.castWith(t, a.ana, e, "Nael", burningHands, slotOfLevel(1), a.at(t, "Goblin"), choice(rules.MetamagicQuickened))
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_BONUS_ACTION_USED)
	wantCombatCastMessage(t, "a used bonus action", err, connect.CodeFailedPrecondition, "the bonus action of this turn is used")

	a.execSQL(t, `UPDATE combatants SET bonus_action_used = false, spell_cast = true WHERE id = $1`, a.id(t, "Nael"))
	_, err = a.castWith(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), choice(rules.MetamagicQuickened))
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_BONUS_ACTION_SPELL_LIMIT)
	wantCombatCastMessage(t, "the bonus action spell limit", err, connect.CodeFailedPrecondition,
		"a spell cast with a bonus action leaves no other spell this turn but a cantrip of 1 action")
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.pens), rules.SorceryPointsKey); left != 5 {
		t.Errorf("the refusals spent points: %d left, want 5", left)
	}
}

// TestPreviewSpellAreaIsRefusedWithItsMessages: the preview's own refusals, in the order its checks run.
func TestPreviewSpellAreaIsRefusedWithItsMessages(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.areaFight(t)
	_, err := c.preview(t, c.ana, "Pensantus", "", slotOfLevel(3), at(20, 7), nil)
	wantCombatCastMessage(t, "no spell key", err, connect.CodeInvalidArgument, "spell_key must name one of the caster's spells")
	_, err = c.preview(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), at(20, 7), nil)
	wantCombatCastMessage(t, "a point for a spell that comes out of the caster", err, connect.CodeInvalidArgument, "this spell comes out of the caster: set direction")
	_, err = c.preview(t, c.ana, "Pensantus", fireball, slotOfLevel(3), nil, nil)
	wantCombatCastMessage(t, "no point for a spell placed at a point", err, connect.CodeInvalidArgument, "this spell is placed at a point: set origin")
	_, err = c.preview(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(-1, 7), nil)
	wantCombatCastMessage(t, "a point outside the map", err, connect.CodeInvalidArgument, "the point of origin must be a square of the map")
	_, err = c.preview(t, c.ana, "Pensantus", fireball, slotOfLevel(3), nil, toward(0, 0))
	wantCombatCastMessage(t, "a direction that is not one of the eight", err, connect.CodeInvalidArgument, "the direction must be one of the eight: dx and dy from -1 to 1, not both 0")

	// The caster must stand on the map.
	c.execSQL(t, `UPDATE combatants SET grid_col = NULL, grid_row = NULL WHERE id = $1`, c.id(t, "Pensantus"))
	_, err = c.preview(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_PLACED)
	wantCombatCastMessage(t, "a caster off the map", err, connect.CodeFailedPrecondition, "the caster must be on the map")
}

// TestPreviewSpellAreaNeedsARunningCombatWithAMap: a combat in setup is not running, and a
// combat without a map has no area to place.
func TestPreviewSpellAreaNeedsARunningCombatWithAMap(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.areaFight(t)
	c.execSQL(t, `UPDATE encounters SET status = 'setup'`)
	_, err := c.preview(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ACTIVE)
	wantCombatCastMessage(t, "a combat in setup", err, connect.CodeFailedPrecondition, "the combat is not running")
	c.execSQL(t, `UPDATE encounters SET status = 'active', mode = 'theatre', grid_columns = 0, grid_rows = 0, map_id = NULL, map_point_id = NULL`)
	_, err = c.preview(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	wantCombatCastMessage(t, "a combat without a map", err, connect.CodeInvalidArgument, "a combat without a map has no area to place: the master decides who is inside")
}

// TestColorSprayOnACreatureThatIsAlreadyBlindedChangesNoCondition: the pool reaches a creature
// that already has the condition: it counts as affected, and its conditions are not written again.
func TestColorSprayOnACreatureThatIsAlreadyBlindedChangesNoCondition(t *testing.T) {
	t.Parallel()
	a := newHPCasters(t)
	e := a.castersFight(t, 1)
	a.stand(t, map[string][2]int32{"Toren": {5, 3}, "Capitão Goblin": {7, 4}})
	a.setConditions(t, e, "Capitão Goblin", blindedC)
	a.h.roller.queue(6, 6, 6, 6, 6, 6)
	res := a.mustCast(t, a.ana, e, "Pensantus", colorSpray, slotOfLevel(1), a.at(t, "Capitão Goblin"), poolInApp)
	if got := effectOf(t, a.get(t, a.master), res.GetCast().GetTargets(), "Capitão Goblin"); got == nil {
		t.Fatal("no effect on the Capitão")
	}
	if conds := byLabel(t, a.get(t, a.master), "Capitão Goblin").GetConditions(); len(conds) != 1 || conds[0] != blindedC {
		t.Errorf("conditions = %v, want the one blinded", conds)
	}
}
