package play

import (
	"regexp"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The damage a spell adds to the caster's weapon hits (#577, combat_hit_effects.go): the effect row is the caster's,
// a mark row of the same casting is the target's, a hit offers the damage as an automatic part, and the roll uses it.
// These tests need the database (MEURPG_TEST_DATABASE_URL). The spell is "Marca de Nanquim" of
// lasting_effects_tablespell_test.go, cast as a bonus action by Pensantus, a wizard 9 with a battleaxe and a shortbow.

const (
	hitModKind = rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_WEAPON_HIT_DAMAGE
	trigNext   = rulesv1.TableHitTrigger_TABLE_HIT_TRIGGER_NEXT_HIT
	trigMelee  = rulesv1.TableHitTrigger_TABLE_HIT_TRIGGER_NEXT_MELEE_HIT
	trigEach   = rulesv1.TableHitTrigger_TABLE_HIT_TRIGGER_EACH_HIT
	trigMarked = rulesv1.TableHitTrigger_TABLE_HIT_TRIGGER_MARKED_TARGET
)

// hitMod is a weapon_hit_damage of one d8 on the trigger; edit changes it.
func hitMod(trigger rulesv1.TableHitTrigger, edit ...func(*tableMod)) *tableMod {
	m := &tableMod{Kind: hitModKind, Trigger: trigger, Die: 8, Dice: 1}
	for _, e := range edit {
		e(m)
	}
	return m
}

// markShape makes the spell a ranged one on a creature (the marked_target's).
func markShape(s *rulesv1.TableSpell) {
	s.Range = &rulesv1.TableSpellRange{Kind: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED, DistanceFt: 60}
	s.Target = tableTarget(rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURE)
}

// newHitCasters is the party with the spell Marca de Nanquim, a self spell for a minute of concentration (shape
// reshapes it) that carries the modifiers.
func newHitCasters(t *testing.T, shape func(*rulesv1.TableSpell), mods ...*tableMod) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		sp := tableSpell("Marca de Nanquim", 1, "class:wizard", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF, tableTarget(rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_SELF))
		sp.Concentration = true
		sp.CastingTime = &rulesv1.TableSpellCastingTime{Unit: rulesv1.CastingTimeUnit_CASTING_TIME_UNIT_BONUS_ACTION, Amount: 1}
		sp.Duration = &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_TIMED, Amount: 1, Unit: rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_MINUTE, UpTo: true}
		sp.Effect = &rulesv1.TableSpellEffect{Modifiers: mods, Visibility: rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC}
		if shape != nil {
			shape(sp)
		}
		if _, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSpell{TableSpell: sp},
		})); err != nil {
			t.Fatalf("CreateTableEntry(Marca de Nanquim) error = %v", err)
		}
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		book := []string{tableMark}
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 9,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, []string{battleaxe, shortbowKey}, []string{fireBolt}, book, book)
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil, nil)
	})
}

// hitFight is a combat in a theatre of the mind with the Capitão and the Goblin, Pensantus first.
func (a *armed) hitFight(t *testing.T) *playv1.Encounter {
	t.Helper()
	return a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.capitao.GetId()}, {CharacterId: a.goblin.GetId()}},
		npcRolls: []int{1, 1},
		players:  map[string]int32{"Pensantus": 20, "Toren": 10, "Brisa": 5},
		reveal:   []string{"Capitão Goblin", "Goblin"},
		theatre:  true,
	})
}

// castMark casts the spell in the given slot on the targets (none for a self spell).
func (a *armed) castMark(t *testing.T, e *playv1.Encounter, slot int32, targets ...string) {
	t.Helper()
	a.mustCast(t, a.ana, e, "Pensantus", tableMark, slotOfLevel(slot), a.at(t, targets...), noCastRoll)
}

