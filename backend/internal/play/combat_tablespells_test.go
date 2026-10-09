package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A spell the master wrote for the table in combat, end to end (MR-025, MR-045, RN-23,
// RN-22, Etapa 10, slice 10.2): an attack, a saving throw with half on a pass, healing, an
// area against three targets, several creatures, a cantrip, and one that only reaches the
// caster, cast through the same CombatSpell seam as the SRD's, with the campaign's content.
// These tests need the database (MEURPG_TEST_DATABASE_URL). The fixture is newCasters'
// party with the table's spells added to Pensantus (a wizard 5, so he has 3rd-level slots) and
// Brisa.

const (
	tableBolt   = "spell:raio-de-nanquim@mesa"
	tableBreath = "spell:sopro-de-nanquim@mesa"
	tableMend   = "spell:cura-de-nanquim@mesa"
	tableEcho   = "spell:eco-de-nanquim@mesa"
	tableWatch  = "spell:vigia-de-nanquim@mesa"
	tableSpark  = "spell:faisca-de-nanquim@mesa"
)

// tableSpell is a table spell of the fixture: level, class, range and target, and the rest
// as the caller sets it.
func tableSpell(name string, level int32, class string, rng rulesv1.SpellRangeKind, target *rulesv1.TableSpellTarget) *rulesv1.TableSpell {
	return &rulesv1.TableSpell{
		NamePt: name, Level: level, SchoolKey: "school:evocation",
		CastingTime: &rulesv1.TableSpellCastingTime{Unit: rulesv1.CastingTimeUnit_CASTING_TIME_UNIT_ACTION, Amount: 1},
		Range:       &rulesv1.TableSpellRange{Kind: rng, DistanceFt: map[bool]int32{true: 60}[rng == rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED]},
		Duration:    &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_INSTANTANEOUS},
		Components:  &rulesv1.TableSpellComponents{Verbal: true, Somatic: true},
		ClassKeys:   []string{class}, DescPt: []string{"Um efeito de teste da mesa."}, Target: target,
	}
}

func tableTarget(kind rulesv1.TableSpellTargetKind) *rulesv1.TableSpellTarget {
	return &rulesv1.TableSpellTarget{Kind: kind}
}

// newTableCasters is the party of newCasters with the table's spells: Pensantus a wizard 5
// (Raio, Sopro, Eco, Vigia and the cantrip Faísca, next to the SRD's), Brisa a cleric 3 with
// the Cura.
func newTableCasters(t *testing.T) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		add := func(s *rulesv1.TableSpell) {
			t.Helper()
			if _, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
				CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSpell{TableSpell: s},
			})); err != nil {
				t.Fatalf("CreateTableEntry(%s) error = %v", s.GetNamePt(), err)
			}
		}
		ranged, touch, self := rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED, rulesv1.SpellRangeKind_SPELL_RANGE_KIND_TOUCH, rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF
		one := tableTarget(rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURE)

		bolt := tableSpell("Raio de Nanquim", 1, "class:wizard", ranged, one)
		bolt.Attack = "ranged"
		bolt.Damage = []*rulesv1.TableSpellDamage{{DamageTypeKey: "damage-type:necrotic", Dice: "2d8", PerSlotLevel: "1d8"}}
		add(bolt)

		breath := tableSpell("Sopro de Nanquim", 1, "class:wizard", self, &rulesv1.TableSpellTarget{
			Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_AREA, Shape: rulesv1.TableAreaShape_TABLE_AREA_SHAPE_CONE, SizeFt: 15,
		})
		breath.Save = &rulesv1.SpellSave{Ability: rulesv1.Ability_ABILITY_DEXTERITY, OnSuccess: rulesv1.SpellSaveSuccess_SPELL_SAVE_SUCCESS_HALF}
		breath.Damage = []*rulesv1.TableSpellDamage{{DamageTypeKey: "damage-type:necrotic", Dice: "3d6", PerSlotLevel: "1d6"}}
		add(breath)

		mend := tableSpell("Cura de Nanquim", 1, "class:cleric", touch, one)
		mend.Heal = &rulesv1.TableSpellHeal{Dice: "1d8", PerSlotLevel: "1d8", AddsModifier: true}
		add(mend)

		echo := tableSpell("Eco de Nanquim", 2, "class:wizard", ranged, &rulesv1.TableSpellTarget{
			Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURES, Count: 2, PerSlotLevel: 1,
		})
		echo.Concentration = true
		echo.Duration = &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_TIMED, Amount: 1, Unit: rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_MINUTE, UpTo: true}
		echo.Save = &rulesv1.SpellSave{Ability: rulesv1.Ability_ABILITY_WISDOM, OnSuccess: rulesv1.SpellSaveSuccess_SPELL_SAVE_SUCCESS_NONE}
		add(echo)

		watch := tableSpell("Vigia de Nanquim", 1, "class:wizard", self, tableTarget(rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_SELF))
		watch.Concentration = true
		watch.Duration = &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_TIMED, Amount: 1, Unit: rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_HOUR, UpTo: true}
		add(watch)

		spark := tableSpell("Faísca de Nanquim", 0, "class:wizard", ranged, one)
		spark.Attack = "ranged"
		spark.Damage = []*rulesv1.TableSpellDamage{{DamageTypeKey: "damage-type:necrotic", Dice: "1d8", PerTier: "1d8"}}
		add(spark)

		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		book := []string{tableBolt, tableBreath, tableEcho, tableWatch, shieldSpell, scorchingRay, "spell:detect-magic"}
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 5,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt, tableSpark}, book, book)
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil,
			[]string{tableMend, cureWounds})
	})
}

