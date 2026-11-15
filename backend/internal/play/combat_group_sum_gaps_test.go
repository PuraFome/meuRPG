package play

import (
	"encoding/json"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The paths of the take-out of a group part that a rogue against a shrub does not reach (#835): the roll that keeps what the other
// damage types rolled, a damage that is only rolled (a player's character), what depends on the damage (a smite's slot, a rider, a
// concentration save) and the refusals.

// ---- the roll of a group's sum, with no database ----

func groupSumParts() []partRecord {
	return []partRecord{
		{Key: partWeapon, Kind: partWeapon, Count: 1, Sides: 8, DamageType: groupPiercing, Available: true},
		{Key: "sneak-attack", Kind: partExtra, Choosable: true, Selected: true, Removed: true, Count: 1, Sides: 6, DamageType: groupPiercing, Available: true},
		{Key: "extra-fire", Kind: partExtra, Choosable: true, Selected: true, Count: 1, Sides: 4, DamageType: groupFire, Available: true},
		{Key: "extra-radiant", Kind: partExtra, Choosable: true, Selected: true, Count: 1, Sides: 4, DamageType: groupRadiant, Available: true},
	}
}

// What the first roll left for the damage types other than the awaited one.
func groupSumKept() []partRoll {
	return []partRoll{
		{Key: "extra-fire", DamageType: groupFire, Count: 1, Sides: 4, Faces: []int32{3}, Sum: 3, Counted: true},
		{Key: "extra-radiant", DamageType: groupRadiant, Count: 1, Sides: 4, Sum: 2, Counted: true, Physical: true},
	}
}

func groupSumRoll(in rollInput, typed typedSums, kept []partRoll) (partsRoll, error) {
	s := &Service{roller: &scriptedRoller{faces: []int{6}}}
	return s.rollParts(rollPartsInput{
		p: playdb.PendingDamage{}, parts: groupSumParts(), rule: combat.CriticalDoubledDice, in: in, typed: typed,
		awaited: groupPiercing, kept: kept,
	})
}

// The roll of a damage that waits for a group's sum makes the dice of the awaited type alone: the other types are what the first
// roll left (the same faces and sums, counted as they were), the part taken out is a row that does not count, the parts are the ones
// the first roll left (no mark is read), and the damage adds up from both.
func TestTheRollOfAGroupSumKeepsWhatTheOtherTypesRolledAndRollsTheAwaitedTypeAlone(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		in    rollInput
		typed typedSums
		sum   int32 // the weapon's sum
	}{
		{"typed", rollInput{typed: 5}, typedSums{byType: map[string]int{groupPiercing: 5}}, 5},
		{"in the app", rollInput{inApp: true}, typedSums{}, 6},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := groupSumRoll(tt.in, tt.typed, groupSumKept())
			if err != nil {
				t.Fatalf("rollParts() error = %v", err)
			}
			if len(got.rolls) != 4 || len(got.parts) != 4 {
				t.Fatalf("rolls %d, parts %d, want 4 and 4: one row for each part", len(got.rolls), len(got.parts))
			}
			weapon, sneak, fire, radiant := got.rolls[0], got.rolls[1], got.rolls[2], got.rolls[3]
			if !weapon.Counted || weapon.Sum != tt.sum {
				t.Errorf("the weapon = %+v, want it counted, summing %d", weapon, tt.sum)
			}
			if sneak.Key != "sneak-attack" || sneak.Counted || sneak.Sum != 0 || sneak.DamageType != groupPiercing {
				t.Errorf("the part taken out = %+v, want a row that does not count", sneak)
			}
			if fire.Sum != 3 || len(fire.Faces) != 1 || fire.Faces[0] != 3 || !fire.Counted {
				t.Errorf("the fire = %+v, want what the first roll left (3 on one die)", fire)
			}
			if radiant.Sum != 2 || !radiant.Counted || !radiant.Physical {
				t.Errorf("the radiant = %+v, want what the first roll left (2, typed)", radiant)
			}
			if want := int(tt.sum) + 3 + 2; got.total != want || got.byType[groupPiercing] != int(tt.sum) || got.byType[groupFire] != 3 || got.byType[groupRadiant] != 2 {
				t.Errorf("total %d by type %v, want %d with the awaited type's sum and the kept ones", got.total, got.byType, want)
			}
		})
	}
}