// nextTurn is Pensantus's next turn.
func (a *armed) nextTurn(t *testing.T, e *playv1.Encounter) *playv1.Encounter {
	t.Helper()
	if _, err := a.endTurn(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	return a.passTo(t, e, "Pensantus")
}

// strike is a hit of Pensantus on the target with the weapon: its pending damage.
func (a *armed) strike(t *testing.T, e *playv1.Encounter, key, target string, roll func(*playv1.RollAttackRequest)) *playv1.PendingDamage {
	t.Helper()
	return a.mustAttack(t, a.ana, e, "Pensantus", key, target, roll).GetPendingDamage()
}

// hitPart is the spell's part of a pending damage, nil when it is not there.
func hitPart(p *playv1.PendingDamage) *playv1.DamagePart {
	for _, d := range p.GetParts() {
		if d.GetKey() == tableMark {
			return d
		}
	}
	return nil
}

func hitRoll(p *playv1.PendingDamage) *playv1.DamagePartRoll {
	for _, r := range p.GetPartRolls() {
		if r.GetPartKey() == tableMark {
			return r
		}
	}
	return nil
}

// rollDamage rolls the damage in the app with the faces the roller gives: the weapon's first, then the spell's.
func (a *armed) rollDamage(t *testing.T, p *playv1.PendingDamage, faces ...int) *playv1.PendingDamage {
	t.Helper()
	a.h.roller.queue(faces...)
	return a.mustDamage(t, a.ana, a.get(t, a.master), p.GetId(), inAppDamage).GetPendingDamage()
}

// count is a number out of a query.
func (a *armed) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := a.h.pool.QueryRow(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return n
}

// rows are the rows of the spell's effect: all, and the spent ones.
func (a *armed) rows(t *testing.T) (all, spent int) {
	t.Helper()
	return a.count(t, `SELECT count(*) FROM combatant_states WHERE source_key = $1`, tableMark),
		a.count(t, `SELECT count(*) FROM combatant_states WHERE source_key = $1 AND spent_at IS NOT NULL`, tableMark)
}

// rowsOn are the rows of the spell's effect on a combatant.
func (a *armed) rowsOn(t *testing.T, label string) int {
	t.Helper()
	return a.count(t, `SELECT count(*) FROM combatant_states s JOIN combatants c ON c.id = s.combatant_id WHERE s.source_key = $1 AND c.label = $2`, tableMark, label)
}

func wantHitPart(t *testing.T, what string, p *playv1.PendingDamage, want bool) *playv1.DamagePart {
	t.Helper()
	part := hitPart(p)
	if (part != nil) != want {
		t.Fatalf("%s: the spell's part = %v, want it present: %v", what, part, want)
	}
	if part != nil && (!part.GetAuto() || part.GetChoosable()) {
		t.Errorf("%s: the part = %v, want an automatic one the player cannot choose off", what, part)
	}
	return part
}

// A cast puts the effect on the caster, and a next_hit is used by the first weapon hit only: a miss leaves it, the
// hit offers it as an automatic part of the dice the spell says, the roll rolls them and spends it, and the next hit
// has no part.
func TestAHitSpellNextHitIsSpentOnceAndAMissLeavesIt(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigNext, func(m *tableMod) { m.Dice = 2 }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	if all, spent := a.rows(t); all != 1 || spent != 0 || a.rowsOn(t, "Pensantus") != 1 {
		t.Fatalf("after the cast: %d rows, %d spent, want the caster's one", all, spent)
	}
	if miss := a.mustAttack(t, a.ana, e, "Pensantus", battleaxe, "Capitão Goblin", d20(1)); miss.GetPendingDamage().GetId() != "" {
		t.Fatalf("a natural 1 = %v, want a miss with no damage", miss.GetRoll().GetOutcome())
	}
	if _, spent := a.rows(t); spent != 0 {
		t.Fatalf("a miss spent the effect")
	}
	e = a.nextTurn(t, e)

	hit := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	part := wantHitPart(t, "the hit", hit, true)
	if part.GetDiceCount() != 2 || part.GetDiceSides() != 8 || part.GetDamageTypeKey() != hit.GetDamageTypeKey() {
		t.Errorf("the part = %v, want 2d8 of the weapon's type", part)
	}
	dmg := a.rollDamage(t, hit, 3, 4, 5)
	if r := hitRoll(dmg); r == nil || r.GetSum() != 9 || len(r.GetFaces()) != 2 {
		t.Errorf("the part rolled %v, want 2d8 of 4 and 5", r)
	}
	if all, spent := a.rows(t); all != 1 || spent != 1 {
		t.Errorf("after the roll: %d rows, %d spent, want the one, spent", all, spent)
	}
	e = a.nextTurn(t, e)
	wantHitPart(t, "the next hit", a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), false)
}

