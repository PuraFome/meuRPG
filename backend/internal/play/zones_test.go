package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The areas a spell keeps on the map (SRD 5.1: Spirit Guardians, Moonbeam, Cloudkill, Web). These
// tests need the database (MEURPG_TEST_DATABASE_URL). The fixture is the cave; the strip of row 7
// is open from column 0 to 22, so the distances are the columns apart. Toren is the caster; the
// ogre (59 hit points, saving throws +0) is the creature the zones hit.

const (
	spiritGuardiansKey = "spell:spirit-guardians"
	moonbeamKey        = "spell:moonbeam"
	cloudkillKey       = "spell:cloudkill"
	webKey             = "spell:web"
)

// zoneCave is the cave with Toren a cleric of level 7 who prepared Spirit Guardians and Bless (Wisdom 16:
// save DC 14), and Pensantus a wizard of level 9 who knows Cloudkill, Web and Thunderwave.
func zoneCave(t *testing.T) *cave {
	t.Helper()
	return newCaveOf(t, 9, []string{cloudkillKey, webKey, thunderwave}, func(a *armed) *charactersv1.Character {
		return a.caio.caster(t, a.campaignID, "Toren", "class:cleric", "race:human", 7, abilities(14, 12, 14, 16, 10),
			[]string{maceKey}, []string{sacredFlame}, nil, []string{spiritGuardiansKey, blessKey})
	})
}

// druidCave is the cave with Toren a druid of level 3 who prepared Moonbeam (Wisdom 16: save DC 13).
func druidCave(t *testing.T) *cave {
	t.Helper()
	return newCaveOf(t, 5, []string{thunderwave}, func(a *armed) *charactersv1.Character {
		return a.caio.caster(t, a.campaignID, "Toren", "class:druid", "race:human", 3, abilities(12, 12, 14, 16, 10),
			nil, nil, nil, []string{moonbeamKey})
	})
}

// zoneFight starts the combat on the strip: Toren first at (5,7), Pensantus (12,7), Brisa (3,7), the
// ogre (7,7) and a goblin far to the east (20,7), the NPCs last. Toren is on turn.
func (c *cave) zoneFight(t *testing.T) *playv1.Encounter {
	t.Helper()
	return c.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: c.ogre.GetId()}, {CharacterId: c.goblins.GetId()}},
		npcRolls: []int{2, 2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 5},
		reveal:   []string{"Ogro", "Goblin"},
		at:       map[string][2]int32{"Toren": {5, 7}, "Pensantus": {12, 7}, "Brisa": {3, 7}, "Ogro": {7, 7}, "Goblin": {20, 7}},
	})
}

// zonesOf are the zones a viewer reads, by spell.
func zonesOf(e *playv1.Encounter, spell string) []*playv1.SpellZone {
	var out []*playv1.SpellZone
	for _, z := range e.GetZones() {
		if z.GetSpellKey() == spell {
			out = append(out, z)
		}
	}
	return out
}

func zoneHas(z *playv1.SpellZone, col, row int32) bool {
	return slices.ContainsFunc(z.GetSquares(), func(s *playv1.SpellOrigin) bool { return s.GetCol() == col && s.GetRow() == row })
}

// oneZone is the only zone of the spell the master reads.
func (c *cave) oneZone(t *testing.T, u *user, spell string) *playv1.SpellZone {
	t.Helper()
	zs := zonesOf(c.get(t, u), spell)
	if len(zs) != 1 {
		t.Fatalf("zones of %s = %v, want exactly one", spell, zs)
	}
	return zs[0]
}

// castSpiritGuardians has Toren cast the aura at the given slot level.
func (c *cave) castSpiritGuardians(t *testing.T, slot int32) {
	t.Helper()
	c.mustCastArea(t, c.caio, "Toren", spiritGuardiansKey, slotOfLevel(slot), nil, nil)
}

// zoneLines are the lines of the log u reads that tell what a creature met in the area of a spell.
func (c *cave) zoneLines(t *testing.T, u *user, spell string) []*playv1.CombatLogEntry {
	t.Helper()
	var out []*playv1.CombatLogEntry
	for _, r := range c.log(t, u, c.get(t, c.master)).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_EFFECT && en.GetEffect().GetSourceKey() == spell {
				out = append(out, en)
			}
		}
	}
	return out
}