func castD20(face int32) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) { r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: face} }
}

// TestMR025_ATableSpellAttackInCombat: a spell attack of the table's, cast with the
// campaign's content: one target, the dice by slot level (2d8 at the 1st, 3d8 at the 2nd),
// the slot, the miss, the log with the spell's name, and a retired spell that still works for
// the sheets that have it.
func TestMR025_ATableSpellAttackInCombat(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 1)

	opts := a.mustOptions(t, a.ana, e, "Pensantus")
	bolt := spellOption(opts, tableBolt)
	if bolt == nil || !bolt.GetEnabled() || len(bolt.GetSlots()) != 3 || bolt.GetSpell().GetNamePt() != "Raio de Nanquim" {
		t.Fatalf("the table spell's option = %v, want it enabled with the 1st, 2nd and 3rd circles", bolt)
	}
	if st := spellTargetsOf(opts, tableBolt); st == nil || st.GetMaxTargets() != 1 || st.GetExtraTargetPerLevel() || st.GetTargetsPerLevel() != 0 {
		t.Errorf("the attack's targets = %v, want one target and no more per circle", st)
	}
	// A player takes one target for a spell attack, as the master wrote it.
	_, err := a.cast(t, a.ana, e, "Pensantus", tableBolt, slotOfLevel(1), a.at(t, "Goblin", "Capitão Goblin"), castD20(15))
	wantCode(t, "two targets for a spell attack", err, connect.CodeInvalidArgument)
	if got := usedSlots(a.vitals(t, a.pens), 1); got != 0 {
		t.Fatalf("a refused cast spent %d slots", got)
	}

	// +6 to hit (Intelligence +3, proficiency +3) against the Capitão's 18: a 15 hits.
	hit := a.mustCast(t, a.ana, e, "Pensantus", tableBolt, slotOfLevel(1), a.at(t, "Capitão Goblin"), castDisadvantage(15))
	tg := hit.GetCast().GetTargets()[0]
	if tg.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || tg.GetPendingDamageId() == "" {
		t.Fatalf("the attack = %v, want a hit with damage to roll", tg)
	}
	if p := hit.GetCast().GetPendingDamages()[0]; p.GetDiceCount() != 2 || p.GetDiceSides() != 8 || p.GetDamageTypeKey() != "damage-type:necrotic" || p.GetHealing() {
		t.Errorf("pending damage = %v, want 2d8 necrotic", p)
	}
	a.h.roller.queue(5, 4)
	if d := a.mustDamage(t, a.ana, e, hit.GetCast().GetPendingDamages()[0].GetId(), inAppDamage).GetPendingDamage(); d.GetAmount() != 9 {
		t.Errorf("the damage = %v, want 9", d)
	}
	if cur, _, _ := a.hp(t, "Capitão Goblin"); cur != 18 {
		t.Errorf("Capitão = %d PV, want 18", cur)
	}
	if got := usedSlots(a.vitals(t, a.pens), 1); got != 1 {
		t.Errorf("1st-level slots used = %d, want 1", got)
	}
	entry := spellEntry(t, a.log(t, a.master, e))
	if entry.GetKey() != tableBolt || entry.GetKeyNamePt() != "Raio de Nanquim" || entry.GetSpell().GetSlot().GetLevel() != 1 ||
		len(entry.GetSpell().GetTargets()) != 1 || entry.GetSpell().GetTargets()[0].GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Errorf("the log's entry = %v, want the table spell by name, slot 1, a hit", entry)
	}

	// Upcast at the 2nd circle (the master casts again for him): 3d8. A 2 misses: no damage.
	up := a.mustCast(t, a.master, e, "Pensantus", tableBolt, slotOfLevel(2), a.at(t, "Goblin"), castDisadvantage(15))
	if p := up.GetCast().GetPendingDamages()[0]; p.GetDiceCount() != 3 || p.GetDiceSides() != 8 {
		t.Errorf("at the 2nd circle: pending damage = %v, want 3d8", p)
	}
	miss := a.mustCast(t, a.master, e, "Pensantus", tableBolt, slotOfLevel(1), a.at(t, "Capitão Goblin"), castDisadvantage(2))
	if tg := miss.GetCast().GetTargets()[0]; tg.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS || len(miss.GetCast().GetPendingDamages()) != 0 {
		t.Errorf("a 2 = %v, want a miss with no damage", miss.GetCast())
	}
	if got := usedSlots(a.vitals(t, a.pens), 1); got != 2 {
		t.Errorf("1st-level slots used = %d, want 2 (the hit and the miss spend one each)", got)
	}
	if got := usedSlots(a.vitals(t, a.pens), 2); got != 1 {
		t.Errorf("2nd-level slots used = %d, want 1", got)
	}

	// The master retires the spell: the sheets that have it keep casting it (RN-23).
	details, err := a.ana.content.GetSpellDetails(t.Context(), connect.NewRequest(&rulesv1.GetSpellDetailsRequest{CampaignId: a.campaignID, SpellKey: tableBolt}))
	if err != nil || details.Msg.GetSpell().GetHitPointEffect() != nil || details.Msg.GetSpell().GetTarget().GetLabelPt() != "Uma criatura" {
		t.Errorf("the spell's details = %v, %v; want a creature target and no hit-point effect", details, err)
	}
	if _, err := a.master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: a.campaignID, Key: tableBolt})); err != nil {
		t.Fatalf("ArchiveTableEntry() error = %v", err)
	}
	if o := spellOption(a.mustOptions(t, a.ana, e, "Pensantus"), tableBolt); o == nil {
		t.Error("the retired spell is gone from the sheet that has it")
	}
	a.mustCast(t, a.master, e, "Pensantus", tableBolt, slotOfLevel(1), a.at(t, "Capitão Goblin"), castDisadvantage(15))
}

