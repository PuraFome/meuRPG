package play

import (
	"context"
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Fog Cloud, Darkness and Stinking Cloud (SRD 5.1) keep their sphere on the map and change what is seen. These
// tests need the database (MEURPG_TEST_DATABASE_URL). The cave is the one of zones_test.go: the ogre (saving throws
// +0) stands at (7,7), Pensantus the wizard (level 9) at (12,7), the goblin far to the east.

const (
	fogCloudKey      = "spell:fog-cloud"
	darknessKey      = "spell:darkness"
	stinkingCloudKey = "spell:stinking-cloud"
)

func obscureCave(t *testing.T) *cave {
	t.Helper()
	return newCaveOf(t, 9, []string{fogCloudKey, darknessKey, stinkingCloudKey, thunderwave}, func(a *armed) *charactersv1.Character {
		return a.caio.caster(t, a.campaignID, "Toren", "class:cleric", "race:human", 7, abilities(14, 12, 14, 16, 10),
			[]string{maceKey}, []string{sacredFlame}, nil, []string{blessKey})
	})
}

// farthest is the largest distance, in squares, of a square of the zone from its origin.
func farthest(z interface {
	GetSquares() []*playv1.SpellOrigin
}, col, row int32,
) int32 {
	var far int32
	for _, s := range z.GetSquares() {
		far = max(far, abs32(s.GetCol()-col), abs32(s.GetRow()-row))
	}
	return far
}

func abs32(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

func (c *cave) combatPositions(t *testing.T) (clouds, dark []grid.Square) {
	t.Helper()
	pos, err := c.h.svc.CombatPositions(context.Background(), nil, c.campaignID, c.mapID)
	if err != nil {
		t.Fatalf("CombatPositions() error = %v", err)
	}
	return pos.Clouds, pos.MagicDark
}

// Fog Cloud is a sphere of 20 feet that grows by 20 feet for each slot above the first; it hits nobody.
func TestFogCloudGrowsWithTheSlotAndHitsNobody(t *testing.T) {
	t.Parallel()
	c := obscureCave(t)
	c.passTo(t, c.zoneFight(t), "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", fogCloudKey, slotOfLevel(1), at(7, 7), nil)
	zs := zonesOf(c.get(t, c.master), fogCloudKey)
	if len(zs) != 1 {
		t.Fatalf("zones = %v, want one", zs)
	}
	if far := farthest(zs[0], 7, 7); far != 4 {
		t.Errorf("reach at the first slot = %d squares, want 4 (20 ft)", far)
	}
	if zs[0].GetSizeFt() != 20 {
		t.Errorf("size read at the first slot = %d ft, want 20", zs[0].GetSizeFt())
	}
	if lines := c.zoneLines(t, c.master, fogCloudKey); len(lines) != 0 {
		t.Errorf("lines of the cloud = %v, want none: it hits nobody", lines)
	}
	clouds, dark := c.combatPositions(t)
	if len(clouds) != len(zs[0].GetSquares()) || len(dark) != 0 {
		t.Errorf("combat positions: %d cloud squares and %d dark, want %d and 0", len(clouds), len(dark), len(zs[0].GetSquares()))
	}
}

func TestFogCloudAtTheSecondSlotIsFortyFeet(t *testing.T) {
	t.Parallel()
	c := obscureCave(t)
	c.passTo(t, c.zoneFight(t), "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", fogCloudKey, slotOfLevel(2), at(7, 7), nil)
	zs := zonesOf(c.get(t, c.master), fogCloudKey)
	if len(zs) != 1 {
		t.Fatalf("zones = %v, want one", zs)
	}
	if far := farthest(zs[0], 7, 7); far != 8 {
		t.Errorf("reach at the second slot = %d squares, want 8 (40 ft)", far)
	}
	if zs[0].GetSizeFt() != 40 {
		t.Errorf("size read at the second slot = %d ft, want 40", zs[0].GetSizeFt())
	}
}

// Darkness is magical darkness of 15 feet, held by concentration; the fog reads it as dark, not as a cloud.
func TestDarknessIsMagicalDarknessHeldByConcentration(t *testing.T) {
	t.Parallel()
	c := obscureCave(t)
	c.passTo(t, c.zoneFight(t), "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", darknessKey, slotOfLevel(2), at(7, 7), nil)
	zs := zonesOf(c.get(t, c.master), darknessKey)
	if len(zs) != 1 {
		t.Fatalf("zones = %v, want one", zs)
	}
	if far := farthest(zs[0], 7, 7); far != 3 {
		t.Errorf("reach = %d squares, want 3 (15 ft)", far)
	}
	clouds, dark := c.combatPositions(t)
	if len(clouds) != 0 || len(dark) != len(zs[0].GetSquares()) {
		t.Errorf("combat positions: %d cloud squares and %d dark, want 0 and %d", len(clouds), len(dark), len(zs[0].GetSquares()))
	}
	c.nextTurnOf(t, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", fogCloudKey, slotOfLevel(1), at(7, 7), nil) // another concentration spell
	if zs := zonesOf(c.get(t, c.master), darknessKey); len(zs) != 0 {
		t.Errorf("darkness after the caster concentrated on another spell = %v, want none", zs)
	}
}

// The undo of the cast takes the cloud away, and with it the squares the fog reads.
func TestUndoOfTheCastTakesTheCloudAway(t *testing.T) {
	t.Parallel()
	c := obscureCave(t)
	c.passTo(t, c.zoneFight(t), "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", fogCloudKey, slotOfLevel(1), at(7, 7), nil)
	if clouds, _ := c.combatPositions(t); len(clouds) == 0 || len(zonesOf(c.get(t, c.master), fogCloudKey)) != 1 {
		t.Fatalf("before the undo: %d cloud squares, want some and one zone", len(clouds))
	}
	c.undoLast(t)
	if clouds, _ := c.combatPositions(t); len(clouds) != 0 || len(zonesOf(c.get(t, c.master), fogCloudKey)) != 0 {
		t.Errorf("after the undo: %d cloud squares, want none, and no zone", len(clouds))
	}
}

// Stinking Cloud: a creature that begins its turn inside makes a Constitution save, and on a failure spends its action.
func TestStinkingCloudSpendsTheActionOfWhoFailsAtTheStartOfItsTurn(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		face int
		want bool
	}{{"fail", 1, true}, {"pass", 20, false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := obscureCave(t)
			c.passTo(t, c.zoneFight(t), "Pensantus")
			c.mustCastArea(t, c.ana, "Pensantus", stinkingCloudKey, slotOfLevel(3), at(7, 7), nil)
			if byLabel(t, c.get(t, c.master), "Ogro").GetActionUsed() {
				t.Fatal("the ogre lost its action at the cast: the cloud hits nobody then")
			}
			c.h.roller.queue(tc.face)
			c.nextTurnOf(t, "Ogro")
			if got := byLabel(t, c.get(t, c.master), "Ogro").GetActionUsed(); got != tc.want {
				t.Errorf("action used at the start of the ogre's turn = %v, want %v", got, tc.want)
			}
			if len(c.zoneLines(t, c.master, stinkingCloudKey)) == 0 {
				t.Error("the cloud wrote no line for the save: it did not catch the ogre")
			}
		})
	}
}

// A creature outside the cloud is not asked.
func TestStinkingCloudDoesNotAskWhoBeginsItsTurnOutside(t *testing.T) {
	t.Parallel()
	c := obscureCave(t)
	c.passTo(t, c.zoneFight(t), "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", stinkingCloudKey, slotOfLevel(3), at(7, 14), nil)
	if z := c.oneZone(t, c.master, stinkingCloudKey); zoneHas(z, 7, 7) {
		t.Fatal("the cloud covers the ogre's square, so the test proves nothing")
	}
	c.nextTurnOf(t, "Ogro")
	if byLabel(t, c.get(t, c.master), "Ogro").GetActionUsed() || len(c.zoneLines(t, c.master, stinkingCloudKey)) != 0 {
		t.Error("the ogre outside the cloud was asked or lost its action")
	}
}

// A creature immune to poison damage is not asked by Stinking Cloud: the skeleton (immune to poison and to the
// poisoned condition) keeps its action; the ogre beside it, in the same cloud, loses its.
func TestStinkingCloudDoesNotAskWhoIsImmuneToPoison(t *testing.T) {
	t.Parallel()
	c := obscureCave(t)
	e := c.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: c.ogre.GetId()}},
		npcRolls: []int{2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 5},
		reveal:   []string{"Ogro"},
		at:       map[string][2]int32{"Toren": {5, 7}, "Pensantus": {12, 7}, "Brisa": {3, 7}, "Ogro": {7, 7}},
		setup:    true,
	})
	c.h.roller.queue(1)
	res := c.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Hidden = "monster:skeleton", 1, new(false)
	})
	var skeleton string
	for _, co := range res.GetEncounter().GetCombatants() {
		if co.GetId() == res.GetCombatantIds()[0] {
			skeleton = co.GetLabel()
		}
	}
	c.mustMove(t, c.master, skeleton, 8, 7)
	e = c.begin(t, c.get(t, c.master))
	c.passTo(t, e, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", stinkingCloudKey, slotOfLevel(3), at(7, 7), nil)
	c.h.roller.queue(1, 1)
	c.nextTurnOf(t, skeleton)
	if byLabel(t, c.get(t, c.master), skeleton).GetActionUsed() {
		t.Errorf("the skeleton lost its action to a gas it is immune to")
	}
	c.nextTurnOf(t, "Ogro")
	if !byLabel(t, c.get(t, c.master), "Ogro").GetActionUsed() {
		t.Error("the ogre kept its action: it failed the save with a 1")
	}
}