// nextTurnOf ends turns, as the master, until the next turn of the combatant begins.
func (c *cave) nextTurnOf(t *testing.T, label string) {
	t.Helper()
	e := c.get(t, c.master)
	if e.GetCurrentCombatantId() == c.id(t, label) {
		if _, err := c.endTurn(t, c.master, e, true); err != nil {
			t.Fatalf("EndTurn() error = %v", err)
		}
	}
	c.passTo(t, c.get(t, c.master), label)
}

func (c *cave) moveZone(t *testing.T, u *user, zone string, col, row int32) (*playv1.MoveSpellZoneResponse, error) {
	t.Helper()
	res, err := u.combat.MoveSpellZone(t.Context(), connect.NewRequest(&playv1.MoveSpellZoneRequest{
		CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), ZoneId: zone, Origin: at(col, row), IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// Casting Spirit Guardians leaves a zone on the map: a sphere of 15 feet around the caster, owned by
// the caster, held by the concentration for ten minutes (100 rounds), that follows the caster. Nobody
// is hit by the cast itself: the damage waits for a creature to enter the area or start its turn in it.
func TestSpiritGuardiansLeavesAZoneAroundTheCasterAndHitsNobodyAtCast(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	c.zoneFight(t)
	before, _, _ := c.hp(t, "Ogro")
	c.castSpiritGuardians(t, 3)
	z := c.oneZone(t, c.master, spiritGuardiansKey)
	if z.GetOwnerCombatantId() != c.id(t, "Toren") || z.GetShape() != rulesv1.SpellAreaShape_SPELL_AREA_SHAPE_SPHERE || z.GetSizeFt() != 15 {
		t.Errorf("zone = %v, want a 15-foot sphere owned by Toren", z)
	}
	if !z.GetConcentration() || z.GetRoundsLeft() != 100 || z.GetMovement() != playv1.SpellZoneMovement_SPELL_ZONE_MOVEMENT_FOLLOWS_CASTER {
		t.Errorf("zone = %v, want concentration, 100 rounds, following the caster", z)
	}
	if z.GetVisibility() != playv1.SpellZoneVisibility_SPELL_ZONE_VISIBILITY_EVERYONE {
		t.Errorf("visibility = %v, want everyone", z.GetVisibility())
	}
	if !zoneHas(z, 5, 7) || !zoneHas(z, 8, 7) || zoneHas(z, 9, 7) {
		t.Errorf("squares = %v, want the radius of 3 squares around (5,7)", z.GetSquares())
	}
	if after, _, _ := c.hp(t, "Ogro"); after != before {
		t.Errorf("the ogre's hit points after the cast = %d, want %d: the cast hits nobody", after, before)
	}
	if got := zonesOf(c.get(t, c.ana), spiritGuardiansKey); len(got) != 1 {
		t.Errorf("a player reads %d zones, want the one of a caster they see", len(got))
	}
}

// A creature that starts its turn in the area makes the saving throw: a failure takes the whole
// 3d8 (the cast's slot), a pass half; the ogre is an NPC, so the damage lands at once.
func TestSpiritGuardiansHitsWhoStartsItsTurnInTheArea(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.castSpiritGuardians(t, 3)
	c.h.roller.queue(5, 4, 4, 4) // the ogre's Wisdom save (5 + 0 < 14), then 3d8 = 12
	c.passTo(t, e, "Ogro")
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-12 {
		t.Fatalf("the ogre's hit points = %d, want 47: it failed the save and took 3d8 = 12", cur)
	}
	lines := c.zoneLines(t, c.master, spiritGuardiansKey)
	if !slices.ContainsFunc(lines, func(en *playv1.CombatLogEntry) bool {
		return en.GetEffect().GetChange() == playv1.CombatLogEffectChange_COMBAT_LOG_EFFECT_CHANGE_SAVE_FAILED && slices.Contains(en.GetEffect().GetTargetLabels(), "Ogro")
	}) {
		t.Errorf("the master's log = %v, want a line for the ogre in the aura", lines)
	}
}

// A creature that passes the save takes half.
func TestSpiritGuardiansHalvesOnASavedThrow(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.castSpiritGuardians(t, 3)
	c.h.roller.queue(20, 4, 4, 5) // saves, then 3d8 = 13, half 6
	c.passTo(t, e, "Ogro")
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-6 {
		t.Fatalf("the ogre's hit points = %d, want 53: half of 13", cur)
	}
}

// A creature outside the area is not hit, and the damage grows a die for each slot level above the third.
func TestSpiritGuardiansLeavesTheOutsideAloneAndScalesWithTheSlot(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.castSpiritGuardians(t, 4)
	c.h.roller.queue(1, 1, 1, 1, 1) // the ogre fails: 4d8 = 4
	e = c.passTo(t, e, "Ogro")
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-4 {
		t.Errorf("the ogre's hit points = %d, want 55: 4d8 at the 4th level", cur)
	}
	c.mustEndTurn(t, c.master, e)
	if cur, _, _ := c.hp(t, "Goblin"); cur != 7 {
		t.Errorf("the goblin's hit points = %d, want 7: it is outside the aura", cur)
	}
}

// A creature that walks into the area on its turn is hit once; moving about inside the area, or out
// and in again, hits it no more that turn, and its next turn starts in the area: hit again.
func TestSpiritGuardiansHitsOncePerTurnWhoEnters(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.mustMove(t, c.master, "Ogro", 14, 7) // out of the area first
	c.castSpiritGuardians(t, 3)
	e = c.passTo(t, e, "Ogro")
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59 {
		t.Fatalf("the ogre's hit points at the start of its turn = %d, want 59: it began outside", cur)
	}
	c.h.roller.queue(5, 4, 4, 4)
	if _, err := c.move(t, c.master, "Ogro", 8, 7); err != nil {
		t.Fatalf("MoveCombatant() error = %v", err)
	}
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-12 {
		t.Fatalf("the ogre's hit points after walking in = %d, want 47", cur)
	}
	// Out and in again, and further in: no more damage, and nothing is rolled.
	for _, sq := range [][2]int32{{9, 7}, {8, 7}, {6, 7}} {
		if _, err := c.move(t, c.master, "Ogro", sq[0], sq[1]); err != nil {
			t.Fatalf("MoveCombatant(%v) error = %v", sq, err)
		}
	}
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-12 {
		t.Errorf("the ogre's hit points after moving about = %d, want 47: once on a turn", cur)
	}
	// The next round it starts its turn inside: hit again.
	c.h.roller.queue(5, 2, 2, 2)
	c.nextTurnOf(t, "Ogro")
	_ = e
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-12-6 {
		t.Errorf("the ogre's hit points on its next turn = %d, want 41", cur)
	}
}

// The aura goes with the caster: when Toren walks, the squares move with him, and a creature the
// new area covers is hit as one that entered.
func TestSpiritGuardiansFollowsTheCasterAndHitsWhoItReaches(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	c.zoneFight(t)
	c.mustMove(t, c.master, "Ogro", 11, 7) // outside the aura at (5,7), inside the one at (9,7)
	c.castSpiritGuardians(t, 3)
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59 {
		t.Fatalf("the ogre's hit points = %d, want 59", cur)
	}
	c.h.roller.queue(5, 4, 4, 4)
	c.mustMove(t, c.caio, "Toren", 9, 7)
	z := c.oneZone(t, c.master, spiritGuardiansKey)
	if z.GetOrigin().GetCol() != 9 || !zoneHas(z, 12, 7) || zoneHas(z, 5, 7) {
		t.Errorf("zone after the caster moved = origin %v squares %v, want it centered on (9,7)", z.GetOrigin(), z.GetSquares())
	}
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-12 {
		t.Errorf("the ogre's hit points = %d, want 47: the aura reached it", cur)
	}
}

// The zone ends with the caster's concentration: another concentration spell ends the aura.
func TestSpiritGuardiansEndsWithTheConcentration(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	c.zoneFight(t)
	c.castSpiritGuardians(t, 3)
	c.nextTurnOf(t, "Toren")
	c.mustCast(t, c.caio, c.get(t, c.master), "Toren", blessKey, slotOfLevel(1), c.at(t, "Toren"), noCastRoll)
	if zs := c.get(t, c.master).GetZones(); len(zs) != 0 {
		t.Errorf("zones after another concentration = %v, want none", zs)
	}
}

// The undo of the cast takes the zone away, with the concentration.
func TestSpiritGuardiansZoneGoesWithTheUndoOfTheCast(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	c.zoneFight(t)
	c.castSpiritGuardians(t, 3)
	c.undoLast(t)
	if zs := c.get(t, c.master).GetZones(); len(zs) != 0 {
		t.Errorf("zones after the undo = %v, want none", zs)
	}
}

// Moonbeam is a 5-foot-radius cylinder at a point; the caster moves it with an action on a later
// turn, up to 60 feet; a creature the beam reaches is hit as one that entered.
func TestMoonbeamIsMovedByItsCasterWithAnAction(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	c.zoneFight(t)
	c.mustCastArea(t, c.caio, "Toren", moonbeamKey, slotOfLevel(2), at(18, 7), nil)
	z := c.oneZone(t, c.master, moonbeamKey)
	if z.GetShape() != rulesv1.SpellAreaShape_SPELL_AREA_SHAPE_CYLINDER || z.GetSizeFt() != 5 || z.GetRoundsLeft() != 10 || z.GetMovement() != playv1.SpellZoneMovement_SPELL_ZONE_MOVEMENT_ACTION {
		t.Fatalf("zone = %v, want a 5-foot cylinder for 10 rounds moved by an action", z)
	}
	// The cast used the action: moving the beam on the same turn is refused.
	if _, err := c.moveZone(t, c.caio, z.GetId(), 8, 7); err == nil {
		t.Fatal("MoveSpellZone() on the casting turn succeeded, want ACTION_USED")
	}
	// Next round: one action moves it 50 feet to (8,7), over the ogre.
	c.nextTurnOf(t, "Toren")
	c.h.roller.queue(5, 5, 5)
	if _, err := c.moveZone(t, c.caio, z.GetId(), 8, 7); err != nil {
		t.Fatalf("MoveSpellZone() error = %v", err)
	}
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-10 {
		t.Errorf("the ogre's hit points = %d, want 49: the beam moved onto it (2d10 = 10)", cur)
	}
	if moved := c.oneZone(t, c.master, moonbeamKey); moved.GetOrigin().GetCol() != 8 || !zoneHas(moved, 7, 7) {
		t.Errorf("zone after the move = %v", moved)
	}
	// A second move on the same turn: the action is spent.
	if _, err := c.moveZone(t, c.caio, z.GetId(), 9, 7); err == nil {
		t.Error("a second MoveSpellZone() on the same turn succeeded, want ACTION_USED")
	}
	// Farther than 60 feet is refused for a player.
	c.nextTurnOf(t, "Toren")
	if _, err := c.moveZone(t, c.caio, z.GetId(), 22, 9); err == nil {
		t.Error("MoveSpellZone() beyond 60 feet succeeded, want it refused")
	}
}

// A Moonbeam lasts a minute: ten rounds, counted to the start of the caster's turn.
func TestMoonbeamEndsAfterTenRounds(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	c.zoneFight(t)
	c.mustCastArea(t, c.caio, "Toren", moonbeamKey, slotOfLevel(2), at(18, 7), nil)
	for range 9 {
		c.nextTurnOf(t, "Toren")
	}
	if zs := zonesOf(c.get(t, c.master), moonbeamKey); len(zs) != 1 || zs[0].GetRoundsLeft() != 1 {
		t.Fatalf("zones in round 10 = %v, want the beam with one round left", zs)
	}
	c.nextTurnOf(t, "Toren")
	if zs := zonesOf(c.get(t, c.master), moonbeamKey); len(zs) != 0 {
		t.Errorf("zones after ten rounds = %v, want none", zs)
	}
}

// Cloudkill drifts 10 feet away from the caster at the start of each of the caster's turns.
func TestCloudkillDriftsAwayFromTheCasterAtTheStartOfItsTurn(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.passTo(t, e, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", cloudkillKey, slotOfLevel(5), at(16, 7), nil)
	z := c.oneZone(t, c.master, cloudkillKey)
	if z.GetShape() != rulesv1.SpellAreaShape_SPELL_AREA_SHAPE_SPHERE || z.GetSizeFt() != 20 || z.GetMovement() != playv1.SpellZoneMovement_SPELL_ZONE_MOVEMENT_DRIFTS {
		t.Fatalf("zone = %v, want a 20-foot sphere that drifts", z)
	}
	c.nextTurnOf(t, "Pensantus")
	moved := c.oneZone(t, c.master, cloudkillKey)
	if moved.GetOrigin().GetCol() != 18 || moved.GetOrigin().GetRow() != 7 {
		t.Errorf("origin after one turn = %v, want (18,7): two squares away from Pensantus at (12,7)", moved.GetOrigin())
	}
}

// Web fills a 20-foot cube; a creature caught takes the web's effect as it always did, and the
// zone stays on the map for the concentration.
func TestWebLeavesAZoneAndKeepsItsRestrainingEffect(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.mustMove(t, c.master, "Ogro", 18, 3)
	c.mustMove(t, c.master, "Pensantus", 17, 7)
	c.passTo(t, e, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", webKey, slotOfLevel(2), at(18, 3), nil)
	z := c.oneZone(t, c.master, webKey)
	if z.GetShape() != rulesv1.SpellAreaShape_SPELL_AREA_SHAPE_CUBE || z.GetSizeFt() != 20 || !z.GetConcentration() {
		t.Fatalf("zone = %v, want a 20-foot cube held by concentration", z)
	}
	if len(z.GetSquares()) != 16 || !zoneHas(z, 18, 3) {
		t.Errorf("squares = %v, want 16 squares around (18,3)", z.GetSquares())
	}
	if card := cardOf(t, byLabel(t, c.get(t, c.master), "Ogro"), webKey); card == nil {
		t.Error("the ogre inside the web has no Web card: the cast keeps the effect it always put")
	}
}

// A creature pushed into the area (Thunderwave) enters it: the push runs the same check.
func TestAPushIntoTheAreaHitsAsAnEntry(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.mustMove(t, c.master, "Ogro", 11, 7)
	c.mustMove(t, c.caio, "Toren", 7, 7)
	c.castSpiritGuardians(t, 3) // covers columns 4 to 10 of row 7
	c.passTo(t, e, "Pensantus")
	c.h.roller.queue(1, 5, 4, 4, 4) // Thunderwave's Constitution save fails; the aura's Wisdom save fails and 3d8 = 12
	if _, err := c.castArea(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), nil, toward(-1, 0)); err != nil {
		t.Fatalf("CastSpell(Thunderwave) error = %v", err)
	}
	if got := byLabel(t, c.get(t, c.master), "Ogro"); got.GetCol() != 9 {
		t.Fatalf("the ogre stands at column %d, want 9: pushed 10 feet", got.GetCol())
	}
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-12 {
		t.Errorf("the ogre's hit points = %d, want 47: the push took it into the aura", cur)
	}
}

// The master's forced move (a teleport) into the area runs the same check.
func TestAForcedMoveIntoTheAreaHitsAsAnEntry(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	c.zoneFight(t)
	c.mustMove(t, c.master, "Ogro", 14, 7)
	c.castSpiritGuardians(t, 3)
	c.h.roller.queue(5, 4, 4, 4)
	c.mustMove(t, c.master, "Ogro", 7, 7)
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-12 {
		t.Errorf("the ogre's hit points = %d, want 47", cur)
	}
}

// A creature that was not in the web and walks in makes the Dexterity save of the web at once, and a
// failure restrains it with the same effect the web always put on who it caught at the cast.
func TestWebCatchesWhoComesInLaterWithADexteritySave(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.mustMove(t, c.master, "Goblin", 21, 6) // outside the cube of columns 17 to 20, rows 2 to 5
	c.mustMove(t, c.master, "Pensantus", 17, 7)
	c.passTo(t, e, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", webKey, slotOfLevel(2), at(18, 3), nil)
	c.passTo(t, c.get(t, c.master), "Goblin")
	c.h.roller.queue(5) // the Dexterity save of the goblin: 5 + 0 < 15
	if _, err := c.move(t, c.master, "Goblin", 20, 5); err != nil {
		t.Fatalf("MoveCombatant() error = %v", err)
	}
	if card := cardOf(t, byLabel(t, c.get(t, c.master), "Goblin"), "effect:web-restrained"); card == nil {
		t.Fatal("the goblin that walked into the web is not restrained")
	}
}

// A creature that passes the save is not restrained.
func TestWebLetsWhoPassesTheSaveWalkThrough(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.mustMove(t, c.master, "Goblin", 21, 6)
	c.mustMove(t, c.master, "Pensantus", 17, 7)
	c.passTo(t, e, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", webKey, slotOfLevel(2), at(18, 3), nil)
	c.passTo(t, c.get(t, c.master), "Goblin")
	c.h.roller.queue(20)
	c.mustMove(t, c.master, "Goblin", 20, 5)
	if card := cardOf(t, byLabel(t, c.get(t, c.master), "Goblin"), "effect:web-restrained"); card != nil {
		t.Errorf("the goblin that passed the save is restrained: %v", card)
	}
}

// A player's character caught by a zone does not lose hit points by itself: the damage waits for the
// master, as every damage on a character does.
func TestAZoneHitsAPlayersCharacterAsADamageThatWaitsForTheMaster(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.mustMove(t, c.master, "Toren", 16, 7)
	c.passTo(t, e, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", cloudkillKey, slotOfLevel(5), at(15, 7), nil)
	c.h.roller.queue(5, 4, 4, 4, 4, 4) // Toren's Constitution save, then 5d8 = 20
	c.nextTurnOf(t, "Toren")
	var status string
	var amount int32
	if err := c.h.pool.QueryRow(t.Context(), `SELECT status, amount FROM pending_damages WHERE target_id = $1 AND effect_source_key = $2`, c.id(t, "Toren"), cloudkillKey).Scan(&status, &amount); err != nil {
		t.Fatalf("read the damage of the cloud: %v", err)
	}
	if status != "rolled" || amount != 20 {
		t.Errorf("the pending damage = %s for %d, want rolled for 20: it waits for the master", status, amount)
	}
}

// Only the caster's player moves the beam, and the master any; a zone that is not there is not found.
func TestMoveSpellZoneIsTheCastersAndTheMasters(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	c.zoneFight(t)
	c.mustCastArea(t, c.caio, "Toren", moonbeamKey, slotOfLevel(2), at(18, 7), nil)
	z := c.oneZone(t, c.master, moonbeamKey)
	if _, err := c.moveZone(t, c.ana, z.GetId(), 18, 8); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("another player moved the beam: error = %v, want permission_denied", err)
	}
	if _, err := c.moveZone(t, c.master, "not-a-uuid", 18, 8); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a zone that does not exist: error = %v, want not_found", err)
	}
	// The master puts it right at once: no action, no distance, and the caster's action stays as it was.
	if _, err := c.moveZone(t, c.master, z.GetId(), 21, 9); err != nil {
		t.Fatalf("MoveSpellZone() as the master error = %v", err)
	}
	if got := c.oneZone(t, c.master, moonbeamKey); got.GetOrigin().GetCol() != 21 || got.GetOrigin().GetRow() != 9 {
		t.Errorf("zone = %v, want it at (21,9)", got)
	}
	if _, err := c.moveZone(t, c.master, z.GetId(), 30, 30); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a square off the map: error = %v, want invalid_argument", err)
	}
}

// A retry of the move with the same key answers the same and does not spend a second action.
func TestMoveSpellZoneRetryWithTheSameKeyChangesNothingTwice(t *testing.T) {
	t.Parallel()
	c := druidCave(t)
	c.zoneFight(t)
	c.mustCastArea(t, c.caio, "Toren", moonbeamKey, slotOfLevel(2), at(18, 7), nil)
	z := c.oneZone(t, c.master, moonbeamKey)
	c.nextTurnOf(t, "Toren")
	req := &playv1.MoveSpellZoneRequest{CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), ZoneId: z.GetId(), Origin: at(14, 7), IdempotencyKey: newKey()}
	for range 2 {
		if _, err := c.caio.combat.MoveSpellZone(t.Context(), connect.NewRequest(req)); err != nil {
			t.Fatalf("MoveSpellZone() error = %v", err)
		}
	}
	if got := c.oneZone(t, c.master, moonbeamKey); got.GetOrigin().GetCol() != 14 {
		t.Errorf("zone = %v, want it at column 14", got)
	}
}
