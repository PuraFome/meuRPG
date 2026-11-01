package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Wall of Fire (SRD 5.1) lays a wall of squares from a point. The geometry tests are pure; the cast tests need the
// database (MEURPG_TEST_DATABASE_URL) and use the cave of zones_test.go: Toren (5,7), Pensantus the wizard of level 9
// (12,7), Brisa (3,7), the ogre (7,7) and the goblin (20,7).

const wallOfFireKey = "spell:wall-of-fire"

func wsq(col, row int) grid.Square { return grid.Square{Col: col, Row: row} }

func setOf(squares []grid.Square) map[grid.Square]bool {
	out := map[grid.Square]bool{}
	for _, s := range squares {
		out[s] = true
	}
	return out
}

func TestWallStraightSquaresFollowTheMapsDistance(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		dir    grid.Direction
		length int
		want   int
	}{
		{"orthogonal 60 ft", grid.Direction{Dx: 1}, 60, 12},
		// The map rounds a distance down, so the diagonal wall of 60 ft is 9 squares: fewer than the 12 of a row.
		{"diagonal 60 ft", grid.Direction{Dx: 1, Dy: 1}, 60, 9},
		{"25 ft", grid.Direction{Dy: -1}, 25, 5},
		{"5 ft", grid.Direction{Dx: -1}, 5, 1},
	} {
		w := wallSpec{Dir: tt.dir, LengthFt: tt.length, Side: playv1.WallSide_WALL_SIDE_LEFT}
		got := w.squares(wsq(30, 30))
		if len(got) != tt.want {
			t.Errorf("%s: %d squares, want %d", tt.name, len(got), tt.want)
		}
		if got[0] != wsq(30, 30) {
			t.Errorf("%s: first square %v, want the point of origin", tt.name, got[0])
		}
	}
}

func TestWallRingSquares(t *testing.T) {
	t.Parallel()
	origin := wsq(10, 10)
	for _, tt := range []struct {
		diameter, want, atFt int
	}{{20, 16, 10}, {10, 8, 5}, {5, 8, 5}} {
		w := wallSpec{Ring: true, LengthFt: tt.diameter, Side: playv1.WallSide_WALL_SIDE_INNER}
		got := w.squares(origin)
		if len(got) != tt.want {
			t.Errorf("diameter %d: %d squares, want %d", tt.diameter, len(got), tt.want)
		}
		for _, s := range got {
			if grid.RangeFt(origin, s) != tt.atFt {
				t.Errorf("diameter %d: %v is %d ft out, want %d", tt.diameter, s, grid.RangeFt(origin, s), tt.atFt)
			}
		}
	}
}

func TestWallIsCutByTheTerrain(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 20, Rows: 20}
	terrain := grid.Terrain{Grid: g, Walls: grid.NewLayer(g)}
	terrain.Walls.Set(13, 5, true)
	w := wallSpec{Dir: grid.Direction{Dx: 1}, LengthFt: 60, Side: playv1.WallSide_WALL_SIDE_LEFT}
	got := w.laid(terrain, wsq(10, 5))
	if want := []grid.Square{wsq(10, 5), wsq(11, 5), wsq(12, 5)}; !slices.Equal(got, want) {
		t.Errorf("laid = %v, want %v: a wall of the map ends the wall", got, want)
	}
}