// A player's character caught at the start of its turn is asked the save (RN-37); a failure spends the action.
func TestAPlayerIsAskedStinkingCloudsSaveAndLosesTheActionOnAFailure(t *testing.T) {
	t.Parallel()
	c := obscureCave(t)
	e := c.zoneFight(t)
	c.mustMove(t, c.master, "Toren", 9, 7)
	c.passTo(t, e, "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", stinkingCloudKey, slotOfLevel(3), at(10, 7), nil)
	c.passRaw(t, "Toren")
	w := c.windowOf(t, c.caio, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatal("Toren's player has no question: the cloud caught his character at the start of its turn")
	}
	p := w.GetEffectSave()
	if p.GetZoneId() == "" || p.GetZoneReason() != "zone_start" || p.GetAbility() != "con" {
		t.Errorf("prompt = %v, want a Constitution zone save with reason zone_start", p)
	}
	if res, err := c.rollEffectSave(t, c.caio, c.get(t, c.master), w.GetId(), effectSaveFace(2)); err != nil || res.GetResult().GetSaved() {
		t.Fatalf("RollEffectSave() = %v, %v; want a failed save", res, err)
	}
	if !byLabel(t, c.get(t, c.master), "Toren").GetActionUsed() {
		t.Error("Toren kept his action after failing the save")
	}
}