// TestMR025_ATableAreaSpellWithASaveAgainstThreeTargets: a cone from the caster
// (Pessoal, an area): the caster picks who it catches, any number; the server rolls each
// save; the damage is one roll for the whole cast, all of it for the ones that fail and half for
// the one that saves, and it grows by circle.
func TestMR025_ATableAreaSpellWithASaveAgainstThreeTargets(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 2)

	opts := a.mustOptions(t, a.ana, e, "Pensantus")
	st := spellTargetsOf(opts, tableBreath)
	if st == nil || st.GetMaxTargets() != 0 || len(st.GetTargets()) < 4 {
		t.Fatalf("the cone's targets = %v, want any number, and everyone to pick from", st)
	}
	for _, tg := range st.GetTargets() {
		if tg.GetTooFar() {
			t.Errorf("the cone's target %s is too far: an area from the caster reaches who the caster picks", tg.GetLabel())
		}
	}

	// DC 14 = 8 + 3 + 3. Goblin 1 rolls 5 and Capitão 12 and fail; Goblin 2 rolls 18 and saves.
	a.h.roller.queue(5, 18, 12)
	cast := a.mustCast(t, a.ana, e, "Pensantus", tableBreath, slotOfLevel(1), a.at(t, "Goblin 1", "Goblin 2", "Capitão Goblin"), noCastRoll)
	saves := map[string]*playv1.SaveResult{}
	for _, tg := range cast.GetCast().GetTargets() {
		saves[tg.GetCombatantId()] = tg.GetSave()
	}
	if saves[a.id(t, "Goblin 1")].GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_FAILED || saves[a.id(t, "Goblin 2")].GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED ||
		saves[a.id(t, "Capitão Goblin")].GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_FAILED {
		t.Fatalf("the saves = %v; want failed, saved, failed", saves)
	}
	if len(cast.GetCast().GetPendingDamages()) != 3 {
		t.Fatalf("pending damages = %d, want one for each target", len(cast.GetCast().GetPendingDamages()))
	}
	halves := 0
	for _, p := range cast.GetCast().GetPendingDamages() {
		if p.GetDiceCount() != 3 || p.GetDiceSides() != 6 || p.GetDamageTypeKey() != "damage-type:necrotic" {
			t.Errorf("pending damage = %v, want 3d6 necrotic", p)
		}
		if p.GetHalf() {
			halves++
		}
	}
	if halves != 1 {
		t.Errorf("%d pending damages are half, want 1", halves)
	}
	a.h.roller.queue(3, 3, 3) // 9: Goblin 1 (7 PV) is defeated, Goblin 2 takes 4, the Capitão 9
	res := a.mustDamage(t, a.ana, e, cast.GetCast().GetPendingDamages()[0].GetId(), inAppDamage)
	if len(res.GetCastPendingDamages()) != 2 {
		t.Fatalf("the roll settled %d other damages, want 2", len(res.GetCastPendingDamages()))
	}
	if cur, _, defeated := a.hp(t, "Goblin 1"); cur != 0 || !defeated {
		t.Errorf("Goblin 1 = %d PV, defeated %v; want 0, defeated", cur, defeated)
	}
	if cur, _, _ := a.hp(t, "Goblin 2"); cur != 3 {
		t.Errorf("Goblin 2 = %d PV, want 3 (half of 9 is 4)", cur)
	}
	if cur, _, _ := a.hp(t, "Capitão Goblin"); cur != 18 {
		t.Errorf("Capitão = %d PV, want 18", cur)
	}
	if got := usedSlots(a.vitals(t, a.pens), 1); got != 1 {
		t.Errorf("1st-level slots used = %d, want 1", got)
	}
	entry := spellEntry(t, a.log(t, a.master, e))
	if entry.GetKey() != tableBreath || entry.GetKeyNamePt() != "Sopro de Nanquim" || len(entry.GetSpell().GetTargets()) != 3 {
		t.Errorf("the log's entry = %v, want the table spell by name and three targets", entry)
	}

	// At the 2nd circle: 4d6, and an area with no target at all is a cast too.
	up := a.mustCast(t, a.master, e, "Pensantus", tableBreath, slotOfLevel(2), a.at(t, "Capitão Goblin"), noCastRoll)
	if p := up.GetCast().GetPendingDamages(); len(p) != 1 || p[0].GetDiceCount() != 4 || p[0].GetDiceSides() != 6 {
		t.Errorf("at the 2nd circle: pending damage = %v, want 4d6", p)
	}
	a.mustCast(t, a.master, e, "Pensantus", tableBreath, slotOfLevel(1), nil, noCastRoll)
}

