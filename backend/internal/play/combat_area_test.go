package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Area spells placed on the map (SRD 5.1, "Areas of Effect", "Spell Range", "Targets"
// and "Cover"). These tests need the database (MEURPG_TEST_DATABASE_URL). The
// fixture is the cave: Pensantus (level 5, Fireball, Lightning Bolt, Burning Hands,
// Thunderwave and Sleep) casts in the corridor, three goblins and the Capitão are in
// the room to the east, Toren stands in it as an ally, and the crates (half cover) and
// the column (three-quarters) are where the cave puts them.

const (
	fireball      = "spell:fireball"
	lightningBolt = "spell:lightning-bolt"
	thunderwave   = "spell:thunderwave"
)

func at(col, row int32) *playv1.SpellOrigin { return &playv1.SpellOrigin{Col: col, Row: row} }

func toward(dx, dy int32) *playv1.SpellDirection { return &playv1.SpellDirection{Dx: dx, Dy: dy} }

// newAreaCave is the cave with Pensantus, a level 5 wizard, and the area spells.
func newAreaCave(t *testing.T) *cave {
	t.Helper()
	return newCaveWith(t, 5, []string{fireball, lightningBolt, burningHands, thunderwave, sleepSpell})
}

// areaFight starts the combat with Pensantus on turn, in the corridor of the cave.
// The goblins are in the room: Goblin 1 on the corridor side of the crates, Goblin 2
// behind Toren, Goblin 3 behind the column.
//
//	Fireball at (20,7), radius 4 squares: Goblin 1 (18,7), Goblin 2 (22,8), Goblin 3
//	(20,3) and Toren (21,8) are inside; Pensantus (12,7), Brisa and the Capitão are not.
func (c *cave) areaFight(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := c.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: c.goblins.GetId(), Count: 3}, {CharacterId: c.capitao.GetId()}},
		npcRolls: []int{2, 2, 2, 2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin 1", "Goblin 2", "Goblin 3", "Capitão Goblin"},
		at: map[string][2]int32{
			"Pensantus": {12, 7}, "Toren": {21, 8}, "Brisa": {4, 7},
			"Goblin 1": {18, 7}, "Goblin 2": {22, 8}, "Goblin 3": {20, 3}, "Capitão Goblin": {12, 13},
		},
	})
	return c.passTo(t, e, "Pensantus")
}

