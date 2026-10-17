package play

import (
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Characterization of checkModeOf / baseCheckModeOf (combat_checkmode.go): the
// mode, dice, bonuses and Helps a check or a save gets for a character in a
// running combat, or outside one.

// checkModeFor calls checkModeOf on the open session of the table.
func (a *armed) checkModeFor(t *testing.T, characterID, skillKey, ability string, check bool) checkMode {
	t.Helper()
	var sessionID string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT id FROM game_sessions WHERE campaign_id = $1 AND ended_at IS NULL`, a.campaignID).Scan(&sessionID); err != nil {
		t.Fatalf("find the open session: %v", err)
	}
	cm, err := a.h.svc.checkModeOf(t.Context(), nil, a.campaignID, sessionID, characterID, skillKey, ability, check)
	if err != nil {
		t.Fatalf("checkModeOf(%s) error = %v", skillKey, err)
	}
	return cm
}

func sourceKinds(cm checkMode) []string {
	var out []string
	for _, s := range cm.Sources {
		out = append(out, s.Kind+"/"+effectName(s.Effect))
	}
	return out
}

func effectName(m combat.RollMode) string {
	switch m {
	case combat.ModeAdvantage:
		return "advantage"
	case combat.ModeDisadvantage:
		return "disadvantage"
	}
	return "normal"
}

func diceKeys(cm checkMode) []string {
	var out []string
	for _, d := range cm.Dice {
		out = append(out, d.Key)
	}
	return out
}

func wantCheckMode(t *testing.T, what string, cm checkMode, mode combat.RollMode, sources, dice []string, bonus, helps int) {
	t.Helper()
	var b int
	for _, x := range cm.Bonuses {
		b += x.Value
	}
	if cm.Mode != mode || !slices.Equal(sourceKinds(cm), sources) || !slices.Equal(diceKeys(cm), dice) || b != bonus || len(cm.Helps) != helps {
		t.Errorf("%s: mode %v, sources %v, dice %v, bonus %d, helps %d; want mode %v, sources %v, dice %v, bonus %d, helps %d",
			what, cm.Mode, sourceKinds(cm), diceKeys(cm), b, len(cm.Helps), mode, sources, dice, bonus, helps)
	}
}

func TestCheckMode_InACombat(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	toren, pens := a.toren.GetId(), a.pens.GetId()
	e := c.refresh(t)

	wantCheckMode(t, "plain check", a.checkModeFor(t, toren, "skill:perception", "wis", true), combat.ModeNormal, nil, nil, 0, 0)
	wantCheckMode(t, "plain stealth check", a.checkModeFor(t, toren, skillStealth, "dex", true), combat.ModeNormal, nil, nil, 0, 0)
	wantCheckMode(t, "plain save", a.checkModeFor(t, toren, "", "wis", false), combat.ModeNormal, nil, nil, 0, 0)

	// A condition that gives disadvantage on checks, but not on every save.
	a.setConditions(t, e, "Toren", "condition:poisoned")
	poisoned := a.checkModeFor(t, toren, "skill:perception", "wis", true)
	if poisoned.Mode != combat.ModeDisadvantage || len(poisoned.Sources) != 1 {
		t.Errorf("poisoned check = %v %v, want disadvantage from one source", poisoned.Mode, sourceKinds(poisoned))
	}
	poisonedSave := a.checkModeFor(t, toren, "", "wis", false)
	if poisonedSave.Mode != combat.ModeNormal || len(poisonedSave.Sources) != 0 {
		t.Errorf("poisoned save = %v %v, want a plain roll: poisoned touches checks, not saves", poisonedSave.Mode, sourceKinds(poisonedSave))
	}
	a.setConditions(t, e, "Toren")

	// Bless: a die on saving throws, none on checks.
	a.mustAddEffect(t, e, "spell:bless", []string{"Toren"}, a.rounds(t, 5, "Toren"))
	wantCheckMode(t, "blessed save", a.checkModeFor(t, toren, "", "wis", false), combat.ModeNormal, nil, []string{"spell:bless"}, 0, 0)
	wantCheckMode(t, "blessed check", a.checkModeFor(t, toren, "skill:perception", "wis", true), combat.ModeNormal, nil, nil, 0, 0)
	if cm := a.checkModeFor(t, toren, "", "wis", false); len(cm.Dice) != 1 || cm.Dice[0].CharacterID != toren {
		t.Errorf("blessed save dice = %+v, want one die with the character's id", cm.Dice)
	}

	// A Help for the task is a source of advantage and is spent by the roll; one for another task,
	// or on a save, or for another ally, is not.
	c.advance(t, "Brisa")
	given := helped(t)(c.helpCheck(t, a.bia, "Brisa", "Toren", "skill:perception"))
	helped := a.checkModeFor(t, toren, "skill:perception", "wis", true)
	wantCheckMode(t, "helped check", helped, combat.ModeAdvantage, []string{"help/advantage"}, nil, 0, 1)
	if len(helped.Helps) != 1 || helped.Helps[0] != given.GetId() {
		t.Errorf("helps = %v, want the Help %s", helped.Helps, given.GetId())
	}
	wantCheckMode(t, "check for another task", a.checkModeFor(t, toren, "skill:athletics", "str", true), combat.ModeNormal, nil, nil, 0, 0)
	wantCheckMode(t, "save with a help for a check", a.checkModeFor(t, toren, "skill:perception", "wis", false), combat.ModeNormal, nil, []string{"spell:bless"}, 0, 0)
	wantCheckMode(t, "another ally", a.checkModeFor(t, pens, "skill:perception", "wis", true), combat.ModeNormal, nil, nil, 0, 0)
}

func TestCheckMode_BonusesAndDiceOfEffects(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	toren := a.toren.GetId()
	e := c.refresh(t)

	a.mustAddEffect(t, e, "spell:guidance", []string{"Toren"}, withCaster(t, a, "Pensantus"))
	wantCheckMode(t, "guidance on a check", a.checkModeFor(t, toren, "skill:perception", "wis", true), combat.ModeNormal, nil, []string{"spell:guidance"}, 0, 0)
	wantCheckMode(t, "guidance on a save", a.checkModeFor(t, toren, "", "wis", false), combat.ModeNormal, nil, nil, 0, 0)
	a.mustAddEffect(t, e, "spell:pass-without-trace", []string{"Toren"}, withCaster(t, a, "Pensantus"))
	// The second spell of the same caster ends the first (one concentration): guidance is gone.
	wantCheckMode(t, "pass without trace on stealth", a.checkModeFor(t, toren, skillStealth, "dex", true), combat.ModeNormal, nil, nil, 10, 0)
	wantCheckMode(t, "pass without trace on another skill", a.checkModeFor(t, toren, "skill:perception", "wis", true), combat.ModeNormal, nil, nil, 0, 0)
	wantCheckMode(t, "pass without trace on a save", a.checkModeFor(t, toren, skillStealth, "dex", false), combat.ModeNormal, nil, nil, 0, 0)
}

func TestCheckMode_OutsideACombat(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	// A character that is not a player combatant of the combat (the goblin's NPC sheet) rolls a plain d20.
	wantCheckMode(t, "not in the combat", a.checkModeFor(t, a.goblin.GetId(), "skill:perception", "wis", true), combat.ModeNormal, nil, nil, 0, 0)
	// A combat that ended is no combat.
	e := c.refresh(t)
	a.mustAddEffect(t, e, "spell:bless", []string{"Toren"}, a.rounds(t, 5, "Toren"))
	a.endEncounter(t, e)
	// The effect goes on the character (outsideModeOf reads it there).
	wantCheckMode(t, "after the combat", a.checkModeFor(t, a.toren.GetId(), "", "wis", false), combat.ModeNormal, nil, []string{"spell:bless"}, 0, 0)
}