func TestReadWallRefusesWhatIsMalformed(t *testing.T) {
	t.Parallel()
	east := &playv1.SpellDirection{Dx: 1}
	left, inner := playv1.WallSide_WALL_SIDE_LEFT, playv1.WallSide_WALL_SIDE_INNER
	for name, w := range map[string]*playv1.SpellWall{
		"straight without direction": {Side: left},
		"ring with direction":        {Ring: true, Direction: east, Side: inner},
		"straight with a ring side":  {Direction: east, Side: inner},
		"ring with a line side":      {Ring: true, Side: left},
		"unspecified side":           {Direction: east},
		"length not a multiple of 5": {Direction: east, Side: left, LengthFt: 22},
		"straight too long":          {Direction: east, Side: left, LengthFt: 65},
		"ring too wide":              {Ring: true, Side: inner, LengthFt: 25},
		"negative length":            {Direction: east, Side: left, LengthFt: -5},
		"not one of the eight":       {Direction: &playv1.SpellDirection{Dx: 2}, Side: left},
		"no way at all":              {Direction: &playv1.SpellDirection{}, Side: left},
	} {
		if _, err := readWall(w); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: error = %v, want invalid_argument", name, err)
		}
	}
	got, err := readWall(&playv1.SpellWall{Direction: east, Side: left})
	if err != nil || got.LengthFt != 60 {
		t.Errorf("full size: %v, %v, want 60 ft", got, err)
	}
	got, err = readWall(&playv1.SpellWall{Ring: true, Side: inner})
	if err != nil || got.LengthFt != 20 {
		t.Errorf("full ring: %v, %v, want 20 ft", got, err)
	}
}

func wallReachOf(w wallSpec, origin grid.Square) map[grid.Square]bool {
	return w.reach(origin, w.squares(origin))
}

// For a wall that runs east the left is north (rows grow downward), within 10 ft, and the wall itself hurts from both.
func TestWallReachOfAStraightWallIsTheDamagingSide(t *testing.T) {
	t.Parallel()
	origin := wsq(10, 10)
	left := wallReachOf(wallSpec{Dir: grid.Direction{Dx: 1}, LengthFt: 15, Side: playv1.WallSide_WALL_SIDE_LEFT}, origin)
	right := wallReachOf(wallSpec{Dir: grid.Direction{Dx: 1}, LengthFt: 15, Side: playv1.WallSide_WALL_SIDE_RIGHT}, origin)
	for _, s := range []grid.Square{wsq(10, 10), wsq(11, 10), wsq(12, 10)} {
		if !left[s] || !right[s] {
			t.Errorf("%v is a square of the wall: it hurts from both sides", s)
		}
	}
	for _, c := range []struct {
		s           grid.Square
		left, right bool
	}{
		{wsq(11, 9), true, false}, {wsq(11, 8), true, false}, {wsq(11, 7), false, false},
		{wsq(11, 11), false, true}, {wsq(11, 12), false, true}, {wsq(11, 13), false, false},
		{wsq(13, 9), true, false}, {wsq(14, 10), false, false}, {wsq(9, 10), false, false},
	} {
		if left[c.s] != c.left || right[c.s] != c.right {
			t.Errorf("%v: left %v right %v, want %v and %v", c.s, left[c.s], right[c.s], c.left, c.right)
		}
	}
	north := wallReachOf(wallSpec{Dir: grid.Direction{Dy: -1}, LengthFt: 15, Side: playv1.WallSide_WALL_SIDE_LEFT}, origin)
	if !north[wsq(9, 9)] || north[wsq(11, 9)] {
		t.Error("walking north, the left is west")
	}
}

func TestWallReachOfARingIsInsideOrOutside(t *testing.T) {
	t.Parallel()
	origin := wsq(10, 10)
	inner := wallReachOf(wallSpec{Ring: true, LengthFt: 20, Side: playv1.WallSide_WALL_SIDE_INNER}, origin)
	outer := wallReachOf(wallSpec{Ring: true, LengthFt: 20, Side: playv1.WallSide_WALL_SIDE_OUTER}, origin)
	for _, s := range []grid.Square{wsq(12, 10), wsq(10, 8), wsq(12, 12)} {
		if !inner[s] || !outer[s] {
			t.Errorf("%v is a square of the ring: it hurts from both sides", s)
		}
	}
	for _, c := range []struct {
		s            grid.Square
		inner, outer bool
	}{
		{wsq(10, 10), true, false}, {wsq(11, 10), true, false}, {wsq(11, 11), true, false},
		{wsq(13, 10), false, true}, {wsq(14, 10), false, true}, {wsq(15, 10), false, false},
	} {
		if inner[c.s] != c.inner || outer[c.s] != c.outer {
			t.Errorf("%v: inner %v outer %v, want %v and %v", c.s, inner[c.s], outer[c.s], c.inner, c.outer)
		}
	}
}

