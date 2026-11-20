package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
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

// drainLines are the log lines of a spell that healed its caster, as the viewer reads them.
func (a *armed) drainLines(t *testing.T, u *user, e *playv1.Encounter, caster, spell string) []*playv1.CombatLogEntry {
	t.Helper()
	var out []*playv1.CombatLogEntry
	for _, round := range a.log(t, u, e).GetRounds() {
		for _, en := range round.GetEntries() {
			if en.GetKey() == spell && en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION && en.GetActorId() == a.id(t, caster) {
				out = append(out, en)
			}
		}
	}
	return out
}

// drainHealOf is the healing damage of the one log line of the heal. Nil when the log has none.
func (a *armed) drainHealOf(t *testing.T, u *user, e *playv1.Encounter, caster, spell string) *playv1.CombatLogDamage {
	t.Helper()
	lines := a.drainLines(t, u, e, caster, spell)
	if len(lines) > 1 {
		t.Fatalf("the log has %d lines of the heal, want at most 1", len(lines))
	}
	if len(lines) == 0 || !lines[0].GetDamage().GetHealing() {
		return nil
	}
	return lines[0].GetDamage()
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

// drainImmuneOpen casts Vampiric Touch at a ghost (immune to necrotic) with the caster hurt to hp, and answers the open damage.
func (a *armed) drainImmuneOpen(t *testing.T, hp int32) (*playv1.Encounter, string) {
	t.Helper()
	e := a.rangeFight(t, "Mago", map[string][2]int32{"Capitão Goblin": {14, 8}, "Goblin 1": {14, 7}, "Goblin 2": {13, 8}})
	ghost := a.monsterNextTo(t, e, "monster:ghost", [2]int32{2, 4})
	a.correct(t, a.pens, hpIs(hp))
	return e, a.takesWith(t, e, "Mago", vampiricTouch, slotOfLevel(3), []string{ghost}).GetTargets()[0].GetPendingDamageId()
}

// drainImmune is drainImmuneOpen with the necrotic damage rolled: the ghost takes none of it.
func (a *armed) drainImmune(t *testing.T, hp int32) (*playv1.Encounter, *playv1.PendingDamage) {
	t.Helper()
	e, id := a.drainImmuneOpen(t, hp)
	a.h.roller.queue(3, 3, 3)
	return e, a.mustDamage(t, a.ana, e, id, inAppDamage).GetPendingDamage()
}

// sameLine is a log line with what differs from one fight to the next (ids, the time) cleared.
func sameLine(en *playv1.CombatLogEntry) *playv1.CombatLogEntry {
	c := proto.Clone(en).(*playv1.CombatLogEntry)
	c.Id, c.ActorId, c.TargetId, c.At = "", "", "", nil
	return c
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
		if n := len(a.drainLines(t, a.master, e, "Mago", vampiricTouch)); n != 0 {
			t.Errorf("the log keeps %d lines of the heal after the undo, want none", n)
		}
	})
}