// A cloud or darkness that goes away tells the fog, whichever way it goes: the caster set to 0 hit points by the
// master (the concentration ends), the combat ended by the master, and the session ended with the combat open.
func TestTheFogHearsOfACloudThatEndsOutsideACombatWrite(t *testing.T) {
	t.Parallel()
	ends := []struct {
		name string
		end  func(t *testing.T, c *cave)
	}{
		{"the caster is set to 0 hit points", func(t *testing.T, c *cave) {
			t.Helper()
			var char string
			for _, co := range c.get(t, c.master).GetCombatants() {
				if co.GetId() == c.id(t, "Pensantus") {
					char = co.GetCharacterId()
				}
			}
			zero := int32(0)
			if _, err := c.master.play.AdjustCharacterVitals(t.Context(), connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{
				CampaignId: c.campaignID, CharacterId: char, IdempotencyKey: newKey(), HitPointsCurrent: &zero,
			})); err != nil {
				t.Fatalf("AdjustCharacterVitals() error = %v", err)
			}
		}},
		{"the master ends the combat", func(t *testing.T, c *cave) {
			t.Helper()
			e := c.get(t, c.master)
			if _, err := c.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: c.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
				t.Fatalf("EndEncounter() error = %v", err)
			}
		}},
		{"the session ends", func(t *testing.T, c *cave) {
			t.Helper()
			sessions, err := c.master.play.ListGameSessions(t.Context(), connect.NewRequest(&playv1.ListGameSessionsRequest{CampaignId: c.campaignID}))
			if err != nil || len(sessions.Msg.GetGameSessions()) == 0 {
				t.Fatalf("ListGameSessions() = %v, %v", sessions, err)
			}
			if _, err := c.master.play.EndGameSession(t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: c.campaignID, GameSessionId: sessions.Msg.GetGameSessions()[0].GetId()})); err != nil {
				t.Fatalf("EndGameSession() error = %v", err)
			}
		}},
	}
	for _, tc := range ends {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := obscureCave(t)
			c.passTo(t, c.zoneFight(t), "Pensantus")
			c.mustCastArea(t, c.ana, "Pensantus", fogCloudKey, slotOfLevel(1), at(7, 7), nil)
			if clouds, _ := c.combatPositions(t); len(clouds) == 0 {
				t.Fatal("the cast put no cloud squares on the map")
			}
			before := c.h.vision.n.Load()
			tc.end(t, c)
			if got := c.h.vision.n.Load(); got <= before {
				t.Errorf("fog refreshes after the cloud ended = %d, before %d: the fog was not told", got, before)
			}
			if clouds, _ := c.combatPositions(t); len(clouds) != 0 {
				t.Errorf("cloud squares after = %d, want none", len(clouds))
			}
		})
	}
}