// TestMR025_ATableHealingSpell: healing by circle with the spellcasting modifier, at a
// touch: the one next to the caster, not the one two squares away.
func TestMR025_ATableHealingSpell(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 1)
	a.correct(t, a.toren, hpIs(3))
	a.mustEndTurn(t, a.ana, e) // Toren
	a.mustEndTurn(t, a.caio, e)

	// The Capitão is two squares from Brisa: Toque does not reach.
	_, err := a.cast(t, a.bia, e, "Brisa", tableMend, slotOfLevel(1), a.at(t, "Capitão Goblin"), noCastRoll)
	wantBlockedBy(t, "a touch spell two squares away", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH)
	if got := usedSlots(a.vitals(t, a.bri), 1); got != 0 {
		t.Fatalf("a refused cast spent %d slots", got)
	}
	cast := a.mustCast(t, a.bia, e, "Brisa", tableMend, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
	p := cast.GetCast().GetPendingDamages()[0]
	if !p.GetHealing() || p.GetDiceCount() != 1 || p.GetDiceSides() != 8 || p.GetBonus() != 3 {
		t.Fatalf("the heal = %v, want 1d8 + 3", p)
	}
	a.h.roller.queue(4)
	if d := a.mustDamage(t, a.bia, e, p.GetId(), inAppDamage).GetPendingDamage(); d.GetAmount() != 7 || d.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		t.Errorf("the heal = %v, want 7 applied at once", d)
	}
	if got := a.vitals(t, a.toren).GetHitPointsCurrent(); got != 10 {
		t.Errorf("Toren = %d PV, want 10", got)
	}
	// At the 2nd circle (the master casts again): 2d8 + 3.
	up := a.mustCast(t, a.master, e, "Brisa", tableMend, slotOfLevel(2), a.at(t, "Toren"), noCastRoll)
	if p := up.GetCast().GetPendingDamages()[0]; p.GetDiceCount() != 2 || p.GetDiceSides() != 8 || p.GetBonus() != 3 {
		t.Errorf("at the 2nd circle: the heal = %v, want 2d8 + 3", p)
	}
	if got := usedSlots(a.vitals(t, a.bri), 1); got != 1 {
		t.Errorf("1st-level slots used = %d, want 1", got)
	}
	if entry := spellEntry(t, a.log(t, a.master, e)); entry.GetKeyNamePt() != "Cura de Nanquim" {
		t.Errorf("the log's entry = %v, want the table spell by name", entry)
	}
}