// The roll refuses what the first roll did not leave: a counted part of another type that has no kept row is an error, not a
// part rolled again; a sum out of the range of the awaited type's dice is invalid_argument.
func TestTheRollOfAGroupSumRefusesAMissingKeptRollAndASumOutOfRange(t *testing.T) {
	t.Parallel()
	if _, err := groupSumRoll(rollInput{typed: 5}, typedSums{byType: map[string]int{groupPiercing: 5}}, groupSumKept()[:1]); err == nil {
		t.Error("rollParts(a counted part of another type with no kept roll) error = nil, want one")
	}
	for _, sum := range []int{0, 9} {
		_, err := groupSumRoll(rollInput{typed: sum}, typedSums{byType: map[string]int{groupPiercing: sum}}, groupSumKept())
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("rollParts(a sum of %d for 1d8) error = %v, want invalid_argument", sum, err)
		}
	}
}

// keptRolls keeps the rolls of the other types and drops the awaited one; the typed sums of the request may name the awaited type or
// its parts and nothing else.
func TestAGroupSumKeepsTheOtherTypesAndRefusesTypedPartsOfThem(t *testing.T) {
	t.Parallel()
	kept := keptRolls([]partRoll{
		{Key: partWeapon, DamageType: groupPiercing}, {Key: "extra-fire", DamageType: groupFire}, {Key: "extra-radiant", DamageType: groupRadiant},
	}, groupPiercing)
	if len(kept) != 2 || kept[0].Key != "extra-fire" || kept[1].Key != "extra-radiant" {
		t.Errorf("keptRolls = %v, want the fire and the radiant rows", kept)
	}
	parts := groupSumParts()
	for _, tt := range []struct {
		name  string
		typed typedSums
		ok    bool
	}{
		{"the awaited type", typedSums{byType: map[string]int{groupPiercing: 4}}, true},
		{"a part of the awaited type", typedSums{byPart: map[string]int{partWeapon: 4}}, true},
		{"another type", typedSums{byType: map[string]int{groupFire: 2}}, false},
		{"a part of another type", typedSums{byPart: map[string]int{"extra-fire": 2}}, false},
	} {
		_, err := groupSumTyped(parts, groupPiercing, rollInput{}, tt.typed)
		if (err == nil) != tt.ok || (!tt.ok && connect.CodeOf(err) != connect.CodeInvalidArgument) {
			t.Errorf("groupSumTyped(%s) error = %v, want ok %v", tt.name, err, tt.ok)
		}
	}
	got, err := groupSumTyped(parts, groupPiercing, rollInput{typed: 5}, typedSums{})
	if err != nil || got.byType[groupPiercing] != 5 {
		t.Errorf("groupSumTyped(typed_sum 5) = %v, %v, want the awaited type's sum 5", got.byType, err)
	}
}

// ---- a smite and a player's character as the target ----

// paladinRogue is a paladin 2 / rogue 2: a Divine Smite (radiant) beside Sneak Attack and a rapier (piercing).
func (u *user) paladinRogue(t *testing.T, campaignID, name string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: abilities(16, 16, 14, 12, 14), RaceKey: "race:human",
		Classes:    []*charactersv1.ClassLevel{{ClassKey: "class:paladin", Level: 2}, {ClassKey: "class:rogue", Level: 2}},
		WeaponKeys: []string{rapier},
	}}}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// sumSmiteHit is Toren's hit on a restrained target, rolled with one piercing sum for the rapier and Sneak Attack and, when smite is
// set, a Divine Smite of the first level.
type sumSmiteHit struct {
	*groupHit
}

