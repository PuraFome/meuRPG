package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The spells the SRD aims at another creature though the database gives them range Self (Vampiric Touch,
// Produce Flame, Eyebite) and the ones whose attack comes on later turns (Flame Blade, Faithful Hound). These
// tests need the database (MEURPG_TEST_DATABASE_URL).

// spellAttackHits is the d20 of a spell attack that hits the goblins.
func spellAttackHits(r *playv1.CastSpellRequest) {
	r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: 18}
}

// rangeFight starts the combat of newSrdParty with the goblins and the Capitão at the squares given (the caster's
// side stands in a column: Mago at 1,4, Clerigo at 1,5, Druida at 1,6); first plays first.
func (a *armed) rangeFight(t *testing.T, first string, at map[string][2]int32) *playv1.Encounter {
	t.Helper()
	players := map[string]int32{"Mago": 10, "Clerigo": 10, "Druida": 10}
	players[first] = 20
	places := map[string][2]int32{"Mago": {1, 4}, "Clerigo": {1, 5}, "Druida": {1, 6}}
	for k, v := range at {
		places[k] = v
	}
	return a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId(), Count: 2}, {CharacterId: a.capitao.GetId()}},
		npcRolls: []int{1, 1, 1}, reveal: []string{"Capitão Goblin", "Goblin 1", "Goblin 2"}, at: places, players: players,
	})
}

// takesWith is takes for a spell attack, rolled with the d20 that hits.
func (a *armed) takesWith(t *testing.T, e *playv1.Encounter, caster, spell string, slot *playv1.SpellSlot, labels []string) *playv1.SpellCast {
	t.Helper()
	res := a.mustCast(t, a.userOf(caster), e, caster, spell, slot, a.at(t, labels...), spellAttackHits).GetCast()
	if len(res.GetTargets()) != len(labels) {
		t.Fatalf("%s hit %d targets, want %d", spell, len(res.GetTargets()), len(labels))
	}
	return res
}

// drainHealOf is the log line of a spell that healed its caster: the entry of the actor and the spell with the
// healing damage, as the viewer reads it. Nil when the log has none.
func (a *armed) drainHealOf(t *testing.T, u *user, e *playv1.Encounter, caster, spell string) *playv1.CombatLogDamage {
	t.Helper()
	for _, round := range a.log(t, u, e).GetRounds() {
		for _, en := range round.GetEntries() {
			if en.GetKey() == spell && en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION && en.GetActorId() == a.id(t, caster) && en.GetDamage().GetHealing() {
				return en.GetDamage()
			}
		}
	}
	return nil
}

// TestSRDVampiricTouchReach: a player's Vampiric Touch takes one creature within reach, never the caster.
func TestSRDVampiricTouchReach(t *testing.T) {
	t.Parallel()
	t.Run("refuses a target 10 ft away", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Mago", map[string][2]int32{"Goblin 1": {2, 4}, "Goblin 2": {3, 4}, "Capitão Goblin": {14, 8}})
		_, err := a.cast(t, a.ana, e, "Mago", vampiricTouch, slotOfLevel(3), a.at(t, "Goblin 2"), spellAttackHits)
		b := wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH)
		if b.GetMissingFt() != 5 {
			t.Errorf("missing %d ft, want 5", b.GetMissingFt())
		}
	})
	t.Run("attacks the creature in reach", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Mago", map[string][2]int32{"Goblin 1": {2, 4}, "Goblin 2": {3, 4}, "Capitão Goblin": {14, 8}})
		res := a.takesWith(t, e, "Mago", vampiricTouch, slotOfLevel(3), []string{"Goblin 1"})
		tg := res.GetTargets()[0]
		if tg.GetCombatantId() != a.id(t, "Goblin 1") || tg.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT && tg.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS {
			t.Errorf("target = %v, want a spell attack against Goblin 1", tg)
		}
		if tg.GetAttackRoll() == nil {
			t.Error("the cast rolled no attack")
		}
	})
	t.Run("refuses the caster", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Mago", map[string][2]int32{"Goblin 1": {2, 4}, "Goblin 2": {3, 4}, "Capitão Goblin": {14, 8}})
		_, err := a.cast(t, a.ana, e, "Mago", vampiricTouch, slotOfLevel(3), a.at(t, "Mago"), spellAttackHits)
		wantCode(t, "Vampiric Touch at the caster", err, connect.CodeInvalidArgument)
		_, err = a.cast(t, a.ana, e, "Mago", vampiricTouch, slotOfLevel(3), nil, spellAttackHits)
		wantCode(t, "Vampiric Touch with no target", err, connect.CodeInvalidArgument)
	})
}