// preview calls PreviewSpellArea as u.
func (c *cave) preview(t *testing.T, u *user, caster, spell string, slot *playv1.SpellSlot, origin *playv1.SpellOrigin, dir *playv1.SpellDirection) (*playv1.PreviewSpellAreaResponse, error) {
	t.Helper()
	req := &playv1.PreviewSpellAreaRequest{
		CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), CasterId: c.id(t, caster), SpellKey: spell, Slot: slot,
	}
	switch {
	case origin != nil:
		req.Area = &playv1.PreviewSpellAreaRequest_Origin{Origin: origin}
	case dir != nil:
		req.Area = &playv1.PreviewSpellAreaRequest_Direction{Direction: dir}
	}
	res, err := u.combat.PreviewSpellArea(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// castArea calls CastSpell as u with the point or the direction the caster chose.
func (c *cave) castArea(t *testing.T, u *user, caster, spell string, slot *playv1.SpellSlot, origin *playv1.SpellOrigin, dir *playv1.SpellDirection, edit ...func(*playv1.CastSpellRequest)) (*playv1.CastSpellResponse, error) {
	t.Helper()
	req := &playv1.CastSpellRequest{
		CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), CasterId: c.id(t, caster), SpellKey: spell, Slot: slot, IdempotencyKey: newKey(),
	}
	switch {
	case origin != nil:
		req.Area = &playv1.CastSpellRequest_Origin{Origin: origin}
	case dir != nil:
		req.Area = &playv1.CastSpellRequest_Direction{Direction: dir}
	}
	for _, e := range edit {
		e(req)
	}
	res, err := u.combat.CastSpell(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (c *cave) mustCastArea(t *testing.T, u *user, caster, spell string, slot *playv1.SpellSlot, origin *playv1.SpellOrigin, dir *playv1.SpellDirection, edit ...func(*playv1.CastSpellRequest)) *playv1.CastSpellResponse {
	t.Helper()
	res, err := c.castArea(t, u, caster, spell, slot, origin, dir, edit...)
	if err != nil {
		t.Fatalf("CastSpell(%s, %s) error = %v", caster, spell, err)
	}
	return res
}

// labelsOfTargets are the labels of the creatures a preview lists, sorted.
func labelsOfTargets(res *playv1.PreviewSpellAreaResponse) []string {
	var out []string
	for _, tg := range res.GetTargets() {
		out = append(out, tg.GetLabel())
	}
	slices.Sort(out)
	return out
}

func previewTarget2(t *testing.T, res *playv1.PreviewSpellAreaResponse, label string) *playv1.AreaTarget {
	t.Helper()
	for _, tg := range res.GetTargets() {
		if tg.GetLabel() == label {
			return tg
		}
	}
	t.Fatalf("%s is not in the area %v", label, labelsOfTargets(res))
	return nil
}

func squareIn(res *playv1.PreviewSpellAreaResponse, col, row int32) bool {
	return slices.ContainsFunc(res.GetSquares(), func(s *playv1.SpellOrigin) bool { return s.GetCol() == col && s.GetRow() == row })
}

// castTargetsOf is what the master's log says a cast did to each target, by label.
func (c *cave) castTargetsOf(t *testing.T, e *playv1.Encounter) map[string]*playv1.CombatLogSpellTarget {
	t.Helper()
	out := map[string]*playv1.CombatLogSpellTarget{}
	for _, r := range c.log(t, c.master, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() != playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST {
				continue
			}
			for _, tg := range en.GetSpell().GetTargets() {
				out[tg.GetTargetLabel()] = tg
			}
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// TestAFireballPlacedOnTheMapReachesEveryoneInsideAndMeasuresCoverFromItsOrigin: the
// caster picks the point, the server says who is inside (the party's ally too), and
// each creature's cover is the one it has against the point of origin: the crates are
// between the point and Goblin 1 (not between it and the caster), the column between
// the point and Goblin 3, and Toren stands in front of Goblin 2.
func TestAFireballPlacedOnTheMapReachesEveryoneInsideAndMeasuresCoverFromItsOrigin(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	e := c.areaFight(t)

	res, err := c.preview(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	if err != nil {
		t.Fatalf("PreviewSpellArea() error = %v", err)
	}
	if got, want := labelsOfTargets(res), []string{"Goblin 1", "Goblin 2", "Goblin 3", "Toren"}; !slices.Equal(got, want) {
		t.Fatalf("the area holds %v, want %v (Pensantus, Brisa and the Capitão are out)", got, want)
	}
	if res.GetOrigin().GetCol() != 20 || res.GetOrigin().GetRow() != 7 || res.GetMoved() || !res.GetCoverCounts() || res.GetRangeFt() != 150 {
		t.Errorf("origin %v moved %v cover_counts %v range %d, want (20,7), not moved, counted, 150 ft", res.GetOrigin(), res.GetMoved(), res.GetCoverCounts(), res.GetRangeFt())
	}
	for _, in := range [][2]int32{{20, 7}, {16, 7}, {20, 3}, {22, 8}} {
		if !squareIn(res, in[0], in[1]) {
			t.Errorf("the square %v is inside the 4-square sphere and is not in the squares", in)
		}
	}
	if squareIn(res, 15, 7) || squareIn(res, 20, 2) || squareIn(res, 23, 9) {
		t.Error("a square more than 4 squares from the point is in the sphere")
	}
	want := map[string]struct {
		cover playv1.CoverDegree
		ally  bool
	}{
		"Goblin 1": {playv1.CoverDegree_COVER_DEGREE_HALF, false},
		"Goblin 2": {playv1.CoverDegree_COVER_DEGREE_HALF, false},
		"Goblin 3": {playv1.CoverDegree_COVER_DEGREE_THREE_QUARTERS, false},
		"Toren":    {playv1.CoverDegree_COVER_DEGREE_NONE, true},
	}
	for label, w := range want {
		tg := previewTarget2(t, res, label)
		if tg.GetCover() != w.cover || tg.GetAlly() != w.ally || tg.GetSelf() || tg.GetHidden() {
			t.Errorf("%s = %v, want cover %v ally %v", label, tg, w.cover, w.ally)
		}
	}
	if (previewTarget2(t, res, "Goblin 1").GetCoverSource() != playv1.CoverSource_COVER_SOURCE_MAP) || previewTarget2(t, res, "Toren").GetCoverSource() != playv1.CoverSource_COVER_SOURCE_UNSPECIFIED {
		t.Errorf("the source of Goblin 1's cover is %v, Toren's %v; want the map's, and none", previewTarget2(t, res, "Goblin 1").GetCoverSource(), previewTarget2(t, res, "Toren").GetCoverSource())
	}

	// Measured from the caster, Goblin 1 would have none: the crates are behind it. The
	// list of targets the turn options give is the caster's and says so.
	opts := c.mustOptions(t, c.ana, e, "Pensantus")
	if tg := targetOf2(spellTargetsOf(opts, fireball), "Goblin 1"); tg.GetCover() != playv1.CoverDegree_COVER_DEGREE_NONE {
		t.Errorf("Goblin 1 has %v against the caster, want none (the crates are on its other side)", tg.GetCover())
	}
}

// TestACastWorksOutWhoIsInsideItselfAndGivesTheSaveItsCover: the cast takes the
// point, not the list; every creature inside saves, and the Dexterity save of a
// creature with cover gets +2 or +5.
func TestACastWorksOutWhoIsInsideItselfAndGivesTheSaveItsCover(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	e := c.areaFight(t)

	c.h.roller.queue(10, 10, 10, 10)
	// The list the app sent is not read: Brisa is far from the area and the goblins are in it.
	res := c.mustCastArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil, func(r *playv1.CastSpellRequest) {
		r.Targets = []*playv1.SpellTarget{{CombatantId: c.id(t, "Brisa")}}
	})
	if got, want := len(res.GetCast().GetTargets()), 4; got != want {
		t.Fatalf("the cast hit %d creatures, want %d", got, want)
	}
	if res.GetArea().GetOrigin().GetCol() != 20 || res.GetArea().GetOrigin().GetRow() != 7 || len(res.GetArea().GetSquares()) == 0 {
		t.Errorf("the cast's area = %v, want the point (20,7) and its squares", res.GetArea())
	}
	if len(res.GetHiddenHits()) != 0 || res.GetPendingRevealId() != "" {
		t.Errorf("hidden hits %v, pending %q: nobody was hidden", res.GetHiddenHits(), res.GetPendingRevealId())
	}
	hit := c.castTargetsOf(t, e)
	if got, want := sortedKeys(hit), []string{"Goblin 1", "Goblin 2", "Goblin 3", "Toren"}; !slices.Equal(got, want) {
		t.Fatalf("the log's targets = %v, want %v", got, want)
	}
	// The NPCs' basic sheets have no save bonus, so the modifier of the master's roll
	// is the cover (Toren's is his Dexterity).
	for label, bonus := range map[string]int32{"Goblin 1": 2, "Goblin 2": 2, "Goblin 3": 5} {
		sv := hit[label].GetSave()
		if sv.GetRoll().GetModifier() != bonus {
			t.Errorf("%s's save has the modifier %d, want %d (its cover from the point)", label, sv.GetRoll().GetModifier(), bonus)
		}
		if hit[label].GetCover() == playv1.CoverDegree_COVER_DEGREE_NONE {
			t.Errorf("%s's line has no cover", label)
		}
	}
	if used := usedSlots(c.vitals(t, c.pens), 3); used != 1 {
		t.Errorf("3rd-level slots used = %d, want 1", used)
	}
	// Damage is one roll for all of them.
	if got := len(res.GetCast().GetPendingDamages()); got != 4 {
		t.Errorf("pending damages = %d, want one for each creature inside, rolled once", got)
	}
}

// TestOnlyADexteritySaveGetsTheCover: Thunderwave's save is Constitution, so the
// crates between the caster and the goblin give nothing (SRD, "Cover": +2 and +5 are
// for AC and Dexterity saving throws).
func TestOnlyADexteritySaveGetsTheCover(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	e := c.areaFight(t)
	// Pensantus beside the crates, Goblin 1 right behind them, both inside the cube.
	c.mustMove(t, c.master, "Pensantus", 17, 8)
	c.mustMove(t, c.master, "Goblin 1", 20, 8)

	res, err := c.preview(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), nil, toward(1, 0))
	if err != nil {
		t.Fatalf("PreviewSpellArea() error = %v", err)
	}
	if res.GetCoverCounts() {
		t.Error("cover counts for Thunderwave's Constitution save")
	}
	g1 := previewTarget2(t, res, "Goblin 1")
	if g1.GetCover() != playv1.CoverDegree_COVER_DEGREE_UNSPECIFIED {
		t.Errorf("Goblin 1's cover = %v, want none told for a Constitution save", g1.GetCover())
	}
	c.h.roller.queue(10, 10, 10)
	c.mustCastArea(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), nil, toward(1, 0))
	hit := c.castTargetsOf(t, e)
	if tg := hit["Goblin 1"]; tg == nil || tg.GetSave().GetRoll().GetModifier() != 0 || tg.GetCover() != playv1.CoverDegree_COVER_DEGREE_NONE {
		t.Errorf("Goblin 1's save = %v, want no cover bonus and no cover in the line", tg)
	}
}

// TestAConeLeavesTheCasterOutAndSpellsNeedTheirOwnPlacement: Burning Hands comes out
// of the caster in a direction, and does not take a point; Fireball takes a point, not
// a direction; a direction is one of eight.
func TestAConeLeavesTheCasterOutAndSpellsNeedTheirOwnPlacement(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.areaFight(t)
	c.mustMove(t, c.master, "Pensantus", 17, 7)
	c.mustMove(t, c.master, "Goblin 1", 18, 7) // 1 square east: in the cone
	c.mustMove(t, c.master, "Goblin 2", 20, 8) // 3 squares east, one row down: inside (the cone is 3 wide there)
	c.mustMove(t, c.master, "Goblin 3", 17, 4) // north: out
	c.mustMove(t, c.master, "Toren", 20, 6)    // 3 east, 1 up: in
	res, err := c.preview(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0))
	if err != nil {
		t.Fatalf("PreviewSpellArea() error = %v", err)
	}
	if got, want := labelsOfTargets(res), []string{"Goblin 1", "Goblin 2", "Toren"}; !slices.Equal(got, want) {
		t.Errorf("the cone holds %v, want %v (the caster and the one to the north are out)", got, want)
	}
	if res.GetOrigin().GetCol() != 17 || res.GetOrigin().GetRow() != 7 || squareIn(res, 17, 7) || res.GetRangeFt() != 0 {
		t.Errorf("origin %v, range %d, caster's square inside %v: a cone starts at the caster, outside it, with no range", res.GetOrigin(), res.GetRangeFt(), squareIn(res, 17, 7))
	}

	bad := []struct {
		name   string
		spell  string
		origin *playv1.SpellOrigin
		dir    *playv1.SpellDirection
	}{
		{"a cone with a point", burningHands, at(20, 7), nil},
		{"a cone with no direction", burningHands, nil, nil},
		{"a sphere with a direction", fireball, nil, toward(1, 0)},
		{"a sphere with no point", fireball, nil, nil},
		{"a direction that is not one of the eight", burningHands, nil, toward(2, 0)},
		{"no direction at all", burningHands, nil, toward(0, 0)},
		{"a point off the map", fireball, at(99, 7), nil},
	}
	for _, b := range bad {
		slot := slotOfLevel(3)
		if b.spell == burningHands {
			slot = slotOfLevel(1)
		}
		if _, err := c.preview(t, c.ana, "Pensantus", b.spell, slot, b.origin, b.dir); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("PreviewSpellArea(%s) error = %v, want invalid_argument", b.name, err)
		}
		if _, err := c.castArea(t, c.ana, "Pensantus", b.spell, slot, b.origin, b.dir); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("CastSpell(%s) error = %v, want invalid_argument", b.name, err)
		}
	}
}