func newSumSmiteHit(t *testing.T, target string, smite bool) *sumSmiteHit {
	t.Helper()
	a, _, shrub := partsFight(t, func(a *armed) { a.toren = a.caio.paladinRogue(t, a.campaignID, "Toren") }, "monster:awakened-shrub")
	if target == "" { // the awakened shrub: an NPC, so the damage lands at once
		target = shrub
	}
	e := a.passTo(t, a.get(t, a.master), "Toren")
	a.setConditions(t, e, target, "condition:restrained")
	hit := a.mustAttack(t, a.caio, e, "Toren", rapier, target, func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
	g := &groupHit{a: a, e: e, shrub: target, id: hit.GetPendingDamage().GetId()}
	marks := []*playv1.SelectedExtra{{Key: "sneak-attack"}}
	typed := []*playv1.TypedPart{{DamageType: groupPiercing, Sum: 9}}
	if smite {
		marks = append(marks, &playv1.SelectedExtra{Key: "divine-smite", SlotLevel: 1})
		typed = append(typed, &playv1.TypedPart{PartKey: "divine-smite", Sum: 5}, &playv1.TypedPart{PartKey: "divine-smite-extra", Sum: 1})
	}
	g.rolled = a.mustDamage(t, a.caio, e, g.id, extrasMarked(marks, typed...)).GetPendingDamage()
	return &sumSmiteHit{groupHit: g}
}

func (h *sumSmiteHit) slotsUsed(t *testing.T) int32 {
	t.Helper()
	return usedSlots(h.a.vitals(t, h.a.toren), 1)
}

func (h *sumSmiteHit) slotsTotal(t *testing.T) int32 {
	t.Helper()
	for _, s := range h.a.vitals(t, h.a.toren).GetSpellSlots() {
		if s.GetLevel() == 1 {
			return s.GetTotal()
		}
	}
	return 0
}

// A damage that is only rolled (a player's character waits for the master) has no hit points to put back. The smite's slot was
// spent once, when the smite was declared (the first roll): the take-out of another part keeps it spent, and the roll that follows
// re-rolls dice and spends nothing.
func TestTakingAPartOutOfARolledDamageKeepsTheSmiteSlotSpentAndTheNextRollSpendsNothing(t *testing.T) {
	t.Parallel()
	h := newSumSmiteHit(t, "Brisa", true)
	if status, _ := h.a.pendingOf(t, h.id); status != pendingRolled {
		t.Fatalf("status = %q after the roll on a player's character, want rolled", status)
	}
	if got := h.slotsUsed(t); got != 1 {
		t.Fatalf("slots used = %d after the smite, want 1", got)
	}
	hp, _, _ := h.a.hp(t, "Brisa")
	res, err := h.takeOut(t, "sneak-attack", "sem aliado")
	if err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	if p := res.GetPendingDamage(); p.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_GROUP_SUM || p.GetAwaitedDamageTypeKey() != groupPiercing {
		t.Fatalf("pending damage = %v awaiting %q, want AWAITING_GROUP_SUM awaiting piercing", p.GetStatus(), p.GetAwaitedDamageTypeKey())
	}
	if got := h.slotsUsed(t); got != 1 {
		t.Errorf("slots used = %d after the take-out, want 1: the kept smite keeps its slot spent", got)
	}
	if now, _, _ := h.a.hp(t, "Brisa"); now != hp {
		t.Errorf("Brisa's hit points = %d, want %d: a damage only rolled changed none", now, hp)
	}
	again, err := h.a.damage(t, h.a.caio, h.e, h.id, typedDamage(4))
	if err != nil {
		t.Fatalf("RollDamage() error = %v", err)
	}
	p := again.GetPendingDamage()
	if p.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED || p.GetAwaitedDamageTypeKey() != "" {
		t.Errorf("pending damage = %v awaiting %q, want ROLLED and nothing awaited", p.GetStatus(), p.GetAwaitedDamageTypeKey())
	}
	smite := partRollOf(p.GetPartRolls(), "divine-smite")
	if smite == nil || smite.GetSum() != 5 || !smite.GetCounted() {
		t.Errorf("the smite = %v after the second roll, want the 5 the first roll left, counted", smite)
	}
	if got := h.slotsUsed(t); got != 1 {
		t.Errorf("slots used = %d after the second roll, want 1: the second roll spends nothing", got)
	}
	if want := h.rolled.GetAmount() - 9 + 4; p.GetAmount() != want {
		t.Errorf("amount = %d, want %d: the first roll's with the group's 9 replaced by the new sum 4", p.GetAmount(), want)
	}
}

// With no slot left after the take-out (all spent elsewhere), the second roll goes through all the same and spends nothing: the
// smite's slot was paid for by the first roll.
func TestTheSecondRollGoesThroughWhenNoSlotIsLeftAndSpendsNothing(t *testing.T) {
	t.Parallel()
	h := newSumSmiteHit(t, "Brisa", true)
	if _, err := h.takeOut(t, "sneak-attack", "x"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	var all []*playv1.SpellSlotsUsed
	for _, s := range h.a.vitals(t, h.a.toren).GetSpellSlots() {
		all = append(all, &playv1.SpellSlotsUsed{Level: s.GetLevel(), Used: s.GetTotal()})
	}
	h.a.correct(t, h.a.toren, func(r *playv1.AdjustCharacterVitalsRequest) { r.SpellSlotsUsed = all })
	res, err := h.a.damage(t, h.a.caio, h.e, h.id, typedDamage(4))
	if err != nil {
		t.Fatalf("RollDamage(no slot left) error = %v", err)
	}
	if got := res.GetPendingDamage().GetStatus(); got != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED {
		t.Errorf("status = %v, want ROLLED", got)
	}
	if h.slotsUsed(t) != h.slotsTotal(t) {
		t.Errorf("slots used = %d of %d, want every slot still spent: the roll spent nothing and gave nothing back", h.slotsUsed(t), h.slotsTotal(t))
	}
}

// The master's other way out of a damage whose second roll cannot be made: the discard.
func TestADamageWaitingForASumThatCannotBeRolledCanBeDiscardedByTheMaster(t *testing.T) {
	t.Parallel()
	h := newSumSmiteHit(t, "Brisa", true)
	if _, err := h.takeOut(t, "sneak-attack", "x"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	got, err := h.a.settle(t, h.a.master, h.e, h.id, false)
	if err != nil {
		t.Fatalf("DiscardPendingDamage() error = %v", err)
	}
	if got.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_DISCARDED {
		t.Errorf("status = %v, want DISCARDED", got.GetStatus())
	}
}

// A divine smite and its extra die never come out of a damage, in a group or not: the refusal is the part's, nothing is undone.
func TestADivineSmiteIsNotTakenOutOfADamageThatHasOne(t *testing.T) {
	t.Parallel()
	h := newSumSmiteHit(t, "Brisa", true)
	for _, key := range []string{"divine-smite", "divine-smite-extra"} {
		_, err := h.takeOut(t, key, "x")
		wantBlockedBy(t, "RemoveDamagePart("+key+")", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_PART_NOT_REMOVABLE)
	}
	if status, _ := h.a.pendingOf(t, h.id); status != pendingRolled || h.slotsUsed(t) != 1 {
		t.Errorf("status %q, slots used %d after the refusals, want it rolled and the slot spent", status, h.slotsUsed(t))
	}
}

// The weapon of a group stays too, and so does a part of a cast's damage: the part's rule comes before anything is undone.
func TestTheWeaponAndAPartOfACastDamageAreNotTakenOutOfAGroup(t *testing.T) {
	t.Parallel()
	h := newSumSmiteHit(t, "Brisa", false)
	_, err := h.takeOut(t, partWeapon, "x")
	wantBlockedBy(t, "RemoveDamagePart(the weapon of a group)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_PART_NOT_REMOVABLE)
	if _, err := h.a.h.pool.Exec(t.Context(), `UPDATE pending_damages SET cast_id = gen_random_uuid() WHERE id = $1`, h.id); err != nil {
		t.Fatalf("make the damage a cast's: %v", err)
	}
	_, err = h.takeOut(t, "sneak-attack", "x")
	wantBlockedBy(t, "RemoveDamagePart(a cast's damage)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_PART_NOT_REMOVABLE)
	if status, _ := h.a.pendingOf(t, h.id); status != pendingRolled {
		t.Errorf("status = %q, want it untouched", status)
	}
}

// A rider the damage opened and the player answered cannot be taken back: the take-out is NOTHING_TO_UNDO and writes nothing; an
// offer still open goes with the roll.
func TestTakingAPartOutOfARolledDamageIsRefusedWhenAHitRiderWasUsed(t *testing.T) {
	t.Parallel()
	for _, used := range []bool{true, false} {
		h := newSumSmiteHit(t, "Brisa", true)
		if _, err := h.a.h.pool.Exec(t.Context(), `
			INSERT INTO hit_riders (encounter_id, attacker_id, target_id, kind, round, created_at, used, pending_id)
			VALUES ($1, $2, $3, 'stunning_strike', 1, now(), $4, $5)`,
			h.e.GetId(), h.a.id(t, "Toren"), h.a.id(t, "Brisa"), used, h.id); err != nil {
			t.Fatalf("insert the rider: %v", err)
		}
		_, err := h.takeOut(t, "sneak-attack", "x")
		var left int
		if err := h.a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM hit_riders WHERE pending_id = $1`, h.id).Scan(&left); err != nil {
			t.Fatalf("count the riders: %v", err)
		}
		if used {
			wantBlockedBy(t, "RemoveDamagePart(a rider was used)", err, blockedNothingToUndo)
			if status, _ := h.a.pendingOf(t, h.id); status != pendingRolled || left != 1 || h.slotsUsed(t) != 1 {
				t.Errorf("used: status %q, riders %d, slots used %d, want it untouched", status, left, h.slotsUsed(t))
			}
			continue
		}
		if err != nil || left != 0 || h.slotsUsed(t) != 1 {
			t.Errorf("an open offer: error = %v, riders left %d, slots used %d, want the take-out, the offer gone and the slot still spent", err, left, h.slotsUsed(t))
		}
	}
}

// The master gave a player's character the damage already: the correction is of the hit points, the take-out is DAMAGE_RESOLVED.
func TestTakingAPartOutOfADamageAppliedToACharacterIsResolved(t *testing.T) {
	t.Parallel()
	h := newSumSmiteHit(t, "Brisa", false)
	if _, err := h.a.settle(t, h.a.master, h.e, h.id, true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	hp, _, _ := h.a.hp(t, "Brisa")
	_, err := h.takeOut(t, "sneak-attack", "x")
	wantBlockedBy(t, "RemoveDamagePart(an applied damage on a character)", err, blockedDamageResolved)
	if status, _ := h.a.pendingOf(t, h.id); status != pendingApplied {
		t.Errorf("status = %q, want applied", status)
	}
	if now, _, _ := h.a.hp(t, "Brisa"); now != hp {
		t.Errorf("hit points = %d, want %d", now, hp)
	}
}

// ---- a concentration save ----

// The target concentrates: the damage opens a save. While it waits, the take-out closes it with the roll; once the master rolled it,
// the save is a later event than the damage and the take-out is NOTHING_TO_UNDO (the rule of the undo), with nothing written.
func TestTakingAPartOutWhileTheTargetsConcentrationSaveWaitsClosesItAndAfterItIsRolledIsRefused(t *testing.T) {
	t.Parallel()
	concentrating := func(g *groupHit) { g.a.concentrate(t, g.shrub, webSpell) }
	t.Run("the save waits", func(t *testing.T) {
		t.Parallel()
		g := newGroupHitWith(t, 12, concentrating)
		if g.a.windowOf(t, g.a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE) == nil {
			t.Fatal("the concentration save did not open")
		}
		if _, err := g.takeOut(t, "sneak-attack", "x"); err != nil {
			t.Fatalf("RemoveDamagePart(the save waits) error = %v", err)
		}
		if w := g.a.windowOf(t, g.a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE); w != nil {
			t.Errorf("the save is still open after the take-out: %v", w)
		}
		if _, err := g.a.damage(t, g.a.caio, g.e, g.id, typedDamage(4)); err != nil {
			t.Fatalf("RollDamage() error = %v", err)
		}
		if g.a.windowOf(t, g.a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE) == nil {
			t.Error("the second roll opened no concentration save")
		}
	})
	t.Run("the save was rolled", func(t *testing.T) {
		t.Parallel()
		g := newGroupHitWith(t, 12, concentrating)
		w := g.a.windowOf(t, g.a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
		if w == nil {
			t.Fatal("the concentration save did not open")
		}
		if _, err := g.a.resolveConcentration(t, g.a.master, g.e, w.GetId(), keepConcentration); err != nil {
			t.Fatalf("ResolveConcentrationSave() error = %v", err)
		}
		hurt := g.hp(t)
		_, err := g.takeOut(t, "sneak-attack", "x")
		wantBlockedBy(t, "RemoveDamagePart(after the save was rolled)", err, blockedNothingToUndo)
		if status, _ := g.a.pendingOf(t, g.id); status != pendingApplied || g.hp(t) != hurt {
			t.Errorf("status %q, hit points %d (were %d), want nothing written", status, g.hp(t), hurt)
		}
	})
}

// ---- a held roll ----

// The target is a rogue who can answer with Uncanny Dodge: the damage waits for the window, and the rogue passes. The take-out takes the
// landed roll back, the second roll waits for the window again, the rogue uses the reaction, and the damage lands halved as a replay
// that keeps what the other damage types rolled.
func TestTheSecondRollOfAGroupSumIsHeldForAReactionAndLandsAsAReplay(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.paladinRogue(t, a.campaignID, "Toren")
		a.pens = rogueFive(t, a)
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Toren": 18, "Reator": 10, "Brisa": 1},
		reveal:  []string{"Goblin"},
		at:      map[string][2]int32{"Toren": {3, 3}, "Goblin": {8, 8}, "Reator": {4, 3}, "Brisa": {9, 9}},
	})
	a.setConditions(t, e, "Reator", "condition:restrained")
	hit := a.mustAttack(t, a.caio, e, "Toren", rapier, "Reator", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
	id := hit.GetPendingDamage().GetId()
	marks := []*playv1.SelectedExtra{{Key: "sneak-attack"}, {Key: "divine-smite", SlotLevel: 1}}
	typed := []*playv1.TypedPart{{DamageType: groupPiercing, Sum: 10}, {PartKey: "divine-smite", Sum: 6}, {PartKey: "divine-smite-extra", Sum: 1}}
	held := a.mustDamage(t, a.caio, e, id, extrasMarked(marks, typed...)).GetPendingDamage()
	if held.GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED {
		t.Fatalf("the damage landed before the rogue answered: %v", held)
	}
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE)
	if w == nil {
		t.Fatal("the rogue has no Uncanny Dodge window")
	}
	a.mustAnswer(t, a.ana, e, w.GetId(), passAnswer)
	status, first := a.pendingOf(t, id)
	if status != pendingRolled {
		t.Fatalf("status = %q after the answer, want rolled", status)
	}

	g := &groupHit{a: a, e: e, shrub: "Reator", id: id}
	res, err := g.takeOut(t, "sneak-attack", "x")
	if err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	if got := res.GetPendingDamage().GetStatus(); got != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_GROUP_SUM {
		t.Fatalf("status = %v after the take-out, want AWAITING_GROUP_SUM", got)
	}
	again, err := a.damage(t, a.caio, e, id, typedDamage(4))
	if err != nil {
		t.Fatalf("RollDamage(the second roll) error = %v", err)
	}
	if got := again.GetPendingDamage().GetStatus(); got == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED {
		t.Fatalf("the second roll landed before the rogue answered: %v", again.GetPendingDamage())
	}
	if status, _ := a.pendingOf(t, id); status != pendingAwaitingGroupSum {
		t.Errorf("status = %q while the second roll is held, want it still waiting for the sum", status)
	}
	w = a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE)
	if w == nil {
		t.Fatal("the second roll opened no Uncanny Dodge window")
	}
	a.mustAnswer(t, a.ana, e, w.GetId(), useAnswer(0))
	status, amount := a.pendingOf(t, id)
	if status != pendingRolled {
		t.Fatalf("status = %q after the second answer, want rolled", status)
	}
	// The new sum 4, the weapon's 3 and the smite's 6 (the extra die counts for an undead alone), halved by the answer.
	if want := (4 + 3 + 6) / 2; amount != int32(want) || amount >= first {
		t.Errorf("amount = %d, want %d (the first roll's was %d)", amount, want, first)
	}
	if reator := byLabel(t, a.get(t, a.master), "Reator"); !reator.GetReactionUsed() {
		t.Error("the rogue's reaction is not used after it answered")
	}
	if got := usedSlots(a.vitals(t, a.toren), 1); got != 1 {
		t.Errorf("slots used = %d after the replay, want 1: the smite's slot was paid by the first roll, and the replay spends nothing", got)
	}
	p := a.pendingView(t, id)
	if p.GetAwaitedDamageTypeKey() != "" {
		t.Errorf("awaited = %q after the replay, want none", p.GetAwaitedDamageTypeKey())
	}
	if smite := partRollOf(p.GetPartRolls(), "divine-smite"); smite == nil || !smite.GetCounted() {
		t.Errorf("the smite = %v after the replay, want it kept and counted", smite)
	}
	if sneak := damagePartOf(p, "sneak-attack"); sneak == nil || sneak.GetSelected() {
		t.Errorf("Sneak Attack = %v after the replay, want it still out", sneak)
	}
}

// pendingView is the damage as the master reads it in the turn options of its attacker.
func (a *armed) pendingView(t *testing.T, id string) *playv1.PendingDamage {
	t.Helper()
	opts, err := a.options(t, a.master, a.get(t, a.master), "Toren")
	if err != nil {
		t.Fatalf("GetTurnOptions(Toren) error = %v", err)
	}
	for _, p := range opts.GetPendingDamages() {
		if p.GetId() == id {
			return p
		}
	}
	t.Fatalf("pending damage %s is not in the turn options", id)
	return nil
}

// ---- a reroll rule ----

// A group's sum is typed, and the typed dice have no faces: nothing of the first roll was rolled again, so nothing is given back
// that the second roll must spend. The second roll, made in the app, is a roll like any other: it rolls the low die again and spends
// one use; with no use left (spent elsewhere between the two rolls) it still goes through, with the die as it came up.
func TestTheSecondRollOfAGroupSumRerollsAsAnyRollDoesAndNeverFailsForWantOfAUse(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		spent     bool
		faces     []int
		wantFaces []int32
		wantLeft  int32
	}{
		{"a use left", false, []int{2, 5}, []int32{5}, 1},
		{"the uses spent elsewhere", true, []int{2}, []int32{2}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var feat string
			a, _, shrub := partsFight(t, func(a *armed) {
				a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:rogue", "race:human", 3, abilities(16, 16, 14, 12, 10), []string{rapier}, nil)
				feat = a.rerollFeat(t, rerollEffects()...)
				a.giveFeat(t, a.toren, feat)
			}, "monster:awakened-shrub")
			e := a.passTo(t, a.get(t, a.master), "Toren")
			a.setConditions(t, e, shrub, "condition:restrained")
			hit := a.mustAttack(t, a.caio, e, "Toren", rapier, shrub, func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
			g := &groupHit{a: a, e: e, shrub: shrub, id: hit.GetPendingDamage().GetId()}
			a.mustDamage(t, a.caio, e, g.id, extrasMarked([]*playv1.SelectedExtra{{Key: "sneak-attack"}}, &playv1.TypedPart{DamageType: groupPiercing, Sum: 9}))
			left := func() int32 { l, _ := poolOf(a.vitals(t, a.toren), rerollUses); return l }
			if left() != 2 {
				t.Fatalf("uses left = %d after a typed roll, want 2: typed dice are not rerolled", left())
			}
			if _, err := g.takeOut(t, "sneak-attack", "x"); err != nil {
				t.Fatalf("RemoveDamagePart() error = %v", err)
			}
			if tt.spent {
				a.correct(t, a.toren, func(r *playv1.AdjustCharacterVitalsRequest) {
					r.ResourcesUsed = []*playv1.ResourceUsed{{Key: rerollUses, Used: 2}}
				})
			}
			a.h.roller.queue(tt.faces...)
			res, err := a.damage(t, a.caio, e, g.id, inAppDamage)
			if err != nil {
				t.Fatalf("RollDamage() error = %v", err)
			}
			weapon := partRollOf(res.GetPendingDamage().GetPartRolls(), partWeapon)
			if weapon == nil || len(weapon.GetFaces()) != 1 || weapon.GetFaces()[0] != tt.wantFaces[0] {
				t.Errorf("the weapon rolled %v, want the faces %v", weapon, tt.wantFaces)
			}
			if got := left(); got != tt.wantLeft {
				t.Errorf("uses left = %d, want %d", got, tt.wantLeft)
			}
		})
	}
}

// ---- who reads the damage that waits for a sum (RN-10) ----

// The damage that waits for a sum, and the type it awaits, are the master's and the attacker's player's: another player may not read
// the attacker's turn options, and no line of the log they read names the status.
func TestTheDamageWaitingForASumIsNotReadByAnotherPlayer(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	if _, err := g.takeOut(t, "sneak-attack", "x"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	for _, c := range []struct {
		name string
		u    *user
		want bool
	}{{"master", g.a.master, true}, {"attacker", g.a.caio, true}, {"other player", g.a.ana, false}} {
		opts, err := g.a.options(t, c.u, g.e, "Toren")
		if (err == nil) != c.want {
			t.Errorf("%s: GetTurnOptions(Toren) error = %v, want it read: %v", c.name, err, c.want)
		}
		for _, p := range opts.GetPendingDamages() {
			if !c.want && (p.GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_GROUP_SUM || p.GetAwaitedDamageTypeKey() != "") {
				t.Errorf("%s reads the status %v and the awaited type %q", c.name, p.GetStatus(), p.GetAwaitedDamageTypeKey())
			}
		}
		for _, r := range g.a.log(t, c.u, g.e).GetRounds() {
			for _, en := range r.GetEntries() {
				if d := en.GetDamage(); !c.want && d != nil && d.GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_GROUP_SUM {
					t.Errorf("%s reads a log line with the status AWAITING_GROUP_SUM", c.name)
				}
			}
		}
	}
}

// ---- the highlights ----

// The damage that was taken back counts for nothing in the combat's highlights, as an undo's does: the damage dealt is what the
// second roll left, not the two rolls added up.
func TestTheHighlightsCountTheSecondRollOfAGroupSumAloneAndNotTheOneTakenBack(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	if _, err := g.takeOut(t, "sneak-attack", "x"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	if _, err := g.a.damage(t, g.a.caio, g.e, g.id, typedDamage(4)); err != nil {
		t.Fatalf("RollDamage() error = %v", err)
	}
	lost := g.before - g.hp(t)
	if lost <= 0 {
		t.Fatalf("the shrub lost %d hit points, want some", lost)
	}
	g.a.endEncounter(t, g.e)
	var dealt int32 = -1
	for _, c := range g.a.mustHighlights(t, g.a.master, g.e).GetCharacters() {
		if c.GetName() == "Toren" {
			dealt = c.GetDamageDealt()
		}
	}
	if dealt != lost {
		t.Errorf("Toren's damage dealt = %d, want %d: the roll that was taken back counts for nothing", dealt, lost)
	}
}

// ---- the roll to take back, found among the latest events ----

// rollEventOf finds the newest roll of the damage that no undo took back and no reaction window still holds, among rolls of other
// damages and events that are no roll.
func TestTheRollToTakeBackIsTheNewestOneOfTheDamageThatWasNotTakenBack(t *testing.T) {
	t.Parallel()
	row := func(kind string, ev actionEvent) playdb.ListRecentSessionEventsRow {
		payload, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("encode the event: %v", err)
		}
		return playdb.ListRecentSessionEventsRow{ID: newKey(), Kind: kind, Payload: payload}
	}
	older := row(eventDamageRolled, actionEvent{Pending: "p1"})
	other := row(eventDamageRolled, actionEvent{Pending: "p2"})
	held := row(eventDamageRolled, actionEvent{Pending: "p1", Reaction: &reactionEvent{Hold: true}})
	newer := row(eventDamageRolled, actionEvent{Pending: "p1"})
	moved := row(eventCombatantMoved, actionEvent{Pending: "p1"})
	// Newest first, as ListRecentSessionEvents lists them.
	got, _, ok := rollEventOf([]playdb.ListRecentSessionEventsRow{moved, other, held, newer, older}, "p1")
	if !ok || got.ID != newer.ID {
		t.Errorf("the roll = %v (found %v), want the newest roll of p1 that no window holds", got.ID, ok)
	}
	undo := row(eventActionUndone, actionEvent{Undone: newer.ID})
	got, _, ok = rollEventOf([]playdb.ListRecentSessionEventsRow{undo, moved, other, held, newer, older}, "p1")
	if !ok || got.ID != older.ID {
		t.Errorf("the roll = %v (found %v), want the older one: the newer was taken back", got.ID, ok)
	}
	if _, _, ok := rollEventOf([]playdb.ListRecentSessionEventsRow{moved, other}, "p1"); ok {
		t.Error("rollEventOf found a roll of p1 among events that hold none")
	}
}