// The master's undo of the damage gives the effect back, and the damage that waits is offered with its part again.
func TestAHitSpellUndoOfTheDamageGivesTheEffectBack(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigNext))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	hit := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	a.rollDamage(t, hit, 3, 4)
	if _, spent := a.rows(t); spent != 1 {
		t.Fatalf("spent = %d, want 1", spent)
	}
	a.undoLast(t, e)
	if all, spent := a.rows(t); all != 1 || spent != 0 {
		t.Errorf("after the undo: %d rows, %d spent, want the one, not spent", all, spent)
	}
	again := a.rollDamage(t, hit, 6, 2)
	if r := hitRoll(again); r == nil || r.GetSum() != 2 {
		t.Errorf("the damage rolled again = %v, want the part back with its die", r)
	}
	if _, spent := a.rows(t); spent != 1 {
		t.Errorf("spent = %d after the roll again, want 1", spent)
	}
}

// The undo of the cast takes the effect off, the pair of a marked_target included.
func TestAHitSpellUndoOfTheCastRemovesBothRows(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, markShape, hitMod(trigMarked))
	e := a.hitFight(t)
	a.castMark(t, e, 1, "Goblin")
	if all, _ := a.rows(t); all != 2 || a.rowsOn(t, "Pensantus") != 1 || a.rowsOn(t, "Goblin") != 1 {
		t.Fatalf("after the cast: %d rows, want the caster's and the goblin's", all)
	}
	a.undoLast(t, e)
	if all, _ := a.rows(t); all != 0 {
		t.Errorf("after the undo of the cast: %d rows, want none", all)
	}
}

// A next_melee_hit stays after a ranged hit and is spent by a melee one.
func TestAHitSpellNextMeleeHitWaitsForAMeleeHit(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigMelee))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	ranged := a.strike(t, e, shortbowKey, "Capitão Goblin", d20(19))
	wantHitPart(t, "the ranged hit", ranged, false)
	a.rollDamage(t, ranged, 3)
	if _, spent := a.rows(t); spent != 0 {
		t.Fatalf("a ranged hit spent the effect")
	}
	e = a.nextTurn(t, e)
	melee := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	wantHitPart(t, "the melee hit", melee, true)
	a.rollDamage(t, melee, 3, 4)
	if _, spent := a.rows(t); spent != 1 {
		t.Errorf("spent = %d after the melee hit, want 1", spent)
	}
}

// An each_hit adds to every hit while the concentration lasts and to none after it.
func TestAHitSpellEachHitEveryHitAndGoneWithTheConcentration(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigEach))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	for range 2 {
		hit := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
		wantHitPart(t, "a hit while it lasts", hit, true)
		a.rollDamage(t, hit, 3, 4)
		e = a.nextTurn(t, e)
	}
	if _, spent := a.rows(t); spent != 0 {
		t.Errorf("an each_hit was spent")
	}
	if _, err := a.ana.combat.EndConcentration(t.Context(), connect.NewRequest(&playv1.EndConcentrationRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Pensantus"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("EndConcentration() error = %v", err)
	}
	if all, _ := a.rows(t); all != 0 {
		t.Errorf("after the concentration: %d rows, want none", all)
	}
	wantHitPart(t, "a hit after it", a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), false)
}

