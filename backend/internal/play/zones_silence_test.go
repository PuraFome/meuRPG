package play

import (
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Silence (SRD 5.1) keeps a sphere of 20 feet of radius on the map. A creature entirely inside it is immune to thunder
// damage and deafened, and a caster there cannot cast a spell with a verbal component. Every combatant occupies one
// square, so "entirely inside" is the creature's square in the sphere. These tests need the database
// (MEURPG_TEST_DATABASE_URL). The cave is the one of zones_test.go: Toren the cleric at (5,7), Pensantus the wizard
// at (12,7), Brisa at (3,7), the ogre at (7,7) and the goblin far to the east.

const (
	silenceKey         = "spell:silence"
	hypnoticPatternKey = "spell:hypnotic-pattern"
)

func silenceCave(t *testing.T) *cave {
	t.Helper()
	return newCaveOf(t, 9, []string{thunderwave, burningHands, hypnoticPatternKey, silenceKey}, func(a *armed) *charactersv1.Character {
		return a.caio.caster(t, a.campaignID, "Toren", "class:cleric", "race:human", 7, abilities(14, 12, 14, 16, 10),
			[]string{maceKey}, []string{sacredFlame}, nil, []string{silenceKey, blessKey})
	})
}

// castSilence has Toren, on turn, put the sphere at the square.
func (c *cave) castSilence(t *testing.T, col, row int32) {
	t.Helper()
	c.passTo(t, c.zoneFight(t), "Toren")
	c.mustCastArea(t, c.caio, "Toren", silenceKey, slotOfLevel(2), at(col, row), nil)
}

func TestSilenceIsATwentyFootSphereThatAsksNobodyAndHitsNobody(t *testing.T) {
	t.Parallel()
	c := silenceCave(t)
	c.castSilence(t, 12, 7)
	z := c.oneZone(t, c.master, silenceKey)
	if far := farthest(z, 12, 7); far != 4 {
		t.Errorf("reach = %d squares, want 4 (20 ft)", far)
	}
	if !z.GetConcentration() || z.GetMovement() != playv1.SpellZoneMovement_SPELL_ZONE_MOVEMENT_FIXED {
		t.Errorf("zone = %v, want a fixed zone held by the concentration", z)
	}
	if lines := c.zoneLines(t, c.master, silenceKey); len(lines) != 0 {
		t.Errorf("lines of the zone = %v, want none: it asks nobody", lines)
	}
	if clouds, dark := c.combatPositions(t); len(clouds) != 0 || len(dark) != 0 {
		t.Errorf("combat positions: %d cloud squares and %d dark, want none: Silence changes nothing seen", len(clouds), len(dark))
	}
}

// A caster inside the sphere cannot cast a spell with a verbal component (Thunderwave), and the refusal costs
// nothing; a spell with somatic and material components only (Hypnotic Pattern) still works.
func TestSilenceRefusesAVerbalSpellToACasterInside(t *testing.T) {
	t.Parallel()
	c := silenceCave(t)
	c.castSilence(t, 12, 7)
	c.passTo(t, c.get(t, c.master), "Pensantus")
	_, err := c.castArea(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), nil, toward(1, 0))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), "verbal") {
		t.Fatalf("Thunderwave inside the sphere: error = %v, want failed_precondition for the verbal component", err)
	}
	if strings.Contains(err.Error(), "Silence") {
		t.Errorf("the refusal %q names the spell", err)
	}
	if byLabel(t, c.get(t, c.master), "Pensantus").GetActionUsed() {
		t.Error("the refused cast spent the action")
	}
	if _, err := c.castAt(t, c.ana, "Pensantus", hypnoticPatternKey, 3, nil, "Ogro"); err != nil {
		t.Errorf("Hypnotic Pattern (somatic and material) inside the sphere: error = %v, want it cast", err)
	}
}

// The last square of the sphere is inside; the next one is outside.
func TestSilenceLetsACasterOutsideCastVerbalSpells(t *testing.T) {
	t.Parallel()
	c := silenceCave(t)
	c.castSilence(t, 12, 7)
	c.passTo(t, c.get(t, c.master), "Pensantus")
	c.mustMove(t, c.master, "Pensantus", 16, 7)
	if _, err := c.castArea(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), nil, toward(1, 0)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("Thunderwave from the last square inside: error = %v, want failed_precondition", err)
	}
	c.mustMove(t, c.master, "Pensantus", 17, 7)
	if _, err := c.castArea(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), nil, toward(1, 0)); err != nil {
		t.Errorf("Thunderwave from the first square outside: error = %v, want it cast", err)
	}
}

