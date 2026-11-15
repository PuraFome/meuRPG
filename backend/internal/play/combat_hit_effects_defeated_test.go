package play

import (
	"testing"
)

// A hit effect's save has something to affect only while the target stands: an NPC the hit's damage killed rolls no
// save (no line, no condition), while a player's character at 0 hit points, dying, still rolls it, because its
// conditions still apply.

func defeatedSaveMod() *tableMod {
	return hitMod(trigEach, func(m *tableMod) {
		m.Save, m.OnFailConditions, m.ConditionDuration = "wis", []string{frightenedKey}, spellEnd
	})
}

func TestAHitSpellAnNPCTheHitKilledRollsNoSave(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, defeatedSaveMod())
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	a.execSQL(t, `UPDATE combatants SET hp_current = 1 WHERE label = 'Goblin'`)
	hit := a.strike(t, e, battleaxe, "Goblin", d20(19))
	a.rollDamage(t, hit, 1, 1, 4) // the weapon, the die of the effect, the save (never rolled)
	if !byLabel(t, a.get(t, a.master), "Goblin").GetDefeated() {
		t.Fatalf("the Goblin is standing: the hit was meant to kill it")
	}
	if got := a.hitRiderEntries(t, a.master, e); len(got) != 0 {
		t.Errorf("the master reads %d save lines of an NPC the hit killed, want none", len(got))
	}
	if a.conditioned(t, "Goblin", frightenedKey) {
		t.Errorf("the dead Goblin is frightened")
	}
	if n := a.count(t, `SELECT count(*) FROM combatant_states s JOIN combatants c ON c.id = s.combatant_id WHERE s.source_key = $1 AND c.label = 'Goblin' AND cardinality(s.condition_keys) > 0`, tableMark); n != 0 {
		t.Errorf("%d condition rows on the dead Goblin, want none", n)
	}
}

func TestAHitSpellACharacterDyingStillRollsTheSave(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, defeatedSaveMod())
	a.hurt(t, a.toren, 0)
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	hit := a.strike(t, e, battleaxe, "Toren", twoD20(19))
	a.rollDamage(t, hit, 1, 1, 4)
	if got := a.hitRiderEntries(t, a.master, e); len(got) != 1 {
		t.Fatalf("the master reads %d save lines of a character at 0 hit points, want 1", len(got))
	}
	if !a.conditioned(t, "Toren", frightenedKey) {
		t.Errorf("Toren at 0 hit points is not frightened after the failed save")
	}
}