// A marked_target adds to the hits on the creature the cast marked, not on another, and a recast moves the mark; the
// concentration ending takes the pair.
func TestAHitSpellMarkedTargetOnlyHitsTheMarkedCreatureAndMoves(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, markShape, hitMod(trigMarked))
	e := a.hitFight(t)
	a.castMark(t, e, 1, "Goblin")
	wantHitPart(t, "the unmarked", a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), false)
	e = a.nextTurn(t, e)
	marked := a.strike(t, e, battleaxe, "Goblin", d20(19))
	wantHitPart(t, "the marked", marked, true)
	a.rollDamage(t, marked, 1, 1)
	e = a.nextTurn(t, e)

	a.castMark(t, e, 1, "Capitão Goblin") // the mark moves
	if all, _ := a.rows(t); all != 2 || a.rowsOn(t, "Goblin") != 0 || a.rowsOn(t, "Capitão Goblin") != 1 {
		t.Fatalf("after the recast: %d rows (goblin %d, capitão %d), want the caster's and the capitão's", all, a.rowsOn(t, "Goblin"), a.rowsOn(t, "Capitão Goblin"))
	}
	wantHitPart(t, "the capitão after the recast", a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), true)
	e = a.nextTurn(t, e)
	wantHitPart(t, "the goblin after the recast", a.strike(t, e, battleaxe, "Goblin", d20(19)), false)

	if _, err := a.ana.combat.EndConcentration(t.Context(), connect.NewRequest(&playv1.EndConcentrationRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Pensantus"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("EndConcentration() error = %v", err)
	}
	if all, _ := a.rows(t); all != 0 {
		t.Errorf("after the concentration: %d rows, want none (the mark with it)", all)
	}
}

// The dice grow with the slot above the spell's level and stop at the cap: a spell of level 1 with 1 die and 1 more
// per slot level, at most 3, cast in the slots 1, 2, 3 and 5.
func TestAHitSpellDiceGrowWithTheSlotUpToTheCap(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigEach, func(m *tableMod) { m.PerSlotDice, m.MaxDice = 1, 3 }))
	e := a.hitFight(t)
	for _, c := range []struct{ slot, dice int32 }{{1, 1}, {2, 2}, {3, 3}, {5, 3}} {
		a.castMark(t, e, c.slot)
		hit := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
		if part := wantHitPart(t, "a hit", hit, true); part.GetDiceCount() != c.dice {
			t.Errorf("slot %d: %d dice, want %d", c.slot, part.GetDiceCount(), c.dice)
		}
		a.rollDamage(t, hit, 1, 1, 1, 1)
		e = a.nextTurn(t, e)
	}
}

// A critical hit doubles the dice of the spell and not the fixed damage of the weapon.
func TestAHitSpellCriticalDoublesTheDiceNotTheFixedDamage(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigNext))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	hit := a.strike(t, e, battleaxe, "Capitão Goblin", d20(20))
	if part := wantHitPart(t, "the critical", hit, true); part.GetDiceCount() != 2 || !part.GetDoubled() {
		t.Errorf("the part = %v, want 2 dice, doubled", part)
	}
	dmg := a.rollDamage(t, hit, 1, 1, 2, 3)
	if r := hitRoll(dmg); r == nil || len(r.GetFaces()) != 2 || r.GetSum() != 5 || r.GetFlat() != 0 {
		t.Errorf("the part rolled %v, want 2 dice (2 and 3) and no fixed damage", r)
	}
	for _, r := range dmg.GetPartRolls() {
		if r.GetPartKey() == extraWeaponDamage && (len(r.GetFaces()) != 2 || r.GetFlat() != 0) {
			t.Errorf("the weapon rolled %v, want 2 dice and its own fixed damage once", r)
		}
	}
}

// The damage type of the spell, not the weapon's, and the target's modifiers by type: an Arbusto Desperto is vulnerable
// to fire and takes the battleaxe's slashing as it comes.
func TestAHitSpellDamageTypeOverridesTheWeaponsAndResistanceCountsByType(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigNext, func(m *tableMod) { m.DamageType = "damage-type:fire" }))
	e := a.monsterSetup(t, true)
	a.h.roller.queue(1)
	res := a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Hidden = "monster:awakened-shrub", 1, new(false)
	})
	shrub := ""
	for _, c := range res.GetEncounter().GetCombatants() {
		if c.GetId() == res.GetCombatantIds()[0] {
			shrub = c.GetLabel()
		}
	}
	e = a.begin(t, a.get(t, a.master))
	e = a.passTo(t, e, "Pensantus")
	a.castMark(t, e, 1)
	hit := a.strike(t, e, battleaxe, shrub, d20(19))
	part := wantHitPart(t, "the hit", hit, true)
	if part.GetDamageTypeKey() != "damage-type:fire" {
		t.Fatalf("the part's type = %q, want fire", part.GetDamageTypeKey())
	}
	a.rollDamage(t, hit, 4, 5)
	var steps []*playv1.DamageStep
	for _, r := range a.log(t, a.master, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
				steps = en.GetDamage().GetSteps()
			}
		}
	}
	kinds := map[string]playv1.DamageStepKind{}
	for _, s := range steps {
		kinds[s.GetDamageTypeKey()] = s.GetKind()
	}
	if len(steps) != 1 || kinds["damage-type:fire"] != playv1.DamageStepKind_DAMAGE_STEP_KIND_VULNERABILITY || steps[0].GetBefore() != 5 || steps[0].GetAfter() != 10 {
		t.Errorf("the steps = %v, want the spell's 5 fire damage doubled (vulnerable) and the slashing weapon untouched", steps)
	}
}