// thunderOnTheOgre has Toren put the sphere at the square, then Pensantus, outside it, cast Thunderwave at the ogre
// (the push of the wave carries it 2 squares west, from (11,7) to (9,7), before the damage is rolled). It answers
// the ogre's hit points.
func (c *cave) thunderOnTheOgre(t *testing.T, col, row int32) int32 {
	t.Helper()
	c.castSilence(t, col, row)
	c.passTo(t, c.get(t, c.master), "Pensantus")
	c.mustMove(t, c.master, "Pensantus", 14, 7)
	c.mustMove(t, c.master, "Ogro", 11, 7)
	if hits := c.mustPreview(t, c.ana, "Pensantus", thunderwave, toward(-1, 0)); !slices.Contains(hits, "Ogro") {
		t.Fatalf("the cube holds %v, want the ogre", hits)
	}
	c.h.roller.queue(10)
	res := c.mustCastArea(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), nil, toward(-1, 0))
	c.h.roller.queue(6, 6)
	c.mustDamage(t, c.ana, c.get(t, c.ana), res.GetCast().GetPendingDamages()[0].GetId(), inAppDamage)
	cur, _, _ := c.hp(t, "Ogro")
	return cur
}

// The ogre inside the sphere takes no thunder damage (Constitution save failed, 12 rolled).
func TestSilenceMakesWhoIsInsideImmuneToThunder(t *testing.T) {
	t.Parallel()
	c := silenceCave(t)
	if cur := c.thunderOnTheOgre(t, 7, 7); cur != 59 {
		t.Errorf("the ogre inside has %d hit points, want 59: immune to thunder", cur)
	}
}

// The same wave on an ogre outside the sphere takes it in full.
func TestSilenceLeavesThunderOutsideAlone(t *testing.T) {
	t.Parallel()
	c := silenceCave(t)
	if cur := c.thunderOnTheOgre(t, 7, 12); cur != 59-12 {
		t.Errorf("the ogre outside has %d hit points, want 47: 12 of thunder", cur)
	}
}

// Another damage type is not changed by the sphere.
func TestSilenceLeavesOtherDamageAlone(t *testing.T) {
	t.Parallel()
	c := silenceCave(t)
	c.castSilence(t, 7, 7)
	c.passTo(t, c.get(t, c.master), "Pensantus")
	c.mustMove(t, c.master, "Pensantus", 14, 7)
	c.mustMove(t, c.master, "Ogro", 11, 7)
	if hits := c.mustPreview(t, c.ana, "Pensantus", burningHands, toward(-1, 0)); !slices.Contains(hits, "Ogro") {
		t.Fatalf("the cone holds %v, want the ogre", hits)
	}
	c.h.roller.queue(10)
	res := c.mustCastArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(-1, 0))
	c.h.roller.queue(3, 3, 3)
	c.mustDamage(t, c.ana, c.get(t, c.ana), res.GetCast().GetPendingDamages()[0].GetId(), inAppDamage)
	if cur, _, _ := c.hp(t, "Ogro"); cur >= 59 {
		t.Errorf("the ogre inside has %d hit points, want it hurt by the fire", cur)
	}
}

// mustPreview are the labels the area of the spell holds, sorted.
func (c *cave) mustPreview(t *testing.T, u *user, caster, spell string, dir *playv1.SpellDirection) []string {
	t.Helper()
	res, err := c.preview(t, u, caster, spell, slotOfLevel(1), nil, dir)
	if err != nil {
		t.Fatalf("PreviewSpellArea(%s) error = %v", spell, err)
	}
	return labelsOfTargets(res)
}

// The conditions the master and the owner read: deafened while the square is inside, and gone when the creature
// steps out or the zone ends; a deafness from another source stays.
func TestSilenceDeafensWhoIsInsideAndOnlyWhileInside(t *testing.T) {
	t.Parallel()
	c := silenceCave(t)
	c.castSilence(t, 7, 7)
	e := c.get(t, c.master)
	for label, want := range map[string]bool{"Ogro": true, "Toren": true, "Brisa": true, "Pensantus": false, "Goblin": false} {
		if got := hasCondition(e, t, label, conditionDeafened); got != want {
			t.Errorf("%s deafened = %v, want %v", label, got, want)
		}
	}
	if !hasCondition(c.get(t, c.caio), t, "Toren", conditionDeafened) {
		t.Error("Toren's player does not read his deafened condition")
	}
	if hasCondition(c.get(t, c.ana), t, "Ogro", conditionDeafened) {
		t.Error("another player reads the ogre deafened: it has no outward sign")
	}
	c.mustMove(t, c.master, "Ogro", 12, 7)
	if hasCondition(c.get(t, c.master), t, "Ogro", conditionDeafened) {
		t.Error("the ogre outside is still deafened")
	}
	c.mustMove(t, c.master, "Ogro", 7, 8)
	if !hasCondition(c.get(t, c.master), t, "Ogro", conditionDeafened) {
		t.Error("the ogre back inside is not deafened")
	}
	c.undoLast(t)
	if hasCondition(c.get(t, c.master), t, "Ogro", conditionDeafened) {
		t.Error("the ogre is deafened after the zone went with the undo of the cast")
	}
}