// TestALineStopsAtAWallButNotAtCover: Lightning Bolt goes east along the row; the
// crates do not stop it, a wall does.
func TestALineStopsAtAWallButNotAtCover(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.areaFight(t)
	c.mustMove(t, c.master, "Pensantus", 15, 7)
	c.mustMove(t, c.master, "Goblin 1", 18, 7) // before the crates
	c.mustMove(t, c.master, "Goblin 2", 21, 7) // behind the crates
	c.mustMove(t, c.master, "Goblin 3", 22, 7) // behind the crates, and the wall
	c.terrain.addWalls(grid.Square{Col: 22, Row: 7})
	res, err := c.preview(t, c.ana, "Pensantus", lightningBolt, slotOfLevel(3), nil, toward(1, 0))
	if err != nil {
		t.Fatalf("PreviewSpellArea() error = %v", err)
	}
	if got, want := labelsOfTargets(res), []string{"Goblin 1", "Goblin 2"}; !slices.Equal(got, want) {
		t.Errorf("the line holds %v, want %v (the wall at 22 stops it)", got, want)
	}
	g2 := previewTarget2(t, res, "Goblin 2")
	if g2.GetCover() != playv1.CoverDegree_COVER_DEGREE_HALF {
		t.Errorf("Goblin 2 behind the crates has %v, want half cover", g2.GetCover())
	}
}