// TestMR025_SeveralCreaturesConcentrationAndOnlyTheCaster: "several creatures" takes the
// number the master wrote and more for each circle; a concentration spell of the table
// is replaced by another and the log says which ended (RN-22); a spell that only reaches the
// caster takes no other target.
func TestMR025_SeveralCreaturesConcentrationAndOnlyTheCaster(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 2)

	opts := a.mustOptions(t, a.ana, e, "Pensantus")
	if st := spellTargetsOf(opts, tableEcho); st == nil || st.GetMaxTargets() != 2 || !st.GetExtraTargetPerLevel() || st.GetTargetsPerLevel() != 1 {
		t.Errorf("the Eco's targets = %v, want two, and one more for each circle", st)
	}
	if st := spellTargetsOf(opts, tableWatch); st == nil || st.GetMaxTargets() != 0 || len(st.GetTargets()) != 1 {
		t.Errorf("the Vigia's targets = %v, want only the caster to pick from", st)
	}
	three := a.at(t, "Goblin 1", "Goblin 2", "Capitão Goblin")

	// At the 2nd circle the Eco takes two: three are refused, and nothing is spent.
	_, err := a.cast(t, a.ana, e, "Pensantus", tableEcho, slotOfLevel(2), three, noCastRoll)
	wantCode(t, "three targets at the 2nd circle", err, connect.CodeInvalidArgument)
	if got := usedSlots(a.vitals(t, a.pens), 2); got != 0 {
		t.Fatalf("a refused cast spent %d slots", got)
	}
	// At the 3rd circle it takes three. The saves (DC 14): 4 fails, 15 saves, 9 fails.
	a.h.roller.queue(4, 15, 9)
	echo := a.mustCast(t, a.ana, e, "Pensantus", tableEcho, slotOfLevel(3), three, noCastRoll)
	if !echo.GetCast().GetConcentrating() || byLabel(t, echo.GetEncounter(), "Pensantus").GetConcentrationSpell() != tableEcho || len(echo.GetCast().GetPendingDamages()) != 0 {
		t.Fatalf("the Eco's cast = %v, want Pensantus concentrating on it and no damage to roll", echo.GetCast())
	}
	var outcomes []playv1.SaveOutcome
	for _, tg := range echo.GetCast().GetTargets() {
		outcomes = append(outcomes, tg.GetSave().GetOutcome())
	}
	if len(outcomes) != 3 || outcomes[0] != playv1.SaveOutcome_SAVE_OUTCOME_FAILED || outcomes[1] != playv1.SaveOutcome_SAVE_OUTCOME_SAVED || outcomes[2] != playv1.SaveOutcome_SAVE_OUTCOME_FAILED {
		t.Errorf("the saves = %v, want failed, saved, failed", outcomes)
	}
	if got := usedSlots(a.vitals(t, a.pens), 3); got != 1 {
		t.Errorf("3rd-level slots used = %d, want 1", got)
	}

	// The Vigia reaches only the caster: another target is refused, none is a cast, and it
	// replaces the concentration on the Eco.
	_, err = a.cast(t, a.master, e, "Pensantus", tableWatch, slotOfLevel(1), a.at(t, "Goblin 1"), noCastRoll)
	wantCode(t, "a target for a spell that reaches only the caster", err, connect.CodeInvalidArgument)
	watch := a.mustCast(t, a.master, e, "Pensantus", tableWatch, slotOfLevel(1), nil, noCastRoll)
	if watch.GetCast().GetConcentrationEndedSpellKey() != tableEcho || byLabel(t, watch.GetEncounter(), "Pensantus").GetConcentrationSpell() != tableWatch {
		t.Errorf("the Vigia's cast = %v, want the Eco ended and the Vigia on", watch.GetCast())
	}
	entry := spellEntry(t, a.log(t, a.caio, e))
	if entry.GetKey() != tableWatch || entry.GetSpell().GetConcentrationEndedKey() != tableEcho || !entry.GetSpell().GetConcentrating() {
		t.Errorf("the log's entry = %v, want the Vigia and the Eco ended", entry)
	}
}