// wallCave is the zone cave with Pensantus (wizard 9) knowing Wall of Fire.
func wallCave(t *testing.T) *cave {
	t.Helper()
	return newCaveOf(t, 9, []string{wallOfFireKey, cloudkillKey}, func(a *armed) *charactersv1.Character {
		return a.caio.caster(t, a.campaignID, "Toren", "class:cleric", "race:human", 7, abilities(14, 12, 14, 16, 10),
			[]string{maceKey}, []string{sacredFlame}, nil, []string{blessKey})
	})
}

// northWall is the wall from (7,8) north: the ogre at (7,7) is in it and (7,6) is a wall of the map, so it is 2 squares.
func northWall(side playv1.WallSide) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) {
		r.Wall = &playv1.SpellWall{Direction: toward(0, -1), Side: side}
	}
}

func (c *cave) castWall(t *testing.T, slot int32, edit func(*playv1.CastSpellRequest)) (*playv1.CastSpellResponse, error) {
	t.Helper()
	return c.castArea(t, c.ana, "Pensantus", wallOfFireKey, slotOfLevel(slot), at(7, 8), nil, edit)
}

func (c *cave) wallFight(t *testing.T) {
	t.Helper()
	c.passTo(t, c.zoneFight(t), "Pensantus")
}

func TestWallOfFireLaysTheWallAndTheCreatureInsideSaves(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		roll int
		want int32
	}{{"failed", 5, 59 - 25}, {"passed", 20, 59 - 12}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := wallCave(t)
			c.wallFight(t)
			c.h.roller.queue(tt.roll, 5, 5, 5, 5, 5) // the ogre's Dexterity save, then 5d8 = 25
			if _, err := c.castWall(t, 4, northWall(playv1.WallSide_WALL_SIDE_LEFT)); err != nil {
				t.Fatalf("CastSpell() error = %v", err)
			}
			z := c.oneZone(t, c.master, wallOfFireKey)
			if len(z.GetSquares()) != 2 || !zoneHas(z, 7, 8) || !zoneHas(z, 7, 7) {
				t.Errorf("squares = %v, want (7,8) and (7,7): the wall of the map ends it", z.GetSquares())
			}
			if !z.GetConcentration() || z.GetRoundsLeft() != 10 || z.GetSaveAbility() != "dex" {
				t.Errorf("zone = %v, want concentration, 10 rounds, a Dexterity save", z)
			}
			if cur, _, _ := c.hp(t, "Ogro"); cur != tt.want {
				t.Errorf("the ogre has %d hit points, want %d", cur, tt.want)
			}
			if cur, _, _ := c.hp(t, "Goblin"); cur != 7 {
				t.Errorf("the goblin has %d hit points, want 7: it is outside the wall", cur)
			}
		})
	}
}

func TestWallOfFireAtTheFifthLevelRollsSixDice(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	c.h.roller.queue(5, 1, 2, 3, 4, 5, 6) // the save fails, then 6d8 = 21
	if _, err := c.castWall(t, 5, northWall(playv1.WallSide_WALL_SIDE_RIGHT)); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if cur, _, _ := c.hp(t, "Ogro"); cur != 59-21 {
		t.Errorf("the ogre has %d hit points, want 38: 6d8 = 21", cur)
	}
}

func TestWallOfFireWithoutAWallIsRefusedAndCostsNothing(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	_, err := c.castWall(t, 4, func(*playv1.CastSpellRequest) {})
	wantBlockedBy(t, "CastSpell", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WALL_PLACEMENT_REQUIRED)
	_, err = c.castArea(t, c.ana, "Pensantus", wallOfFireKey, slotOfLevel(4), nil, nil)
	wantBlockedBy(t, "CastSpell without a point", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WALL_PLACEMENT_REQUIRED)
	if byLabel(t, c.get(t, c.master), "Pensantus").GetActionUsed() {
		t.Error("the refused cast spent the action")
	}
	if used := usedSlots(c.vitals(t, c.pens), 4); used != 0 {
		t.Errorf("slots used = %d, want 0", used)
	}
	if zs := c.get(t, c.master).GetZones(); len(zs) != 0 {
		t.Errorf("zones = %v, want none", zs)
	}
}