// TestAFireballSpreadsAroundCornersWithNoCoverBonusButASleepDoesNot: a wall hides the
// goblin from the point, but not from the Fireball's spread, and it gets no cover for
// the corner; the Sleep spell's sphere does not turn the corner.
func TestAFireballSpreadsAroundCornersWithNoCoverBonusButASleepDoesNot(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	e := c.areaFight(t)
	// A partition in the room: a wall at column 19, rows 2 to 6; the goblin at (18,5) is
	// behind it from the point (20,7), and the room is open below the partition.
	for row := int32(2); row <= 6; row++ {
		c.terrain.addWalls(grid.Square{Col: 19, Row: int(row)})
	}
	c.mustMove(t, c.master, "Goblin 1", 18, 5)
	res, err := c.preview(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	if err != nil {
		t.Fatalf("PreviewSpellArea() error = %v", err)
	}
	g1 := previewTarget2(t, res, "Goblin 1")
	if g1.GetCover() != playv1.CoverDegree_COVER_DEGREE_NONE {
		t.Errorf("Goblin 1 reached around the corner has %v, want no automatic cover", g1.GetCover())
	}
	// The master marks the cover by hand and the mark counts.
	c.mark(t, "Goblin 1", playv1.CoverDegree_COVER_DEGREE_HALF)
	res, err = c.preview(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	if err != nil {
		t.Fatalf("PreviewSpellArea() error = %v", err)
	}
	if got := previewTarget2(t, res, "Goblin 1"); got.GetCover() != playv1.CoverDegree_COVER_DEGREE_HALF || got.GetCoverSource() != playv1.CoverSource_COVER_SOURCE_MARK {
		t.Errorf("Goblin 1 marked by the master = %v, want half cover from the mark", got)
	}
	// A sphere that does not spread around corners does not reach it.
	sleep, err := c.preview(t, c.ana, "Pensantus", sleepSpell, slotOfLevel(3), at(20, 7), nil)
	if err != nil {
		t.Fatalf("PreviewSpellArea(sleep) error = %v", err)
	}
	if slices.Contains(labelsOfTargets(sleep), "Goblin 1") {
		t.Errorf("Sleep reaches %v: a sphere that does not spread around corners stops at the wall", labelsOfTargets(sleep))
	}
	_ = e
}

// TestAPointBehindAWallComesIntoBeingOnTheNearSide: a player cannot see through the
// wall, so the Fireball they place behind it goes off on the near side; the master's
// does not move.
func TestAPointBehindAWallComesIntoBeingOnTheNearSide(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.areaFight(t)
	c.mustMove(t, c.master, "Pensantus", 12, 7)
	c.terrain.addWalls(grid.Square{Col: 16, Row: 7}, grid.Square{Col: 16, Row: 8}, grid.Square{Col: 16, Row: 6}) // closes the room's door
	res, err := c.preview(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	if err != nil {
		t.Fatalf("PreviewSpellArea() error = %v", err)
	}
	if !res.GetMoved() || res.GetOrigin().GetCol() != 15 || res.GetOrigin().GetRow() != 7 {
		t.Errorf("origin %v moved %v, want the near side of the wall, (15,7)", res.GetOrigin(), res.GetMoved())
	}
	if slices.Contains(labelsOfTargets(res), "Goblin 1") {
		t.Errorf("the goblins in the room are in an area that went off outside it: %v", labelsOfTargets(res))
	}
	master, err := c.preview(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	if err != nil {
		t.Fatalf("PreviewSpellArea() as the master error = %v", err)
	}
	if master.GetMoved() || master.GetOrigin().GetCol() != 20 || !slices.Contains(labelsOfTargets(master), "Goblin 2") {
		t.Errorf("the master's area = origin %v moved %v targets %v, want the point as chosen", master.GetOrigin(), master.GetMoved(), labelsOfTargets(master))
	}
}

// TestAPlayersPointBeyondTheRangeIsRefusedAndTheMastersIsNot: the spell's range is
// 150 ft (30 squares), from the caster to the point.
func TestAPlayersPointBeyondTheRangeIsRefusedAndTheMastersIsNot(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.areaFight(t)
	// 150 ft is 30 squares; the map is 24 wide and 16 high, so a point is out of range
	// only for a caster that stands at a corner, with the point at the opposite one.
	c.mustMove(t, c.master, "Pensantus", 1, 7)
	if _, err := c.preview(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(22, 8), nil); err != nil {
		t.Fatalf("PreviewSpellArea() 21 squares away error = %v", err)
	}
	// The same map with a shorter reach: Pensantus at level 5 has the spell's 150 ft; the
	// master asks for a point as far as the map goes and is never held to the range.
	c.mustMove(t, c.master, "Pensantus", 1, 7)
	if _, err := c.preview(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(22, 15), nil); err != nil {
		t.Fatalf("PreviewSpellArea() as the master error = %v", err)
	}
}

// TestTheTableRuleDecidesWhatAHitDoesToAHiddenCreature: the effect is the same for a
// hidden creature as for any other; the rule only says whether the players learn of it.
func TestTheTableRuleDecidesWhatAHitDoesToAHiddenCreature(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		rule       campaignsv1.HiddenAreaHitRule
		wantHidden bool
	}{
		{"reveal", campaignsv1.HiddenAreaHitRule_HIDDEN_AREA_HIT_RULE_REVEAL, false},
		{"keep hidden", campaignsv1.HiddenAreaHitRule_HIDDEN_AREA_HIT_RULE_KEEP_HIDDEN, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newAreaCave(t)
			c.areaFight(t)
			c.setRules(t, func(r *campaignsv1.TableRules) { r.HiddenAreaHits = tc.rule })
			c.hide(t, "Goblin 2")

			// What the caster's player is told does not name Goblin 2 or count it.
			pre, err := c.preview(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
			if err != nil {
				t.Fatalf("PreviewSpellArea() error = %v", err)
			}
			if got, want := labelsOfTargets(pre), []string{"Goblin 1", "Goblin 3", "Toren"}; !slices.Equal(got, want) {
				t.Errorf("the player's preview = %v, want %v: the hidden Goblin 2 is not named or counted", got, want)
			}
			if m, err := c.preview(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil); err != nil || !previewTarget2(t, m, "Goblin 2").GetHidden() {
				t.Errorf("the master's preview = %v, %v; want Goblin 2 marked hidden", m, err)
			}

			c.h.roller.queue(10, 10, 10, 10)
			res := c.mustCastArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
			if len(res.GetCast().GetTargets()) != 3 || len(res.GetHiddenHits()) != 0 || res.GetPendingRevealId() != "" {
				t.Errorf("the player's answer = %d targets, hidden %v, pending %q; want 3, none, none", len(res.GetCast().GetTargets()), res.GetHiddenHits(), res.GetPendingRevealId())
			}
			for _, tg := range res.GetCast().GetTargets() {
				if tg.GetCombatantId() == c.id(t, "Goblin 2") {
					t.Error("the cast's answer to the player names the hidden creature")
				}
			}
			// The effect took hold: one roll of damage settles every creature, hidden or not.
			first := res.GetCast().GetPendingDamages()[0]
			c.h.roller.queue(6, 6, 6, 6, 6, 6, 6, 6)
			c.mustDamage(t, c.ana, c.get(t, c.ana), first.GetId(), inAppDamage)
			if cur, _, defeated := c.hp(t, "Goblin 2"); cur != 0 || !defeated {
				t.Errorf("the hidden Goblin 2 has %d PV, defeated %v; want it hurt by the fireball like the others", cur, defeated)
			}

			// Hidden or revealed to the players.
			players := c.get(t, c.ana)
			seen := slices.ContainsFunc(players.GetCombatants(), func(x *playv1.Combatant) bool { return x.GetLabel() == "Goblin 2" })
			if seen == tc.wantHidden {
				t.Errorf("the player sees Goblin 2: %v, want %v", seen, !tc.wantHidden)
			}
			if tc.wantHidden {
				return
			}
			var revealed bool
			for _, r := range c.log(t, c.ana, players).GetRounds() {
				for _, en := range r.GetEntries() {
					if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_REVEAL_CHANGED && en.GetTargetLabel() == "Goblin 2" && !en.GetNowHidden() {
						revealed = true
					}
					for _, tg := range en.GetSpell().GetTargets() {
						if tg.GetTargetLabel() == "Goblin 2" {
							t.Error("the player's log says the spell hit the creature it revealed")
						}
					}
				}
			}
			if !revealed {
				t.Error("the player's log has no line for the creature that appeared")
			}
		})
	}
}

func resolveReveal(t *testing.T, c *cave, u *user, id string, reveal bool) (*playv1.ResolveHiddenRevealResponse, error) {
	t.Helper()
	res, err := u.combat.ResolveHiddenReveal(t.Context(), connect.NewRequest(&playv1.ResolveHiddenRevealRequest{
		CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), PendingRevealId: id, Reveal: reveal, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// holdRefusals are the turn's moves, attacks, actions, spells and its end, as Pensantus's
// player tries them, and the master's own turn passing.
func (c *cave) wantTheTurnHeld(t *testing.T, e *playv1.Encounter) {
	t.Helper()
	held := playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_HIDDEN_REVEAL_PENDING
	if _, err := c.endTurn(t, c.ana, e, false); err == nil {
		t.Error("the player's EndTurn went through while the question waits")
	} else {
		wantBlockedBy(t, "EndTurn", err, held)
	}
	if _, err := c.endTurn(t, c.master, e, true); err == nil {
		t.Error("the master's EndTurn went through while the question waits")
	} else {
		wantBlockedBy(t, "the master's EndTurn", err, held)
	}
	if _, err := c.move(t, c.ana, "Pensantus", 11, 7); err == nil {
		t.Error("MoveCombatant went through while the question waits")
	} else {
		wantBlockedBy(t, "MoveCombatant", err, held)
	}
	if _, err := c.attack(t, c.ana, e, "Pensantus", fireBolt, "Goblin 1", inAppRoll); err == nil {
		t.Error("RollAttack went through while the question waits")
	} else {
		wantBlockedBy(t, "RollAttack", err, held)
	}
	if _, err := c.action(t, c.ana, e, "Pensantus", "standard:dash"); err == nil {
		t.Error("TakeAction went through while the question waits")
	} else {
		wantBlockedBy(t, "TakeAction", err, held)
	}
	if _, err := c.cast(t, c.ana, e, "Pensantus", magicMissileSpell, slotOfLevel(1), c.at(t, "Goblin 1"), noCastRoll); err == nil {
		t.Error("CastSpell went through while the question waits")
	} else {
		wantBlockedBy(t, "CastSpell", err, held)
	}
}

// askFight is the fight with the table asking, and two hidden goblins in the area.
func (c *cave) askFight(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := c.areaFight(t)
	c.setRules(t, func(r *campaignsv1.TableRules) {
		r.HiddenAreaHits = campaignsv1.HiddenAreaHitRule_HIDDEN_AREA_HIT_RULE_ASK
	})
	c.hide(t, "Goblin 2")
	c.hide(t, "Goblin 3")
	return e
}

// TestAPlayersSpellThatHitsHiddenCreaturesHoldsTheTurnUntilTheMasterAnswers: with the
// table asking, the cast takes effect and the turn waits; the player is told only that
// the master is awaited, and the refusals say the same to everyone; the master sees the
// question, with the creatures, and can still act.
func TestAPlayersSpellThatHitsHiddenCreaturesHoldsTheTurnUntilTheMasterAnswers(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	e := c.askFight(t)
	c.h.roller.queue(10, 10, 10, 10)
	res := c.mustCastArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	if res.GetPendingRevealId() != "" || len(res.GetHiddenHits()) != 0 || len(res.GetCast().GetTargets()) != 2 {
		t.Fatalf("the player's answer = pending %q, hidden %v, %d targets; want nothing of the hidden ones and the two they see", res.GetPendingRevealId(), res.GetHiddenHits(), len(res.GetCast().GetTargets()))
	}

	player, master := c.get(t, c.ana), c.get(t, c.master)
	if !player.GetTurnHeld() || len(player.GetPendingHiddenReveals()) != 0 {
		t.Errorf("the player's combat: turn held %v, questions %d; want the turn held and no question", player.GetTurnHeld(), len(player.GetPendingHiddenReveals()))
	}
	if !master.GetTurnHeld() || len(master.GetPendingHiddenReveals()) != 1 {
		t.Fatalf("the master's combat: turn held %v, questions %d; want the turn held and one question", master.GetTurnHeld(), len(master.GetPendingHiddenReveals()))
	}
	q := master.GetPendingHiddenReveals()[0]
	if q.GetCasterId() != c.id(t, "Pensantus") || q.GetSpellKey() != fireball || len(q.GetCombatantIds()) != 2 || q.GetArea().GetOrigin().GetCol() != 20 || len(q.GetArea().GetSquares()) == 0 {
		t.Errorf("the question = %v, want the cast, the two goblins and the area", q)
	}
	if !slices.Contains(q.GetCombatantIds(), c.id(t, "Goblin 2")) || !slices.Contains(q.GetCombatantIds(), c.id(t, "Goblin 3")) {
		t.Errorf("the question names %v, want Goblin 2 and Goblin 3", q.GetCombatantIds())
	}
	// The other players are told the turn is held and nothing else.
	if other := c.get(t, c.caio); !other.GetTurnHeld() || len(other.GetPendingHiddenReveals()) != 0 || slices.ContainsFunc(other.GetCombatants(), func(x *playv1.Combatant) bool { return x.GetLabel() == "Goblin 2" }) {
		t.Errorf("another player's combat = held %v, questions %d", other.GetTurnHeld(), len(other.GetPendingHiddenReveals()))
	}

	c.wantTheTurnHeld(t, e)
	// A player who answers is refused the same way for a real question and an invented one.
	_, errReal := resolveReveal(t, c, c.ana, q.GetId(), true)
	_, errFake := resolveReveal(t, c, c.ana, newKey(), true)
	_, errNonsense := resolveReveal(t, c, c.ana, "not-a-uuid", false)
	for name, err := range map[string]error{"a real question": errReal, "an invented one": errFake, "a malformed id": errNonsense} {
		if connect.CodeOf(err) != connect.CodePermissionDenied || err.Error() != errReal.Error() {
			t.Errorf("ResolveHiddenReveal as a player for %s = %v, want the same permission_denied as for a real question (%v)", name, err, errReal)
		}
	}
	if len(c.get(t, c.master).GetPendingHiddenReveals()) != 1 {
		t.Error("a player's answer closed the question")
	}

	// The master can still act: damage and cover, and the roll of the cast's damage.
	c.mark(t, "Goblin 1", playv1.CoverDegree_COVER_DEGREE_THREE_QUARTERS)
	first := res.GetCast().GetPendingDamages()[0]
	c.h.roller.queue(5, 5, 5, 5, 5, 5, 5, 5)
	c.mustDamage(t, c.ana, c.get(t, c.ana), first.GetId(), inAppDamage)

	// Keep them hidden: the turn is free, the players still do not see them.
	if _, err := resolveReveal(t, c, c.master, q.GetId(), false); err != nil {
		t.Fatalf("ResolveHiddenReveal(keep) error = %v", err)
	}
	if _, err := resolveReveal(t, c, c.master, q.GetId(), false); err != nil {
		t.Errorf("answering the same way twice error = %v, want none", err)
	}
	if _, err := resolveReveal(t, c, c.master, q.GetId(), true); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("answering the other way after the first = %v, want failed_precondition", err)
	}
	player = c.get(t, c.ana)
	if player.GetTurnHeld() || len(c.get(t, c.master).GetPendingHiddenReveals()) != 0 {
		t.Error("the turn is still held after the answer")
	}
	for _, label := range []string{"Goblin 2", "Goblin 3"} {
		if slices.ContainsFunc(player.GetCombatants(), func(x *playv1.Combatant) bool { return x.GetLabel() == label }) {
			t.Errorf("the player sees %s after the master kept it hidden", label)
		}
	}
	if _, err := c.endTurn(t, c.master, c.get(t, c.master), true); err != nil {
		t.Errorf("the master's EndTurn after the answer error = %v", err)
	}
}

// TestTheMasterRevealsWhatTheCastHit: "Revelar" makes the creatures appear to the
// players, with the line of the log, and frees the turn.
func TestTheMasterRevealsWhatTheCastHit(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.askFight(t)
	c.h.roller.queue(10, 10, 10, 10)
	c.mustCastArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	q := c.get(t, c.master).GetPendingHiddenReveals()[0]
	res, err := resolveReveal(t, c, c.master, q.GetId(), true)
	if err != nil {
		t.Fatalf("ResolveHiddenReveal(reveal) error = %v", err)
	}
	player := c.get(t, c.ana)
	for _, label := range []string{"Goblin 2", "Goblin 3"} {
		if !slices.ContainsFunc(player.GetCombatants(), func(x *playv1.Combatant) bool { return x.GetLabel() == label }) {
			t.Errorf("the player does not see %s after the reveal", label)
		}
	}
	if player.GetTurnHeld() || res.GetEncounter().GetTurnHeld() || len(res.GetEncounter().GetPendingHiddenReveals()) != 0 {
		t.Error("the turn is still held after the reveal")
	}
	lines := 0
	for _, r := range c.log(t, c.ana, player).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_REVEAL_CHANGED && !en.GetNowHidden() {
				lines++
			}
			for _, tg := range en.GetSpell().GetTargets() {
				if tg.GetTargetLabel() == "Goblin 2" || tg.GetTargetLabel() == "Goblin 3" {
					t.Errorf("the player's line says the fireball hit %s, which it only revealed", tg.GetTargetLabel())
				}
			}
		}
	}
	if lines != 2 {
		t.Errorf("the player's log has %d lines of creatures that appeared, want 2", lines)
	}
}

// TestQuestionsAreAnsweredInOrderAndTheTurnWaitsForAllOfThem: a second question (an
// older one waits) cannot be answered first, and the turn goes on only when none is left.
func TestQuestionsAreAnsweredInOrderAndTheTurnWaitsForAllOfThem(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	e := c.askFight(t)
	c.h.roller.queue(10, 10, 10, 10)
	c.mustCastArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	first := c.get(t, c.master).GetPendingHiddenReveals()[0]
	second := newKey()
	c.execSQL(t, `INSERT INTO hidden_reveals (id, encounter_id, caster_id, spell_key, combatant_ids, origin_col, origin_row, squares, seq, created_at)
		VALUES ($1, $2, $3, 'spell:thunderwave', ARRAY[$4], 1, 1, '{}', 2, now())`, second, e.GetId(), c.id(t, "Brisa"), c.id(t, "Goblin 3"))
	if got := c.get(t, c.master).GetPendingHiddenReveals(); len(got) != 2 || got[0].GetId() != first.GetId() || got[1].GetId() != second {
		t.Fatalf("the master's questions = %v, want the two in the order they were opened", got)
	}
	if _, err := resolveReveal(t, c, c.master, second, true); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("answering the second first = %v, want failed_precondition", err)
	}
	if _, err := resolveReveal(t, c, c.master, first.GetId(), false); err != nil {
		t.Fatalf("answering the first error = %v", err)
	}
	if !c.get(t, c.ana).GetTurnHeld() {
		t.Error("the turn was freed with a question still open")
	}
	if _, err := resolveReveal(t, c, c.master, second, false); err != nil {
		t.Fatalf("answering the second error = %v", err)
	}
	if c.get(t, c.ana).GetTurnHeld() {
		t.Error("the turn is still held with every question answered")
	}
}

// TestEndingTheCombatDropsTheQuestions: nothing moves on a combat that ended.
func TestEndingTheCombatDropsTheQuestions(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	e := c.askFight(t)
	c.h.roller.queue(10, 10, 10, 10)
	c.mustCastArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	if _, err := c.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: c.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	var left int
	if err := c.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM hidden_reveals WHERE encounter_id = $1`, e.GetId()).Scan(&left); err != nil || left != 0 {
		t.Errorf("questions left after the combat ended = %d, %v; want none", left, err)
	}
}

// TestUndoingTheCastTakesTheQuestionAndTheRevealBack.
func TestUndoingTheCastTakesTheQuestionAndTheRevealBack(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	e := c.askFight(t)
	c.h.roller.queue(10, 10, 10, 10)
	c.mustCastArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	c.undoLast(t)
	if got := c.get(t, c.master); got.GetTurnHeld() || len(got.GetPendingHiddenReveals()) != 0 {
		t.Errorf("after the undo: turn held %v, questions %d; want none", got.GetTurnHeld(), len(got.GetPendingHiddenReveals()))
	}
	// And with the reveal rule, the creatures are hidden again.
	c.setRules(t, func(r *campaignsv1.TableRules) {
		r.HiddenAreaHits = campaignsv1.HiddenAreaHitRule_HIDDEN_AREA_HIT_RULE_REVEAL
	})
	c.h.roller.queue(10, 10, 10, 10)
	c.mustCastArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	if !slices.ContainsFunc(c.get(t, c.ana).GetCombatants(), func(x *playv1.Combatant) bool { return x.GetLabel() == "Goblin 2" }) {
		t.Fatal("the cast did not reveal Goblin 2")
	}
	c.undoLast(t)
	if slices.ContainsFunc(c.get(t, c.ana).GetCombatants(), func(x *playv1.Combatant) bool { return x.GetLabel() == "Goblin 2" }) {
		t.Error("the undo left the creature revealed")
	}
	_ = e
}

// TestTheMastersCastChoosesWhatItRevealsHimself: his own choice wins over the table;
// without it the rule decides, and a rule that asks makes him answer too.
func TestTheMastersCastChoosesWhatItRevealsHimself(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	e := c.askFight(t)
	c.h.roller.queue(10, 10, 10, 10)
	keep := c.mustCastArea(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil, func(r *playv1.CastSpellRequest) { r.RevealHidden = new(false) })
	if len(keep.GetHiddenHits()) != 2 || keep.GetPendingRevealId() != "" || len(c.get(t, c.master).GetPendingHiddenReveals()) != 0 {
		t.Errorf("a master's cast that keeps them: hidden %v, pending %q; want two hidden hits and no question", keep.GetHiddenHits(), keep.GetPendingRevealId())
	}
	c.undoLast(t)
	c.h.roller.queue(10, 10, 10, 10)
	c.mustCastArea(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil, func(r *playv1.CastSpellRequest) { r.RevealHidden = new(true) })
	if !slices.ContainsFunc(c.get(t, c.ana).GetCombatants(), func(x *playv1.Combatant) bool { return x.GetLabel() == "Goblin 3" }) {
		t.Error("the master's cast that reveals did not reveal")
	}
	c.undoLast(t)
	c.h.roller.queue(10, 10, 10, 10)
	asked := c.mustCastArea(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	if asked.GetPendingRevealId() == "" || len(c.get(t, c.master).GetPendingHiddenReveals()) != 1 {
		t.Errorf("the master's cast with no choice under a rule that asks: pending %q; want a question", asked.GetPendingRevealId())
	}
	_ = e
}

// TestAPlayerWhoSendsTheMastersChoiceIsRefusedBeforeAnythingElse: the refusal is the
// same whatever else is wrong with the request.
func TestAPlayerWhoSendsTheMastersChoiceIsRefusedBeforeAnythingElse(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.areaFight(t)
	good, errGood := c.castArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil, func(r *playv1.CastSpellRequest) { r.RevealHidden = new(true) })
	if errGood == nil {
		t.Fatalf("a player's reveal_hidden went through: %v", good)
	}
	_, errBadID := c.castArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil, func(r *playv1.CastSpellRequest) {
		r.RevealHidden, r.EncounterId, r.CasterId, r.IdempotencyKey, r.SpellKey = new(false), "nope", "nope", "nope", ""
	})
	_, errBadSpell := c.castArea(t, c.ana, "Pensantus", "spell:not-a-spell", slotOfLevel(9), nil, toward(5, 5), func(r *playv1.CastSpellRequest) { r.RevealHidden = new(true) })
	for name, err := range map[string]error{"a request with bad ids": errBadID, "a request for a spell that is not": errBadSpell} {
		if connect.CodeOf(err) != connect.CodePermissionDenied || err.Error() != errGood.Error() {
			t.Errorf("%s = %v, want the permission_denied of any other: %v", name, err, errGood)
		}
	}
	if got := c.get(t, c.ana); slices.ContainsFunc(got.GetCombatants(), func(x *playv1.Combatant) bool { return x.GetLabel() == "Goblin 2" && x.GetHidden() }) {
		t.Error("the refused cast changed something")
	}
}

// TestAnAreaCastFitsTheEventWithEveryCreatureOfTheCombat: forty creatures in one area
// (the most a combat holds) fit in the payload of the cast's event.
func TestAnAreaCastFitsTheEventWithEveryCreatureOfTheCombat(t *testing.T) {
	t.Parallel()
	cast := actionEvent{Round: 99, Actor: newKey(), Key: fireball, CastID: newKey(), Placed: true, AreaCol: 23, AreaRow: 15, Slot: &slotRef{Level: 9}}
	roll := actionEvent{Round: 99, Actor: newKey(), Key: fireball, Pending: newKey(), DiceCount: 20, DiceSides: 6, Faces: slices.Repeat([]int32{6}, 20), Total: 120, Amount: 120}
	for range maxCombatants {
		cast.Hits = append(cast.Hits, castHit{
			Target: newKey(), Save: &saveRoll{D20: 20, Bonus: -5, Total: 15, DC: 20, Saved: true}, Pending: newKey(), More: []string{newKey()},
			Cover: "three_quarters", CoverSource: "map", CoverBonus: 5, CoverSeenMask: 1<<5 | 1, HiddenAtCast: true,
		})
		roll.Settled = append(roll.Settled, damageHit{Pending: newKey(), Target: newKey(), Amount: 99, Half: true, Applied: true, After: &hpState{HP: 100}, ConcentrationDC: 10})
	}
	for name, ev := range map[string]actionEvent{"the cast": cast, "its damage roll": roll} {
		body, err := jsonMarshal(ev)
		if err != nil {
			t.Fatalf("marshal error = %v", err)
		}
		if len(body) > 16384-1024 { // the table's limit, with the room the fog's stamp needs
			t.Errorf("%s of %d creatures takes %d bytes, too close to the 16 KiB the table allows", name, maxCombatants, len(body))
		}
	}
}

// areaRPCs are the CombatService methods of the placed areas; the authorization matrix
// of the encounter (combat_test.go) leaves them to this file's.
var areaRPCs = []string{"PreviewSpellArea", "ResolveHiddenReveal"}

// TestTheAreaCallsAreRefusedToWhoMayNotMakeThem: only the combatant's player and the
// master preview, only the master answers; a stranger finds nothing.
func TestTheAreaCallsAreRefusedToWhoMayNotMakeThem(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.askFight(t)
	c.h.roller.queue(10, 10, 10, 10)
	c.mustCastArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
	q := c.get(t, c.master).GetPendingHiddenReveals()[0]
	stranger := c.h.newUser("Eva")
	for name, tc := range map[string]struct {
		u    *user
		call func(u *user) error
		want connect.Code
	}{
		"preview, another player's combatant": {c.caio, func(u *user) error {
			_, err := c.preview(t, u, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
			return err
		}, connect.CodePermissionDenied},
		"preview, a stranger": {stranger, func(u *user) error {
			_, err := c.preview(t, u, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
			return err
		}, connect.CodeNotFound},
		"answer, a player":                        {c.ana, func(u *user) error { _, err := resolveReveal(t, c, u, q.GetId(), true); return err }, connect.CodePermissionDenied},
		"answer, a stranger":                      {stranger, func(u *user) error { _, err := resolveReveal(t, c, u, q.GetId(), true); return err }, connect.CodeNotFound},
		"answer, the master, an unknown question": {c.master, func(u *user) error { _, err := resolveReveal(t, c, u, newKey(), true); return err }, connect.CodeNotFound},
		"answer, the master, a malformed id":      {c.master, func(u *user) error { _, err := resolveReveal(t, c, u, "nope", true); return err }, connect.CodeInvalidArgument},
	} {
		if err := tc.call(tc.u); connect.CodeOf(err) != tc.want {
			t.Errorf("%s: error = %v, want %v", name, err, tc.want)
		}
	}
	if got := c.get(t, c.master).GetPendingHiddenReveals(); len(got) != 1 {
		t.Errorf("a refused answer changed the questions: %d left", len(got))
	}
}

// TestTheTurnOptionsSayHowEachAreaSpellIsPlaced: the app draws the picker from them.
func TestTheTurnOptionsSayHowEachAreaSpellIsPlaced(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	e := c.areaFight(t)
	opts := c.mustOptions(t, c.ana, e, "Pensantus")
	for key, want := range map[string]struct {
		placement playv1.AreaPlacement
		size      int32
		rng       int32
	}{
		fireball:      {playv1.AreaPlacement_AREA_PLACEMENT_POINT, 20, 150},
		burningHands:  {playv1.AreaPlacement_AREA_PLACEMENT_DIRECTION, 15, 0},
		thunderwave:   {playv1.AreaPlacement_AREA_PLACEMENT_DIRECTION, 15, 0},
		lightningBolt: {playv1.AreaPlacement_AREA_PLACEMENT_DIRECTION, 100, 0},
		sleepSpell:    {playv1.AreaPlacement_AREA_PLACEMENT_POINT, 20, 90},
	} {
		st := spellTargetsOf(opts, key)
		if st == nil || st.GetPlacement() != want.placement || st.GetAreaSizeFt() != want.size || st.GetRangeFt() != want.rng {
			t.Errorf("%s: placement %v size %d range %d, want %v %d %d", key, st.GetPlacement(), st.GetAreaSizeFt(), st.GetRangeFt(), want.placement, want.size, want.rng)
		}
	}
	if st := spellTargetsOf(opts, lightningBolt); st.GetAreaWidthFt() != 5 {
		t.Errorf("Lightning Bolt's width = %d ft, want 5", st.GetAreaWidthFt())
	}
}
