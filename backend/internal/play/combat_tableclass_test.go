package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A table class's third caster in combat, end to end (MR-025, RN-23, Etapa 10, slice
// 10.3, E10-02 state 7): a Fighter with the table's subclass "Lâmina de Tinta" casts
// the table's spell "Lâmina de Nanquim", which is on the wizard's list, with the
// subclass's ability (Intelligence) and its save DC, from the slots of its own table.
// These tests need the database (MEURPG_TEST_DATABASE_URL).

const (
	inkBladeSpell = "spell:lamina-de-nanquim@mesa"
	inkGripSpell  = "spell:aperto-de-nanquim@mesa"
)

// TestMR025_AThirdCasterOfTheTableCastsATableSpell: the option lists the spell with
// the 1st circle only (a Fighter 5 has three 1st-circle slots), the attack is
// Intelligence + proficiency against the Capitão's armor class, the damage is the
// 2d8 the master wrote, and the slot is spent; a spell that is not on the
// subclass's list is not among the options.
func TestMR025_AThirdCasterOfTheTableCastsATableSpell(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		defaults, err := a.master.table.GetClassTableDefaults(t.Context(), connect.NewRequest(&rulesv1.GetClassTableDefaultsRequest{CampaignId: a.campaignID}))
		if err != nil {
			t.Fatalf("GetClassTableDefaults() error = %v", err)
		}
		var third *rulesv1.CastingTableDefault
		for _, tab := range defaults.Msg.GetTables() {
			if tab.GetKind() == "third" && tab.GetPreparation() == "known" {
				third = tab
			}
		}
		sub := &rulesv1.TableSubclass{
			NamePt: "Lâmina de Tinta", ClassKey: "class:fighter", DescPt: []string{"Tinta e aço."},
			Casting: &rulesv1.TableCasting{Kind: "third", Ability: rulesv1.Ability_ABILITY_INTELLIGENCE, Preparation: "known", ListFrom: "class:wizard"},
		}
		for i, r := range third.GetRows()[third.GetStartLevel()-1:] {
			sub.Levels = append(sub.Levels, &rulesv1.TableSubclassLevel{
				Level: int32(i) + third.GetStartLevel(), CantripsKnown: r.GetCantripsKnown(), SpellsKnown: r.GetSpellsKnown(), Slots: slices.Clone(r.GetSlots()),
			})
		}
		create := func(body *rulesv1.CreateTableEntryRequest) *rulesv1.TableEntry {
			t.Helper()
			res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(body))
			if err != nil {
				t.Fatalf("CreateTableEntry() error = %v", err)
			}
			return res.Msg.GetEntry()
		}
		ink := create(&rulesv1.CreateTableEntryRequest{CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: sub}})
		blade := tableSpell("Lâmina de Nanquim", 1, "class:wizard", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED, tableTarget(rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURE))
		blade.Attack = "ranged"
		blade.Damage = []*rulesv1.TableSpellDamage{{DamageTypeKey: "damage-type:necrotic", Dice: "2d8", PerSlotLevel: "1d8"}}
		if got := create(&rulesv1.CreateTableEntryRequest{CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSpell{TableSpell: blade}}).GetKey(); got != inkBladeSpell {
			t.Fatalf("the spell's key = %q, want %q", got, inkBladeSpell)
		}

		grip := tableSpell("Aperto de Nanquim", 1, "class:wizard", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED, tableTarget(rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURE))
		grip.Save = &rulesv1.SpellSave{Ability: rulesv1.Ability_ABILITY_WISDOM, OnSuccess: rulesv1.SpellSaveSuccess_SPELL_SAVE_SUCCESS_NONE}
		grip.Damage = []*rulesv1.TableSpellDamage{{DamageTypeKey: "damage-type:necrotic", Dice: "2d6", PerSlotLevel: "1d6"}}
		if got := create(&rulesv1.CreateTableEntryRequest{CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSpell{TableSpell: grip}}).GetKey(); got != inkGripSpell {
			t.Fatalf("the spell's key = %q, want %q", got, inkGripSpell)
		}

		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		// Pensantus is the third caster: Fighter 5 of the Lâmina de Tinta, Intelligence 16,
		// two cantrips and four spells of the wizard's list, the table's among them.
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 16, Wisdom: 10, Charisma: 8}, RaceKey: "race:human",
			Background: &charactersv1.FullSheet_BackgroundKey{BackgroundKey: "background:acolyte"},
			Classes: []*charactersv1.ClassLevel{{
				ClassKey: "class:fighter", Level: 5, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: ink.GetKey()},
			}},
			SkillProficiencyKeys: []string{"skill:athletics", "skill:perception"},
			FeatureChoiceKeys:    []string{"feature:fighter-fighting-style-defense"},
			CantripKeys:          []string{fireBolt},
			KnownSpellKeys:       []string{inkBladeSpell, inkGripSpell, magicMissileSpell, shieldSpell},
			WeaponKeys:           []string{"equipment:longsword"},
			HitPoints:            &charactersv1.HitPoints{Method: charactersv1.HitPointsMethod_HIT_POINTS_METHOD_AVERAGE}, ExperiencePoints: 6500,
		}}}
		res, err := a.ana.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Pensantus", Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Pensantus) error = %v", err)
		}
		a.pens = res.Msg.GetCharacter()
		if issues := a.pens.GetDerived().GetIssues(); len(issues) != 0 {
			t.Fatalf("the third caster's sheet has issues: %v", issues)
		}
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
	})
	e := a.castersFight(t, 1)

	opts := a.mustOptions(t, a.ana, e, "Pensantus")
	blade := spellOption(opts, inkBladeSpell)
	if blade == nil || !blade.GetEnabled() || len(blade.GetSlots()) != 1 || blade.GetSlots()[0].GetLevel() != 1 || blade.GetSlots()[0].GetFree() != 3 || blade.GetSpell().GetNamePt() != "Lâmina de Nanquim" {
		t.Fatalf("the table spell's option = %v, want it enabled with the three 1st-circle slots of a third caster of level 5", blade)
	}
	if st := spellTargetsOf(opts, inkBladeSpell); st == nil || st.GetMaxTargets() != 1 {
		t.Errorf("the spell's targets = %v, want one", st)
	}
	// An SRD spell of the same wizard list works for him too, with the same numbers.
	if mm := spellOption(opts, magicMissileSpell); mm == nil || !mm.GetEnabled() {
		t.Errorf("Magic Missile = %v, want it among the third caster's enabled options", mm)
	}

	// Intelligence +3 and proficiency +3: +6 against the Capitão's 18; a 15 hits.
	hit := a.mustCast(t, a.ana, e, "Pensantus", inkBladeSpell, slotOfLevel(1), a.at(t, "Capitão Goblin"), castDisadvantage(15))
	tg := hit.GetCast().GetTargets()[0]
	if tg.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || tg.GetPendingDamageId() == "" {
		t.Fatalf("the attack = %v, want a hit with damage to roll", tg)
	}
	if roll := tg.GetAttackRoll(); roll.GetTotal() != 21 || roll.GetModifier() != 6 {
		t.Errorf("the attack roll = %v, want d20 15 + 6 = 21 (Intelligence +3, proficiency +3, the subclass's)", roll)
	}
	if p := hit.GetCast().GetPendingDamages()[0]; p.GetDiceCount() != 2 || p.GetDiceSides() != 8 || p.GetDamageTypeKey() != "damage-type:necrotic" {
		t.Errorf("pending damage = %v, want 2d8 necrotic", p)
	}
	a.h.roller.queue(5, 6)
	if d := a.mustDamage(t, a.ana, e, hit.GetCast().GetPendingDamages()[0].GetId(), inAppDamage).GetPendingDamage(); d.GetAmount() != 11 {
		t.Errorf("the damage = %v, want 11", d)
	}
	if cur, _, _ := a.hp(t, "Capitão Goblin"); cur != 16 {
		t.Errorf("Capitão = %d PV, want 16", cur)
	}
	if got := usedSlots(a.vitals(t, a.pens), 1); got != 1 {
		t.Errorf("1st-level slots used = %d, want 1", got)
	}
	if entry := spellEntry(t, a.log(t, a.master, e)); entry.GetKey() != inkBladeSpell || entry.GetKeyNamePt() != "Lâmina de Nanquim" || entry.GetSpell().GetSlot().GetLevel() != 1 {
		t.Errorf("the log's entry = %v, want the table spell by name at the 1st circle", entry)
	}
	// The save DC of a spell that asks for one is the subclass's too: 8 + 3 + 3 = 14, as the
	// caster's player reads it (the outcome and the DC, never an NPC's dice).
	a.h.roller.queue(3)
	grip := a.mustCast(t, a.master, e, "Pensantus", inkGripSpell, slotOfLevel(1), a.at(t, "Goblin"), noCastRoll)
	if sv := grip.GetCast().GetTargets()[0].GetSave(); sv.GetDc() != 14 || sv.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_FAILED {
		t.Errorf("the save = %v, want a failure against DC 14 (Intelligence +3, proficiency +3)", sv)
	}
	// A 2nd-circle slot he does not have is refused, and spends nothing.
	_, err := a.cast(t, a.master, e, "Pensantus", inkBladeSpell, slotOfLevel(2), a.at(t, "Goblin"), castD20(15))
	wantCode(t, "a 2nd-circle slot the third caster does not have", err, connect.CodeInvalidArgument)
}