// TestSRDVampiricTouchHealGuards: the heal needs damage that landed, and its line follows who may read it.
func TestSRDVampiricTouchHealGuards(t *testing.T) {
	t.Parallel()
	near := map[string][2]int32{"Capitão Goblin": {2, 4}, "Goblin 1": {14, 8}, "Goblin 2": {13, 8}}
	t.Run("a miss heals nothing", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Mago", near)
		a.correct(t, a.pens, hpIs(10))
		miss := func(r *playv1.CastSpellRequest) { r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: 1} }
		res := a.mustCast(t, a.ana, e, "Mago", vampiricTouch, slotOfLevel(3), a.at(t, "Capitão Goblin"), miss).GetCast()
		if res.GetTargets()[0].GetPendingDamageId() != "" {
			t.Fatalf("a natural 1 opened damage: %v", res.GetTargets()[0])
		}
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 10 {
			t.Errorf("Mago has %d hit points, want 10", got)
		}
		if n := len(a.drainLines(t, a.master, e, "Mago", vampiricTouch)); n != 0 {
			t.Errorf("the log has %d lines of a heal, want none", n)
		}
	})
	t.Run("a creature already down gives a heal of 0", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Mago", near)
		a.correct(t, a.pens, hpIs(10))
		id := a.takesWith(t, e, "Mago", vampiricTouch, slotOfLevel(3), []string{"Capitão Goblin"}).GetTargets()[0].GetPendingDamageId()
		if _, err := a.adjustHP(t, e, "Capitão Goblin", damageHP(500)); err != nil {
			t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
		}
		a.h.roller.queue(3, 3, 3)
		a.mustDamage(t, a.ana, e, id, inAppDamage)
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 10 {
			t.Errorf("Mago has %d hit points, want 10", got)
		}
		if line := a.drainHealOf(t, a.master, e, "Mago", vampiricTouch); line == nil || line.GetAmount() != 0 {
			t.Errorf("the log line of the heal = %v, want a heal of 0", line)
		}
	})
	t.Run("a character already at 0 gives a heal of 0 when the master applies", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Mago", map[string][2]int32{"Druida": {2, 4}, "Goblin 1": {14, 8}, "Goblin 2": {13, 8}, "Capitão Goblin": {12, 8}})
		a.correct(t, a.pens, hpIs(10))
		id := a.takesWith(t, e, "Mago", vampiricTouch, slotOfLevel(3), []string{"Druida"}).GetTargets()[0].GetPendingDamageId()
		a.h.roller.queue(3, 3, 3)
		a.mustDamage(t, a.ana, e, id, inAppDamage)
		a.correct(t, a.toren, hpIs(0))
		if _, err := a.settle(t, a.master, e, id, true); err != nil {
			t.Fatalf("ApplyPendingDamage() error = %v", err)
		}
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 10 {
			t.Errorf("Mago has %d hit points, want 10", got)
		}
		if line := a.drainHealOf(t, a.master, e, "Mago", vampiricTouch); line == nil || line.GetAmount() != 0 {
			t.Errorf("the log line of the heal = %v, want a heal of 0", line)
		}
	})
	t.Run("one line, with the numbers for the master and the caster's player only", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e, _ := a.drainCast(t, 10, 3, 3, 3)
		for who, want := range map[*user]bool{a.master: true, a.ana: true, a.bia: false} {
			lines := a.drainLines(t, who, e, "Mago", vampiricTouch)
			if len(lines) != 1 {
				t.Fatalf("a viewer reads %d lines of the heal, want 1", len(lines))
			}
			if got := lines[0].GetDamage() != nil; got != want {
				t.Errorf("numbers shown = %v, want %v", got, want)
			}
		}
	})
	t.Run("a heal of 0 reads as a heal of 4 to another player", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		four, _ := a.drainCast(t, 10, 3, 3, 3) // 9 necrotic: a heal of 4
		b := newSrdParty(t)
		zero, _ := b.drainImmune(t, 10)
		got, want := b.drainLines(t, b.bia, zero, "Mago", vampiricTouch), a.drainLines(t, a.bia, four, "Mago", vampiricTouch)
		if len(got) != 1 || len(want) != 1 {
			t.Fatalf("another player reads %d and %d lines of the heal, want 1 and 1", len(got), len(want))
		}
		if !proto.Equal(sameLine(got[0]), sameLine(want[0])) {
			t.Errorf("another player reads %v for a heal of 0 and %v for a heal of 4, want the same line", got[0], want[0])
		}
	})
	t.Run("a heal of 0 changes nothing of a caster at 0 and the undo takes the line back", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e, id := a.drainImmuneOpen(t, 10)
		a.correct(t, a.pens, hpIs(0))
		before := byLabel(t, a.get(t, a.master), "Mago")
		a.h.roller.queue(3, 3, 3)
		a.undoes(t, "the damage of Vampiric Touch", func() {
			a.mustDamage(t, a.ana, e, id, inAppDamage)
			if line := a.drainHealOf(t, a.master, e, "Mago", vampiricTouch); line == nil || line.GetAmount() != 0 {
				t.Fatalf("the log line of the heal = %v, want a heal of 0", line)
			}
			if got := byLabel(t, a.get(t, a.master), "Mago"); !proto.Equal(got, before) {
				t.Errorf("Mago = %v after a heal of 0, want %v", got, before)
			}
		})
		if n := len(a.drainLines(t, a.master, e, "Mago", vampiricTouch)); n != 0 {
			t.Errorf("the log keeps %d lines of the heal after the undo, want none", n)
		}
		if got := byLabel(t, a.get(t, a.master), "Mago"); !proto.Equal(got, before) {
			t.Errorf("Mago = %v after the undo, want %v", got, before)
		}
	})
	t.Run("an undone roll rolled again undoes only its own heal", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name    string
			rerolls func(t *testing.T, a *armed, e *playv1.Encounter) int32
		}{
			{"that heals again", func(*testing.T, *armed, *playv1.Encounter) int32 { return 9 }},
			{"that heals nothing", func(t *testing.T, a *armed, e *playv1.Encounter) int32 {
				t.Helper()
				if _, err := a.adjustHP(t, e, "Capitão Goblin", damageHP(500)); err != nil {
					t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
				}
				return 5
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				a := newSrdParty(t)
				e := a.rangeFight(t, "Mago", near)
				a.correct(t, a.pens, hpIs(10))
				id := a.takesWith(t, e, "Mago", vampiricTouch, slotOfLevel(3), []string{"Capitão Goblin"}).GetTargets()[0].GetPendingDamageId()
				a.h.roller.queue(3, 3, 3)
				a.mustDamage(t, a.ana, e, id, inAppDamage)
				a.undoLast(t, e)
				a.correct(t, a.pens, hpIs(5))
				want := tc.rerolls(t, a, e)
				a.h.roller.queue(3, 3, 3)
				a.mustDamage(t, a.ana, e, id, inAppDamage)
				if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != want {
					t.Fatalf("Mago has %d hit points after the second roll, want %d", got, want)
				}
				a.undoLast(t, e)
				if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 5 {
					t.Errorf("Mago has %d hit points after the second undo, want the 5 he had", got)
				}
				if n := len(a.drainLines(t, a.master, e, "Mago", vampiricTouch)); n != 0 {
					t.Errorf("the log keeps %d lines of the heal, want none", n)
				}
			})
		}
	})
	t.Run("a hidden target keeps the line from the other players", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Mago", near)
		id := a.takesWith(t, e, "Mago", vampiricTouch, slotOfLevel(3), []string{"Capitão Goblin"}).GetTargets()[0].GetPendingDamageId()
		if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Capitão Goblin"), IdempotencyKey: newKey(), Hidden: true,
		})); err != nil {
			t.Fatalf("SetCombatantHidden() error = %v", err)
		}
		a.correct(t, a.pens, hpIs(10))
		a.h.roller.queue(3, 3, 3)
		a.mustDamage(t, a.master, e, id, inAppDamage)
		if n := len(a.drainLines(t, a.master, e, "Mago", vampiricTouch)); n != 1 {
			t.Errorf("the master reads %d lines of the heal, want 1", n)
		}
		if n := len(a.drainLines(t, a.bia, e, "Mago", vampiricTouch)); n != 0 {
			t.Errorf("another player reads %d lines of the heal, want none", n)
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

// monsterNextTo adds a monster of the SRD to the combat and puts it at the square; it answers its label.
func (a *armed) monsterNextTo(t *testing.T, e *playv1.Encounter, creature string, sq [2]int32) string {
	t.Helper()
	a.h.roller.queue(1)
	res := a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Hidden = creature, 1, new(false)
	})
	id := res.GetCombatantIds()[0]
	if _, err := a.master.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: id, IdempotencyKey: newKey(), Col: sq[0], Row: sq[1],
	})); err != nil {
		t.Fatalf("MoveCombatant(%s) error = %v", creature, err)
	}
	for _, c := range res.GetEncounter().GetCombatants() {
		if c.GetId() == id {
			return c.GetLabel()
		}
	}
	t.Fatalf("%s is not in the combat", creature)
	return ""
}