// drainCast casts Vampiric Touch at the Capitão in reach with the caster hurt to hp, and rolls the necrotic faces.
func (a *armed) drainCast(t *testing.T, hp int32, faces ...int) (*playv1.Encounter, *playv1.PendingDamage) {
	t.Helper()
	e := a.rangeFight(t, "Mago", map[string][2]int32{"Capitão Goblin": {2, 4}, "Goblin 1": {14, 8}, "Goblin 2": {13, 8}})
	a.correct(t, a.pens, hpIs(hp))
	res := a.takesWith(t, e, "Mago", vampiricTouch, slotOfLevel(3), []string{"Capitão Goblin"})
	id := res.GetTargets()[0].GetPendingDamageId()
	if id == "" {
		t.Fatalf("the attack opened no damage: %v", res.GetTargets()[0])
	}
	a.h.roller.queue(faces...)
	return e, a.mustDamage(t, a.ana, e, id, inAppDamage).GetPendingDamage()
}

// TestSRDVampiricTouchHeals: the caster regains half the necrotic damage dealt (rounded down), up to the
// maximum, the log says so and the master's undo takes it back.
func TestSRDVampiricTouchHeals(t *testing.T) {
	t.Parallel()
	t.Run("half the damage, odd rounds down", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		// A 3rd-level slot casts 3d6 of necrotic damage: 3 + 3 + 3 is 9, and half of it is 4.
		e, dmg := a.drainCast(t, 10, 3, 3, 3)
		if dmg.GetAmount() != 9 || dmg.GetDamageTypeKey() != "damage-type:necrotic" {
			t.Fatalf("damage = %v, want 9 necrotic", dmg)
		}
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 14 {
			t.Errorf("Mago has %d hit points, want 10 + 4", got)
		}
		for _, who := range []*user{a.master, a.ana} {
			if line := a.drainHealOf(t, who, e, "Mago", vampiricTouch); line.GetAmount() != 4 {
				t.Errorf("the log line of the heal = %v, want 4 hit points", line)
			}
		}
	})
	t.Run("up to the maximum", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		full := a.vitals(t, a.pens).GetHitPointsMax()
		e, _ := a.drainCast(t, full-1, 6, 6, 6)
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != full {
			t.Errorf("Mago has %d hit points, want the maximum %d", got, full)
		}
		if line := a.drainHealOf(t, a.master, e, "Mago", vampiricTouch); line.GetAmount() != 1 {
			t.Errorf("the log line of the heal = %v, want the 1 hit point regained", line)
		}
	})
	t.Run("the undo takes the heal and its line back", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Mago", map[string][2]int32{"Capitão Goblin": {2, 4}, "Goblin 1": {14, 8}, "Goblin 2": {13, 8}})
		a.correct(t, a.pens, hpIs(10))
		res := a.takesWith(t, e, "Mago", vampiricTouch, slotOfLevel(3), []string{"Capitão Goblin"})
		id := res.GetTargets()[0].GetPendingDamageId()
		a.h.roller.queue(3, 3, 3)
		a.undoes(t, "the damage of Vampiric Touch", func() {
			a.mustDamage(t, a.ana, e, id, inAppDamage)
			if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 14 {
				t.Fatalf("Mago has %d hit points after the damage, want 10 + 4", got)
			}
		})
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 10 {
			t.Errorf("Mago has %d hit points after the undo, want 10", got)
		}
		if line := a.drainHealOf(t, a.master, e, "Mago", vampiricTouch); line != nil {
			t.Errorf("the log keeps the heal after the undo: %v", line)
		}
	})
	t.Run("a hit on a character heals when the master applies it", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Mago", map[string][2]int32{"Druida": {2, 4}, "Goblin 1": {14, 8}, "Goblin 2": {13, 8}, "Capitão Goblin": {12, 8}})
		a.correct(t, a.pens, hpIs(10))
		res := a.takesWith(t, e, "Mago", vampiricTouch, slotOfLevel(3), []string{"Druida"})
		id := res.GetTargets()[0].GetPendingDamageId()
		a.h.roller.queue(3, 3, 3)
		a.mustDamage(t, a.ana, e, id, inAppDamage)
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 10 {
			t.Fatalf("Mago has %d hit points before the master applies the damage, want 10", got)
		}
		if _, err := a.settle(t, a.master, e, id, true); err != nil {
			t.Fatalf("ApplyPendingDamage() error = %v", err)
		}
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 14 {
			t.Errorf("Mago has %d hit points, want 10 + 4", got)
		}
		if line := a.drainHealOf(t, a.master, e, "Mago", vampiricTouch); line.GetAmount() != 4 {
			t.Errorf("the log line of the heal = %v, want 4 hit points", line)
		}
		a.undoLast(t, e)
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 10 {
			t.Errorf("Mago has %d hit points after the undo, want 10", got)
		}
	})
}