func TestWallOnAnotherSpellIsRefused(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	_, err := c.castArea(t, c.ana, "Pensantus", cloudkillKey, slotOfLevel(5), at(7, 8), nil, northWall(playv1.WallSide_WALL_SIDE_LEFT))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("error = %v, want invalid_argument", err)
	}
}

func TestAStraightWallWithoutADirectionIsRefused(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	_, err := c.castWall(t, 4, func(r *playv1.CastSpellRequest) { r.Wall = &playv1.SpellWall{Side: playv1.WallSide_WALL_SIDE_LEFT} })
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("error = %v, want invalid_argument", err)
	}
}

func TestAWallWithoutAPointOfOriginIsRefused(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	_, err := c.castArea(t, c.ana, "Pensantus", wallOfFireKey, slotOfLevel(4), nil, nil, northWall(playv1.WallSide_WALL_SIDE_LEFT))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("error = %v, want invalid_argument", err)
	}
	if zs := c.get(t, c.master).GetZones(); len(zs) != 0 {
		t.Errorf("zones = %v, want none", zs)
	}
}

func TestAWallInACombatWithoutAMapIsRefused(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	c.execSQL(t, `UPDATE encounters SET status = 'active', mode = 'theatre', grid_columns = 0, grid_rows = 0, map_id = NULL, map_point_id = NULL`)
	_, err := c.castWall(t, 4, northWall(playv1.WallSide_WALL_SIDE_LEFT))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("error = %v, want invalid_argument", err)
	}
}

func TestAPlayersWallBeyondTheRangeIsRefusedAndTheMastersIsNot(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	// The map is 24 squares wide, so the encounter is widened to put a point past the 120 ft of the spell.
	c.execSQL(t, `UPDATE encounters SET grid_columns = 48`)
	far := func(u *user) error {
		_, err := c.castArea(t, u, "Pensantus", wallOfFireKey, slotOfLevel(4), at(40, 7), nil, northWall(playv1.WallSide_WALL_SIDE_LEFT))
		return err
	}
	wantBlockedBy(t, "CastSpell", far(c.ana), blockedOutOfReach)
	if byLabel(t, c.get(t, c.master), "Pensantus").GetActionUsed() {
		t.Error("the refused cast spent the action")
	}
	if err := far(c.master); err != nil {
		t.Errorf("the master's cast was held to the range: %v", err)
	}
}

func TestTheMasterCanLayTheWall(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	c.h.roller.queue(5, 5, 5, 5, 5, 5)
	if _, err := c.castArea(t, c.master, "Pensantus", wallOfFireKey, slotOfLevel(4), at(7, 8), nil, northWall(playv1.WallSide_WALL_SIDE_RIGHT)); err != nil {
		t.Fatalf("CastSpell() as the master error = %v", err)
	}
	if z := c.oneZone(t, c.master, wallOfFireKey); len(z.GetSquares()) != 2 {
		t.Errorf("squares = %v, want 2", z.GetSquares())
	}
}

// The answer of the cast draws the wall, and the answer of a retry of the same key draws the same one.
func TestTheAnswerAndItsRetryDrawTheWall(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	c.h.roller.queue(20, 1, 1, 1, 1, 1)
	key := newKey()
	cast := func() *playv1.CastSpellResponse {
		res, err := c.castWall(t, 4, func(r *playv1.CastSpellRequest) {
			northWall(playv1.WallSide_WALL_SIDE_LEFT)(r)
			r.IdempotencyKey = key
		})
		if err != nil {
			t.Fatalf("CastSpell() error = %v", err)
		}
		return res
	}
	first, retry := cast(), cast()
	for name, res := range map[string]*playv1.CastSpellResponse{"cast": first, "retry": retry} {
		got := res.GetArea().GetSquares()
		if len(got) != 2 || got[0].GetCol() != 7 || got[1].GetCol() != 7 {
			t.Errorf("area of the %s = %v, want the squares (7,7) and (7,8)", name, got)
		}
	}
	if zs := c.get(t, c.master).GetZones(); len(zs) != 1 {
		t.Errorf("zones after the retry = %d, want 1", len(zs))
	}
}