func TestSilenceKeepsADeafnessFromAnotherSource(t *testing.T) {
	t.Parallel()
	c := silenceCave(t)
	c.castSilence(t, 7, 7)
	c.setConditions(t, c.get(t, c.master), "Goblin", conditionDeafened)
	c.mustMove(t, c.master, "Goblin", 7, 8)
	c.mustMove(t, c.master, "Goblin", 20, 7)
	if !hasCondition(c.get(t, c.master), t, "Goblin", conditionDeafened) {
		t.Error("the goblin lost the deafness the master gave it when it left the sphere")
	}
	c.setConditions(t, c.get(t, c.master), "Ogro") // nothing set by hand: the ogre keeps only the sphere's deafness
	if !hasCondition(c.get(t, c.master), t, "Ogro", conditionDeafened) {
		t.Error("the ogre inside lost its deafness when the master cleared its conditions")
	}
}

// Two spheres: the creature stays deafened while one of them still covers it, though the first one ended.
func TestSilenceStaysWhileAnotherSphereCoversTheCreature(t *testing.T) {
	t.Parallel()
	c := silenceCave(t)
	c.castSilence(t, 7, 7)
	c.passTo(t, c.get(t, c.master), "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", silenceKey, slotOfLevel(2), at(9, 7), nil)
	c.mustMove(t, c.master, "Toren", 2, 7) // outside both spheres
	c.nextTurnOf(t, "Toren")
	if _, err := c.castAt(t, c.caio, "Toren", blessKey, 1, nil, "Toren"); err != nil { // the concentration moves on: the first sphere ends
		t.Fatalf("Bless from outside the spheres error = %v", err)
	}
	zs := zonesOf(c.get(t, c.master), silenceKey)
	if len(zs) != 1 || zs[0].GetOwnerLabel() != "Pensantus" {
		t.Fatalf("zones = %v, want only Pensantus's sphere", zs)
	}
	if !hasCondition(c.get(t, c.master), t, "Ogro", conditionDeafened) {
		t.Error("the ogre lost its deafness while the second sphere still covers it")
	}
	c.mustMove(t, c.master, "Ogro", 15, 8)
	if hasCondition(c.get(t, c.master), t, "Ogro", conditionDeafened) {
		t.Error("the ogre outside both spheres is still deafened")
	}
}

// A hidden caster's Silence is the master's alone, and the deafness it gives names neither the spell nor the caster.
func TestAHiddenCastersSilenceNamesNeitherTheSpellNorTheCaster(t *testing.T) {
	t.Parallel()
	c := newCaveWith(t, 5, []string{thunderwave})
	c.druidFightWith(t, []string{silenceKey})
	c.mustCastArea(t, c.master, "Druida", silenceKey, slotOfLevel(2), at(3, 7), nil)
	z := c.oneZone(t, c.master, silenceKey)
	if z.GetVisibility() != playv1.SpellZoneVisibility_SPELL_ZONE_VISIBILITY_MASTER_ONLY {
		t.Errorf("visibility = %v, want MASTER_ONLY: the caster is hidden", z.GetVisibility())
	}
	for who, u := range map[string]*user{"Toren's player": c.caio, "Pensantus's player": c.ana, "Brisa's player": c.bia} {
		e := c.get(t, u)
		if zs := e.GetZones(); len(zs) != 0 {
			t.Errorf("%s reads zones %v, want none: the caster is hidden", who, zs)
		}
		text := asJSONAny(t, e) + asJSONAny(t, c.log(t, u, e))
		for _, w := range []string{silenceKey, "Druida", "Silence", "Silêncio", z.GetId()} {
			if strings.Contains(text, w) {
				t.Errorf("%s's copy has %q of the hidden caster's Silence", who, w)
			}
		}
	}
	if !hasCondition(c.get(t, c.caio), t, "Toren", conditionDeafened) {
		t.Error("Toren, inside the sphere, does not read his own deafness")
	}
	if !slices.Contains(byLabel(t, c.get(t, c.master), "Brisa").GetConditions(), conditionDeafened) {
		t.Error("the master does not read Brisa's deafness")
	}
}

