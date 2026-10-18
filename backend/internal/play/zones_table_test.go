package play

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The area a table spell keeps on the map (RN-37): the master's own spell, with a neutral fixture, goes through the
// same zone as the SRD's. These tests need the database (MEURPG_TEST_DATABASE_URL).

const (
	tableDryZone   = "spell:zona-de-teste-seca@mesa"
	tableSaveZone  = "spell:zona-de-teste@mesa"
	tableFollowing = "spell:zona-seguidora@mesa"
)

// tableZoneSpell is a table spell that keeps a cube of 5 feet at a point (or a sphere that follows the caster), for a minute.
func tableZoneSpell(name string, save bool, trigger rulesv1.TableZoneTrigger, follows bool) *rulesv1.TableSpell {
	rng := rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED
	target := &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_AREA, Shape: rulesv1.TableAreaShape_TABLE_AREA_SHAPE_CUBE, SizeFt: 5}
	if follows {
		rng = rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF
		target.Shape, target.SizeFt = rulesv1.TableAreaShape_TABLE_AREA_SHAPE_SPHERE, 15
	}
	s := tableSpell(name, 2, "class:wizard", rng, target)
	s.Concentration = true
	s.Duration = &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_TIMED, Amount: 1, Unit: rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_MINUTE, UpTo: true}
	s.Damage = []*rulesv1.TableSpellDamage{{DamageTypeKey: "damage-type:force", Dice: "2d6", PerSlotLevel: "1d6"}}
	if save {
		s.Save = &rulesv1.SpellSave{Ability: rulesv1.Ability_ABILITY_CONSTITUTION, OnSuccess: rulesv1.SpellSaveSuccess_SPELL_SAVE_SUCCESS_HALF}
	}
	s.Zone = &rulesv1.TableSpellZone{Trigger: trigger, FollowsCaster: follows}
	return s
}

func addTableSpell(t *testing.T, a *armed, s *rulesv1.TableSpell) error {
	t.Helper()
	_, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
		CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSpell{TableSpell: s},
	}))
	return err
}

// tableZoneCave is the cave with Pensantus a wizard of level 5 who knows the three table zone spells.
func tableZoneCave(t *testing.T) *cave {
	t.Helper()
	return newCaveOf(t, 5, []string{tableDryZone, tableSaveZone, tableFollowing}, func(a *armed) *charactersv1.Character {
		both, entry := rulesv1.TableZoneTrigger_TABLE_ZONE_TRIGGER_BOTH, rulesv1.TableZoneTrigger_TABLE_ZONE_TRIGGER_FIRST_ENTRY
		for _, s := range []*rulesv1.TableSpell{
			tableZoneSpell("Zona de Teste Seca", false, both, false),
			tableZoneSpell("Zona de Teste", true, entry, false),
			tableZoneSpell("Zona Seguidora", true, both, true),
		} {
			if err := addTableSpell(t, a, s); err != nil {
				t.Fatalf("CreateTableEntry(%s) error = %v", s.GetNamePt(), err)
			}
		}
		return a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, abilities(15, 13, 14, 10, 8), []string{battleaxe}, nil)
	})
}

// A table spell with no saving throw leaves a zone whose damage waits to be rolled by whoever rolls the caster's damage.
func TestATableSpellWithNoSaveKeepsAZoneThatOffersTheDamageRoll(t *testing.T) {
	t.Parallel()
	c := tableZoneCave(t)
	e := c.zoneFight(t)
	c.mustMove(t, c.master, "Ogro", 14, 7)
	c.passTo(t, e, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", tableDryZone, slotOfLevel(2), at(16, 7), nil)
	z := c.oneZone(t, c.master, tableDryZone)
	if z.GetSaveAbility() != "" || z.GetDamageDice() != "2d6" || z.GetDamageTypeKey() != "damage-type:force" || !z.GetConcentration() || z.GetRoundsLeft() != 10 {
		t.Fatalf("zone = %v, want 2d6 force, no save, concentration, 10 rounds", z)
	}
	if _, err := c.move(t, c.master, "Ogro", 16, 7); err != nil {
		t.Fatalf("MoveCombatant() error = %v", err)
	}
	var id, status string
	if err := c.h.pool.QueryRow(t.Context(), `SELECT id, status FROM pending_damages WHERE target_id = $1 AND attack_key = $2`, c.id(t, "Ogro"), tableDryZone).Scan(&id, &status); err != nil || status != "awaiting_roll" {
		t.Fatalf("pending damage = %q, %v; want one awaiting its roll", status, err)
	}
	c.h.roller.queue(3, 4)
	if d := c.mustDamage(t, c.ana, c.get(t, c.master), id, inAppDamage).GetPendingDamage(); d.GetAmount() != 7 {
		t.Errorf("damage = %d, want 7", d.GetAmount())
	}
}

// A zone that catches only on the first entry leaves alone who begins its turn inside, and catches who comes in.
func TestATableZoneOfFirstEntryCatchesWhoComesIn(t *testing.T) {
	t.Parallel()
	c := tableZoneCave(t)
	e := c.zoneFight(t)
	c.mustMove(t, c.master, "Ogro", 16, 7)
	c.passTo(t, e, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", tableSaveZone, slotOfLevel(2), at(16, 7), nil)
	c.nextTurnOf(t, "Ogro")
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59 {
		t.Fatalf("the ogre's hit points = %d, want 59: a first-entry zone does not catch who begins its turn inside", cur)
	}
	c.mustMove(t, c.master, "Ogro", 14, 7)
	c.h.roller.queue(5, 3, 3)
	if _, err := c.move(t, c.master, "Ogro", 16, 7); err != nil {
		t.Fatalf("MoveCombatant() error = %v", err)
	}
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-6 {
		t.Errorf("the ogre's hit points = %d, want 53: it came in and failed the save", cur)
	}
}

// A table zone that follows the caster goes where the caster goes.
func TestATableZoneCanFollowItsCaster(t *testing.T) {
	t.Parallel()
	c := tableZoneCave(t)
	e := c.zoneFight(t)
	c.passTo(t, e, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", tableFollowing, slotOfLevel(2), at(12, 7), nil)
	c.mustMove(t, c.ana, "Pensantus", 10, 7)
	if z := c.oneZone(t, c.master, tableFollowing); z.GetOrigin().GetCol() != 10 || z.GetMovement() != playv1.SpellZoneMovement_SPELL_ZONE_MOVEMENT_FOLLOWS_CASTER {
		t.Errorf("zone = %v, want one centered on (10,7) that follows the caster", z)
	}
}

// The master's spell is refused at the field that is wrong, one sentence for each.
func TestATableSpellZoneIsRefusedAtTheFieldThatIsWrong(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(*armed) {})
	s := tableZoneSpell("Zona Sem Gatilho", true, rulesv1.TableZoneTrigger_TABLE_ZONE_TRIGGER_UNSPECIFIED, false)
	err := addTableSpell(t, a, s)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateTableEntry() error = %v, want invalid_argument", err)
	}
	found := false
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("error is %T, want a connect error", err)
	}
	var fields []string
	for _, d := range ce.Details() {
		if v, e := d.Value(); e == nil {
			if r, ok := v.(*rulesv1.TableContentRefusal); ok {
				for _, vi := range r.GetViolations() {
					fields = append(fields, vi.GetField())
					found = found || vi.GetField() == "table_spell.zone.trigger"
				}
			}
		}
	}
	if !found {
		t.Errorf("violations at %v, want one at table_spell.zone.trigger", fields)
	}
}