func TestAPreviewOfTheWallShowsTheSquaresTheCastLays(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	wall := &playv1.SpellWall{Direction: toward(0, -1), Side: playv1.WallSide_WALL_SIDE_LEFT}
	res, err := c.ana.combat.PreviewSpellArea(t.Context(), connect.NewRequest(&playv1.PreviewSpellAreaRequest{
		CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), CasterId: c.id(t, "Pensantus"), SpellKey: wallOfFireKey,
		Slot: slotOfLevel(4), Area: &playv1.PreviewSpellAreaRequest_Origin{Origin: at(7, 8)}, Wall: wall,
	}))
	if err != nil {
		t.Fatalf("PreviewSpellArea() error = %v", err)
	}
	if got := labelsOfTargets(res.Msg); !slices.Equal(got, []string{"Ogro"}) {
		t.Errorf("targets = %v, want the ogre", got)
	}
	c.h.roller.queue(20, 1, 1, 1, 1, 1)
	if _, err := c.castWall(t, 4, northWall(playv1.WallSide_WALL_SIDE_LEFT)); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	z := c.oneZone(t, c.master, wallOfFireKey)
	if len(res.Msg.GetSquares()) != len(z.GetSquares()) {
		t.Fatalf("preview squares = %v, zone squares = %v", res.Msg.GetSquares(), z.GetSquares())
	}
	for _, s := range res.Msg.GetSquares() {
		if !zoneHas(z, s.GetCol(), s.GetRow()) {
			t.Errorf("the preview has %v, which the zone does not", s)
		}
	}
	_, err = c.ana.combat.PreviewSpellArea(t.Context(), connect.NewRequest(&playv1.PreviewSpellAreaRequest{
		CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), CasterId: c.id(t, "Pensantus"), SpellKey: wallOfFireKey,
		Slot: slotOfLevel(4), Area: &playv1.PreviewSpellAreaRequest_Origin{Origin: at(7, 8)},
	}))
	wantBlockedBy(t, "PreviewSpellArea without a wall", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WALL_PLACEMENT_REQUIRED)
}

// The ring around (19,7) holds the goblin, moved to (21,8), and is told to the viewer as a sphere of its radius.
func TestARingedWallIsLaidAroundThePoint(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	c.mustMove(t, c.master, "Goblin", 21, 8)
	c.h.roller.queue(5, 5, 5, 5, 5, 5)
	_, err := c.castArea(t, c.ana, "Pensantus", wallOfFireKey, slotOfLevel(4), at(19, 7), nil, func(r *playv1.CastSpellRequest) {
		r.Wall = &playv1.SpellWall{Ring: true, Side: playv1.WallSide_WALL_SIDE_OUTER}
	})
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	z := c.oneZone(t, c.master, wallOfFireKey)
	if len(z.GetSquares()) != 16 || zoneHas(z, 19, 7) || !zoneHas(z, 21, 7) || !zoneHas(z, 21, 8) {
		t.Errorf("squares = %v, want the 16 squares 10 ft from (19,7)", z.GetSquares())
	}
	if z.GetShape() != rulesv1.SpellAreaShape_SPELL_AREA_SHAPE_SPHERE || z.GetSizeFt() != 10 {
		t.Errorf("shape %v size %d, want a sphere of 10 ft: the ring's radius", z.GetShape(), z.GetSizeFt())
	}
	if cur, _, _ := c.hp(t, "Goblin"); cur > 0 {
		t.Errorf("the goblin has %d hit points, want it down: it stands in the ring", cur)
	}
}

func TestAStraightWallIsToldAsALineOfItsLength(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	c.h.roller.queue(20, 1, 1, 1, 1, 1)
	_, err := c.castWall(t, 4, func(r *playv1.CastSpellRequest) {
		r.Wall = &playv1.SpellWall{Direction: toward(0, -1), Side: playv1.WallSide_WALL_SIDE_LEFT, LengthFt: 25}
	})
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if z := c.oneZone(t, c.master, wallOfFireKey); z.GetShape() != rulesv1.SpellAreaShape_SPELL_AREA_SHAPE_LINE || z.GetSizeFt() != 25 {
		t.Errorf("shape %v size %d, want a line of 25 ft", z.GetShape(), z.GetSizeFt())
	}
}