// The save of the spell: a target that passes it is left as it is; one that fails it gets the conditions, which end
// with the caster's concentration.
func TestAHitSpellSaveFailGivesTheConditionsAndPassLeavesThem(t *testing.T) {
	t.Parallel()
	frightened := "condition:frightened"
	a := newHitCasters(t, nil, hitMod(trigEach, func(m *tableMod) { m.Save, m.OnFailConditions = "wis", []string{frightened} }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	conditionsOf := func(label string) []string { return byLabel(t, a.get(t, a.master), label).GetConditions() }
	has := func(label string) bool {
		for _, k := range conditionsOf(label) {
			if k == frightened {
				return true
			}
		}
		return false
	}

	hit := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	a.rollDamage(t, hit, 1, 1, 19) // the save: 19 against the caster's DC 15
	if has("Capitão Goblin") {
		t.Fatalf("a target that passed the save is %v", conditionsOf("Capitão Goblin"))
	}
	e = a.nextTurn(t, e)
	hit = a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	a.rollDamage(t, hit, 1, 1, 2)
	if !has("Capitão Goblin") {
		t.Fatalf("a target that failed the save is %v, want frightened", conditionsOf("Capitão Goblin"))
	}
	e = a.nextTurn(t, e)
	hit = a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	a.rollDamage(t, hit, 1, 1, 2) // it fails again: the conditions are replaced, not piled up
	if n := a.count(t, `SELECT count(*) FROM combatant_states s JOIN combatants c ON c.id = s.combatant_id WHERE s.source_key = $1 AND c.label = 'Capitão Goblin' AND cardinality(s.condition_keys) > 0`, tableMark); n != 1 {
		t.Errorf("%d condition rows on the target, want 1", n)
	}
	if _, err := a.ana.combat.EndConcentration(t.Context(), connect.NewRequest(&playv1.EndConcentrationRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Pensantus"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("EndConcentration() error = %v", err)
	}
	if has("Capitão Goblin") {
		t.Errorf("the condition outlived the concentration: %v", conditionsOf("Capitão Goblin"))
	}
}

// RN-10: the dice, the die, the trigger and the save of the pending effect are the caster's player's and the master's;
// no other player reads them in the combat, the log or the turn options. The canary is a d12 of 17 dice (and a 19
// cap), which nothing else of the fight shows.
func TestRN10_APendingHitEffectReachesOnlyItsCastersPlayerAndTheMaster(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, markShape, hitMod(trigMarked, func(m *tableMod) {
		m.Die, m.Dice, m.MaxDice, m.Save, m.OnFailConditions = 12, 17, 19, "wis", []string{"condition:frightened"}
	}))
	e := a.hitFight(t)
	a.castMark(t, e, 1, "Goblin")
	canaries := []*regexp.Regexp{regexp.MustCompile(`"die":\s*12`), regexp.MustCompile(`"dice[A-Za-z]*":\s*17`), regexp.MustCompile(`(?i)marked_target|next_hit|each_hit`)}
	for _, p := range []struct {
		name string
		u    *user
	}{{"Caio", a.caio}, {"Bia", a.bia}} {
		text := a.readsOf(t, p.u, e, "Toren")
		for _, re := range canaries {
			if re.MatchString(text) {
				t.Errorf("%s reads %s of the pending effect", p.name, re)
			}
		}
	}
	// The positive controls: the caster's player and the master read the card with the die.
	for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
		if text := a.readsOf(t, u, e, "Pensantus"); !canaries[0].MatchString(text) {
			t.Errorf("%s does not read the die of the effect: the canary is unreachable", name)
		}
	}
}