// TestSRDProduceFlameReach: the hurl reaches 30 ft as a ranged spell attack.
func TestSRDProduceFlameReach(t *testing.T) {
	t.Parallel()
	t.Run("refuses a target at 35 ft and takes one at 30 ft", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Druida", map[string][2]int32{"Goblin 1": {7, 6}, "Goblin 2": {8, 6}, "Capitão Goblin": {14, 8}})
		_, err := a.attack(t, a.caio, e, "Druida", produceFlame, "Goblin 2", d20(18))
		if b := wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH); b.GetMissingFt() != 5 {
			t.Errorf("missing %d ft, want 5", b.GetMissingFt())
		}
		res := a.mustAttack(t, a.caio, e, "Druida", produceFlame, "Goblin 1", d20(18))
		if got := res.GetRoll().GetMode(); got != playv1.RollMode_ROLL_MODE_NORMAL || res.GetPendingDamage() == nil {
			t.Errorf("attack = %v, want a ranged spell attack at 30 ft that hit, with no circumstance", res)
		}
	})
	t.Run("a hostile creature within 5 ft gives disadvantage", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Druida", map[string][2]int32{"Goblin 1": {7, 6}, "Goblin 2": {2, 6}, "Capitão Goblin": {14, 8}})
		a.h.roller.queue(18, 17) // two d20: the lower counts
		res := a.mustAttack(t, a.caio, e, "Druida", produceFlame, "Goblin 1", inAppRoll)
		if got := res.GetRoll().GetMode(); got != playv1.RollMode_ROLL_MODE_DISADVANTAGE {
			t.Errorf("attack mode = %v, want disadvantage with a hostile creature at 5 ft", got)
		}
	})
}

// TestSRDEyebite: one creature within 60 ft.
func TestSRDEyebite(t *testing.T) {
	t.Parallel()
	a := newSrdParty(t)
	e := a.rangeFight(t, "Mago", map[string][2]int32{"Goblin 1": {13, 4}, "Goblin 2": {14, 4}, "Capitão Goblin": {14, 8}})
	a.wantTooMany(t, e, "Mago", eyebite, slotOfLevel(6), []string{"Goblin 1", "Capitão Goblin"})
	_, err := a.cast(t, a.ana, e, "Mago", eyebite, slotOfLevel(6), a.at(t, "Goblin 2"), noCastRoll)
	b := wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH)
	if b.GetMissingFt() != 5 {
		t.Errorf("missing %d ft, want 5", b.GetMissingFt())
	}
	a.takes(t, e, "Mago", eyebite, slotOfLevel(6), []string{"Goblin 1"})
}

// TestSRDSpellsWithNoAttackAtTheCast: Faithful Hound and Flame Blade name no creature, roll no attack and open no
// damage when they are cast.
func TestSRDSpellsWithNoAttackAtTheCast(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, caster, spell string
		slot                *playv1.SpellSlot
	}{
		{"Faithful Hound", "Mago", faithfulHound, slotOfLevel(4)},
		{"Flame Blade", "Druida", flameBlade, slotOfLevel(2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newSrdParty(t)
			e := a.rangeFight(t, tc.caster, map[string][2]int32{"Goblin 1": {2, 6}, "Goblin 2": {2, 4}, "Capitão Goblin": {14, 8}})
			res := a.mustCast(t, a.userOf(tc.caster), e, tc.caster, tc.spell, tc.slot, nil, noCastRoll).GetCast()
			for _, tg := range res.GetTargets() {
				if tg.GetAttackRoll() != nil || tg.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_UNSPECIFIED || tg.GetPendingDamageId() != "" || tg.GetSave() != nil {
					t.Errorf("%s target = %v, want no attack, no save and no damage", tc.name, tg)
				}
			}
			if len(res.GetPendingDamages()) != 0 {
				t.Errorf("%s opened damage %v at the cast", tc.name, res.GetPendingDamages())
			}
			for _, c := range []*charactersv1.Character{a.pens, a.toren} {
				if v := a.vitals(t, c); v.GetHitPointsCurrent() != v.GetHitPointsMax() {
					t.Errorf("%s hurt %s: %d of %d", tc.name, c.GetName(), v.GetHitPointsCurrent(), v.GetHitPointsMax())
				}
			}
		})
	}
}