// Revivify has a verbal component: Ilaria inside a Silence cannot cast it, and the refusal costs nothing.
func TestSilenceRefusesRevivifyToACasterInside(t *testing.T) {
	t.Parallel()
	a, e := newMortuary(t)
	a.dies(t, e, "Toren", a.toren)
	_, err := playdb.New(a.h.pool).InsertSpellZone(t.Context(), playdb.InsertSpellZoneParams{
		EncounterID: e.GetId(), OwnerID: a.id(t, "Goblin"), SpellKey: silenceKey, CastID: newKey(), SlotLevel: 2,
		OriginCol: 3, OriginRow: 4, Squares: squaresToJSON([]grid.Square{{Col: 3, Row: 4}}), Concentration: true, StartedRound: 1, CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("InsertSpellZone() error = %v", err)
	}
	_, err = a.castRevivify(t, a.ana, e, "Ilaria", "Toren")
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("Revivify inside the sphere: error = %v, want failed_precondition", err)
	}
	if strings.Contains(err.Error(), "Silence") {
		t.Errorf("the refusal %q names the spell", err)
	}
	if v := a.vitals(t, a.pens); usedSlots(v, 3) != 0 {
		t.Errorf("slots used after the refusal = %d, want 0", usedSlots(v, 3))
	}
}

// A reactor inside a Silence is offered no reaction spell with a verbal component; outside it is.
func TestSilenceTakesTheVerbalReactionsAway(t *testing.T) {
	t.Parallel()
	st := link.ReactionStats{Spells: []string{spellShield, spellHellishRebuke, spellFeatherFall}, SlotsTotal: [9]int{2}}
	for _, spell := range []string{spellShield, spellHellishRebuke, spellFeatherFall} {
		if got := (kit{who: playdb.Combatant{Kind: kindNPC}, st: st}).slotsFor(spell); len(got) == 0 {
			t.Fatalf("slotsFor(%s) outside a Silence = none, want the 1st level slots", spell)
		}
		if got := (kit{who: playdb.Combatant{Kind: kindNPC}, st: st, muted: true}).slotsFor(spell); len(got) != 0 {
			t.Errorf("slotsFor(%s) inside a Silence = %v, want none", spell, got)
		}
	}
}

// shatterZoneKey is a fixture: Shatter given the row of a zone that deals its thunder damage to a creature that comes in,
// to reach the damage of a zone (afterResistance) with a type Silence changes. Registered once, before any test runs.
const shatterZoneKey = "spell:shatter"

func init() {
	zoneDefs[shatterZoneKey] = zoneDef{
		kind: zoneDamage, shape: grid.ShapeSphere, sizeFt: 10, save: "con", onEnter: true,
		placement: playv1.AreaPlacement_AREA_PLACEMENT_POINT, movement: playv1.SpellZoneMovement_SPELL_ZONE_MOVEMENT_FIXED,
	}
}

// thunderZoneHitsTheOgre puts a thunder zone at (16,7), with a Silence over it (centered one square east, so the caster stays out of it) or not, walks the ogre in after a failed save
// and returns its hit points.
func thunderZoneHitsTheOgre(t *testing.T, silenced bool) int {
	t.Helper()
	c := newCaveOf(t, 9, []string{shatterZoneKey, silenceKey}, func(a *armed) *charactersv1.Character {
		return a.caio.caster(t, a.campaignID, "Toren", "class:cleric", "race:human", 7, abilities(14, 12, 14, 16, 10),
			[]string{maceKey}, []string{sacredFlame}, nil, []string{silenceKey, blessKey})
	})
	if silenced {
		c.castSilence(t, 17, 7) // it starts the fight
	} else {
		c.zoneFight(t)
	}
	c.passTo(t, c.get(t, c.master), "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", shatterZoneKey, slotOfLevel(2), at(16, 7), nil)
	c.h.roller.queue(1, 5, 5, 5) // the Constitution save fails; 3d8 = 15
	c.mustMove(t, c.master, "Ogro", 16, 7)
	cur, _, _ := c.hp(t, "Ogro")
	return int(cur)
}

// The damage of a zone goes through afterResistance: a creature inside a Silence takes none of a zone's thunder.
func TestSilenceMakesWhoIsInsideImmuneToTheThunderOfAZone(t *testing.T) {
	t.Parallel()
	if got := thunderZoneHitsTheOgre(t, false); got != 59-15 {
		t.Fatalf("the ogre outside a Silence has %d hit points, want 44: 15 of thunder from the zone", got)
	}
	if got := thunderZoneHitsTheOgre(t, true); got != 59 {
		t.Errorf("the ogre inside a Silence has %d hit points, want 59: immune to thunder", got)
	}
}