// noModifierWords fails when what a player receives says that a target resists, is vulnerable or is immune.
func noModifierWords(t *testing.T, who string, payloads ...proto.Message) {
	t.Helper()
	for _, p := range payloads {
		text := strings.ToLower(asJSON(t, p))
		for _, word := range []string{"imune", "imunidade", "resist", "vulnerá", "vulnera"} {
			if strings.Contains(text, word) {
				t.Errorf("what %s receives says %q: %s", who, word, text)
			}
		}
	}
}

// TestSRDVampiricTouchHealFollowsTheModifiers: the heal is half the damage after the target's resistance,
// vulnerability or immunity, and no player reads which of them the target has.
func TestSRDVampiricTouchHealFollowsTheModifiers(t *testing.T) {
	t.Parallel()
	far := map[string][2]int32{"Capitão Goblin": {14, 8}, "Goblin 1": {14, 7}, "Goblin 2": {13, 8}}
	// 3 + 3 + 3 is 9 necrotic as rolled: a resistant target takes 4 (heal 2), a vulnerable one 18 (heal 9).
	t.Run("a resistant target", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Mago", far)
		wight := a.monsterNextTo(t, e, "monster:wight", [2]int32{2, 4})
		a.correct(t, a.pens, hpIs(10))
		id := a.takesWith(t, e, "Mago", vampiricTouch, slotOfLevel(3), []string{wight}).GetTargets()[0].GetPendingDamageId()
		a.h.roller.queue(3, 3, 3)
		dmg := a.mustDamage(t, a.ana, e, id, inAppDamage).GetPendingDamage()
		if dmg.GetAmount() != 9 {
			t.Errorf("the damage the player reads = %d, want it as rolled (9)", dmg.GetAmount())
		}
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 12 {
			t.Errorf("Mago has %d hit points, want 10 + 2 (half of 4)", got)
		}
		if line := a.drainHealOf(t, a.master, e, "Mago", vampiricTouch); line.GetAmount() != 2 {
			t.Errorf("the log line of the heal = %v, want 2 hit points", line)
		}
		noModifierWords(t, "Mago's player", a.log(t, a.ana, e), a.get(t, a.ana))
		noModifierWords(t, "another player", a.log(t, a.bia, e), a.get(t, a.bia))
	})
	t.Run("an immune target", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		e := a.rangeFight(t, "Mago", far)
		ghost := a.monsterNextTo(t, e, "monster:ghost", [2]int32{2, 4})
		a.correct(t, a.pens, hpIs(10))
		cast := a.mustCast(t, a.ana, e, "Mago", vampiricTouch, slotOfLevel(3), a.at(t, ghost), spellAttackHits)
		id := cast.GetCast().GetTargets()[0].GetPendingDamageId()
		a.h.roller.queue(3, 3, 3)
		rolled := a.mustDamage(t, a.ana, e, id, inAppDamage)
		noModifierWords(t, "Mago's player (answers)", cast, rolled)
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 10 {
			t.Errorf("Mago has %d hit points, want 10", got)
		}
		for _, who := range []*user{a.master, a.ana, a.bia} {
			if n := len(a.drainLines(t, who, e, "Mago", vampiricTouch)); n != 1 {
				t.Errorf("a viewer reads %d lines of the heal, want 1", n)
			}
		}
		if line := a.drainHealOf(t, a.master, e, "Mago", vampiricTouch); line == nil || line.GetAmount() != 0 {
			t.Errorf("the log line of the heal = %v, want a heal of 0", line)
		}
		noModifierWords(t, "Mago's player", a.log(t, a.ana, e), a.get(t, a.ana))
		noModifierWords(t, "another player", a.log(t, a.bia, e), a.get(t, a.bia))
	})
	t.Run("a vulnerable character", func(t *testing.T) {
		t.Parallel()
		a := newSrdParty(t)
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
				NamePt: "Carne podre", DescPt: []string{"Um talento da mesa."},
				Effects: []*rulesv1.TableEffect{{Type: "resistance", Kind: "vulnerable", DamageTypes: []string{"damage-type:necrotic"}}},
			}},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		cur := a.master.character(t, a.toren)
		sheet := cur.GetSheet()
		sheet.GetFull().FeatKeys = []string{res.Msg.GetEntry().GetKey()}
		if _, err := a.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
			CampaignId: a.campaignID, CharacterId: cur.GetId(), Revision: cur.GetRevision(), Name: cur.GetName(), Sheet: sheet,
		})); err != nil {
			t.Fatalf("UpdateCharacter(Druida) error = %v", err)
		}
		e := a.rangeFight(t, "Mago", map[string][2]int32{"Druida": {2, 4}, "Goblin 1": {14, 8}, "Goblin 2": {13, 8}, "Capitão Goblin": {12, 8}})
		a.correct(t, a.pens, hpIs(10))
		id := a.takesWith(t, e, "Mago", vampiricTouch, slotOfLevel(3), []string{"Druida"}).GetTargets()[0].GetPendingDamageId()
		a.h.roller.queue(3, 3, 3)
		a.mustDamage(t, a.ana, e, id, inAppDamage)
		if _, err := a.settle(t, a.master, e, id, true); err != nil {
			t.Fatalf("ApplyPendingDamage() error = %v", err)
		}
		if got := a.vitals(t, a.pens).GetHitPointsCurrent(); got != 19 {
			t.Errorf("Mago has %d hit points, want 10 + 9 (half of 18)", got)
		}
		noModifierWords(t, "Mago's player", a.log(t, a.ana, e), a.get(t, a.ana))
		noModifierWords(t, "another player", a.log(t, a.bia, e), a.get(t, a.bia))
	})
}