// A zone keeps up to 4,096 squares, enough for Fog Cloud at a 9th-level slot (a sphere of 36 squares of radius,
// 4,053 squares); one more is refused by the table's check (migration 00296).
func TestAZoneKeepsUpTo4096SquaresAndNoMore(t *testing.T) {
	t.Parallel()
	c := obscureCave(t)
	c.passTo(t, c.zoneFight(t), "Pensantus")
	c.mustCastArea(t, c.ana, "Pensantus", fogCloudKey, slotOfLevel(1), at(7, 7), nil)
	set := func(n int) error {
		_, err := c.h.pool.Exec(t.Context(), `UPDATE spell_zones SET squares = (SELECT jsonb_agg(jsonb_build_array(i % 64, i / 64)) FROM generate_series(0, $1::int - 1) AS i) WHERE spell_key = $2`, n, fogCloudKey)
		return err
	}
	if err := set(4096); err != nil {
		t.Errorf("4,096 squares: %v, want them kept", err)
	}
	if err := set(4097); err == nil {
		t.Error("4,097 squares were kept, want the check to refuse them")
	}
}

// An NPC caster the master removes from the combat takes its cloud with it, and the fog hears of it.
func TestTheFogHearsOfACloudWhoseCasterLeavesTheCombat(t *testing.T) {
	t.Parallel()
	c := newCaveWith(t, 5, []string{fogCloudKey})
	c.druidFightWith(t, []string{fogCloudKey}, "Druida")
	c.mustCastArea(t, c.master, "Druida", fogCloudKey, slotOfLevel(1), at(7, 7), nil)
	if clouds, _ := c.combatPositions(t); len(clouds) == 0 {
		t.Fatal("the cast put no cloud squares on the map")
	}
	before := c.h.vision.n.Load()
	if _, err := c.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
		CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), CombatantId: c.id(t, "Druida"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("RemoveCombatant() error = %v", err)
	}
	if got := c.h.vision.n.Load(); got <= before {
		t.Errorf("fog refreshes after the caster left = %d, before %d: the fog was not told", got, before)
	}
	if clouds, _ := c.combatPositions(t); len(clouds) != 0 {
		t.Errorf("cloud squares after = %d, want none", len(clouds))
	}
}

// The master moving a cloud changes the squares that block sight, and the fog hears of it.
func TestTheFogHearsOfACloudTheMasterMoves(t *testing.T) {
	t.Parallel()
	c := newCaveWith(t, 5, []string{fogCloudKey})
	c.druidFightWith(t, []string{fogCloudKey}, "Druida")
	c.mustCastArea(t, c.master, "Druida", fogCloudKey, slotOfLevel(1), at(7, 7), nil)
	z := c.oneZone(t, c.master, fogCloudKey)
	was, _ := c.combatPositions(t)
	before := c.h.vision.n.Load()
	if _, err := c.moveZone(t, c.master, z.GetId(), 12, 7); err != nil {
		t.Fatalf("MoveSpellZone() error = %v", err)
	}
	if got := c.h.vision.n.Load(); got <= before {
		t.Errorf("fog refreshes after the master moved the cloud = %d, before %d: the fog was not told", got, before)
	}
	if now, _ := c.combatPositions(t); slices.Equal(now, was) {
		t.Errorf("cloud squares after the move = those before (%d squares): the cloud did not move", len(now))
	}
}