// TestMR025_ATableCantripGrowsByTheCharactersLevel: an attack cantrip of the table is rolled
// as an attack (like Raio de Fogo), its dice by tier come from the character's level (a
// wizard 5 is at the second tier), and it spends no slot.
func TestMR025_ATableCantripGrowsByTheCharactersLevel(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 1)
	opts := a.mustOptions(t, a.ana, e, "Pensantus")
	if o := attackOption(opts, tableSpark); o == nil || !o.GetEnabled() || o.GetAttack().GetKind() != rulesv1.AttackKind_ATTACK_KIND_SPELL {
		t.Fatalf("the cantrip's attack option = %v, want an enabled spell attack", o)
	}
	_, err := a.cast(t, a.ana, e, "Pensantus", tableSpark, nil, a.at(t, "Capitão Goblin"), castD20(15))
	wantCode(t, "CastSpell for an attack cantrip", err, connect.CodeInvalidArgument)

	res := a.mustAttack(t, a.ana, e, "Pensantus", tableSpark, "Capitão Goblin", disadvantage(15))
	if p := res.GetPendingDamage(); p == nil || p.GetDiceCount() != 2 || p.GetDiceSides() != 8 || p.GetDamageTypeKey() != "damage-type:necrotic" {
		t.Errorf("the cantrip's damage = %v, want 2d8 necrotic for a character of level 5", p)
	}
	for lvl := int32(1); lvl <= 3; lvl++ {
		if got := usedSlots(a.vitals(t, a.pens), lvl); got != 0 {
			t.Errorf("slots used at circle %d = %d, a cantrip spends none", lvl, got)
		}
	}
}