// A player's character inside the wall at the cast is asked its save, and a failure leaves the damage for the master.
func TestAPlayerInsideTheWallIsAskedTheSaveAtTheCast(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	c.mustCastArea(t, c.ana, "Pensantus", wallOfFireKey, slotOfLevel(4), at(5, 8), nil, northWall(playv1.WallSide_WALL_SIDE_LEFT))
	w := c.windowOf(t, c.caio, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatal("Toren's player has no question: the wall caught his character at the cast")
	}
	if p := w.GetEffectSave(); p.GetZoneReason() != "zone_cast" || p.GetAbility() != "dex" || !p.GetHalfOnSave() {
		t.Errorf("prompt = %v, want a Dexterity zone save at the cast with half on a pass", p)
	}
	c.h.roller.queue(4, 4, 4, 4, 4) // 5d8 = 20
	if _, err := c.rollEffectSave(t, c.caio, c.get(t, c.master), w.GetId(), effectSaveFace(1)); err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	var amount int32
	if err := c.h.pool.QueryRow(t.Context(), `SELECT amount FROM pending_damages WHERE target_id = $1 AND effect_source_key = $2`, c.id(t, "Toren"), wallOfFireKey).Scan(&amount); err != nil || amount != 20 {
		t.Errorf("pending damage = %d, %v, want 20", amount, err)
	}
}

func TestAWallWithNoDirectionLaysNothingInTheGeometry(t *testing.T) {
	t.Parallel()
	if got := (wallSpec{LengthFt: 60}).squares(wsq(5, 5)); len(got) != 0 {
		t.Errorf("squares = %v, want none for a wall that runs nowhere", got)
	}
}

func TestWallSideIsStoredAndReadBack(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	c.wallFight(t)
	c.h.roller.queue(20, 1, 1, 1, 1, 1)
	if _, err := c.castWall(t, 4, northWall(playv1.WallSide_WALL_SIDE_LEFT)); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	zs, err := readZonesOf(t.Context(), c.h.svc.queries, c.get(t, c.master).GetId())
	if err != nil || len(zs) != 1 {
		t.Fatalf("readZonesOf = %v, %v, want one zone", zs, err)
	}
	z := zs[0]
	if z.wall == nil || z.wall.Side != playv1.WallSide_WALL_SIDE_LEFT || z.wall.Dir != (grid.Direction{Dy: -1}) || z.wall.LengthFt != 60 {
		t.Fatalf("wall read back = %+v, want left, north, 60 ft", z.wall)
	}
	// Walking north the left is west: Toren's square (5,7) is within 10 ft of the wall on that side.
	for sqr, want := range map[grid.Square]bool{wsq(7, 7): true, wsq(6, 8): true, wsq(5, 7): true, wsq(8, 7): false, wsq(9, 8): false, wsq(4, 7): false} {
		if z.hurtsAtEnd(sqr) != want {
			t.Errorf("hurtsAtEnd(%v) = %v, want %v", sqr, z.hurtsAtEnd(sqr), want)
		}
	}
}

func TestWallOfFireIsUnavailableInTheTurnOptions(t *testing.T) {
	t.Parallel()
	c := wallCave(t)
	e := c.zoneFight(t)
	e = c.passTo(t, e, "Pensantus")
	opt := spellOption(c.mustOptions(t, c.ana, e, "Pensantus"), wallOfFireKey)
	if opt == nil || opt.GetEnabled() || opt.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_WALL_PLACEMENT_REQUIRED {
		t.Fatalf("Wall of Fire option = %v, want disabled with WALL_PLACEMENT_REQUIRED", opt)
	}
	if o := spellOption(c.mustOptions(t, c.ana, e, "Pensantus"), cloudkillKey); o == nil || !o.GetEnabled() {
		t.Errorf("Cloudkill option = %v, want enabled", o)
	}
}