// TestMR025_ScorchingRayTakesOneMoreRayForEachCircle: the rays are targets: three at the
// 2nd circle and one more for each circle above, and the app is told (max_targets plus
// targets_per_level), as for Magic Missile's darts.
func TestMR025_ScorchingRayTakesOneMoreRayForEachCircle(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 4) // four goblins and the Capitão: five to pick from
	st := spellTargetsOf(a.mustOptions(t, a.ana, e, "Pensantus"), scorchingRay)
	if st == nil || st.GetMaxTargets() != 3 || !st.GetExtraTargetPerLevel() || st.GetTargetsPerLevel() != 1 {
		t.Fatalf("Raio Ardente's targets = %v, want 3, and one more for each circle", st)
	}
	five := a.at(t, "Goblin 1", "Goblin 2", "Goblin 3", "Goblin 4", "Capitão Goblin")
	_, err := a.cast(t, a.ana, e, "Pensantus", scorchingRay, slotOfLevel(3), five, poolInApp)
	wantCode(t, "five rays at the 3rd circle", err, connect.CodeInvalidArgument)
	if got := usedSlots(a.vitals(t, a.pens), 3); got != 0 {
		t.Fatalf("a refused cast spent %d slots", got)
	}
	a.h.roller.queue(10, 10, 10, 10)
	cast := a.mustCast(t, a.ana, e, "Pensantus", scorchingRay, slotOfLevel(3), five[:4], poolInApp)
	if len(cast.GetCast().GetTargets()) != 4 {
		t.Errorf("rays at the 3rd circle = %d, want 4", len(cast.GetCast().GetTargets()))
	}
	// Each ray is its own spell attack (+6 against the goblins' 15: a 10 hits) for 2d6 fire.
	for i, tg := range cast.GetCast().GetTargets() {
		if tg.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || tg.GetPendingDamageId() == "" {
			t.Errorf("ray %d = %v, want a hit with damage to roll", i+1, tg)
		}
	}
	pending := cast.GetCast().GetPendingDamages()
	if len(pending) != 4 {
		t.Fatalf("pending damages = %d, want one for each ray that hit", len(pending))
	}
	for i, p := range pending {
		if p.GetDiceCount() != 2 || p.GetDiceSides() != 6 || p.GetDamageTypeKey() != "damage-type:fire" {
			t.Errorf("ray %d damage = %v, want 2d6 fire", i+1, p)
		}
	}
}

// TestMR025_ASpellForTheCasterAloneHasNobodyToPick: Detectar Magia (Pessoal) lists only
// the caster, takes no other target, and is cast with none.
func TestMR025_ASpellForTheCasterAloneHasNobodyToPick(t *testing.T) {
	t.Parallel()
	a := newTableCasters(t)
	e := a.castersFight(t, 1)
	st := spellTargetsOf(a.mustOptions(t, a.ana, e, "Pensantus"), "spell:detect-magic")
	if st == nil || st.GetMaxTargets() != 0 || len(st.GetTargets()) != 1 {
		t.Fatalf("Detectar Magia's targets = %v, want only the caster", st)
	}
	_, err := a.cast(t, a.ana, e, "Pensantus", "spell:detect-magic", slotOfLevel(1), a.at(t, "Goblin"), noCastRoll)
	wantCode(t, "a target for a spell that reaches the caster alone", err, connect.CodeInvalidArgument)
	a.mustCast(t, a.ana, e, "Pensantus", "spell:detect-magic", slotOfLevel(1), nil, noCastRoll)
}
