package play

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/cutting"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// The picker of the master's check says, for each creature, what the app needs to know before it asks the master anything: whether the
// map measures the creature against a bard (so the checkbox "Os bardos veem a criatura e ela os ouve" is not asked), and whether the
// creature is hidden from the players (so the checkbox starts unticked).

// mapWith makes a map of the table, with that many columns of grid (none: a map with no grid), the session's current one and returns its id.
func (n *npcTable) mapWith(t *testing.T, columns int) string {
	t.Helper()
	mapID := n.h.newMap(n.campaignID, columns)
	if _, err := n.master.play.SetCurrentMap(t.Context(), connect.NewRequest(&playv1.SetCurrentMapRequest{CampaignId: n.campaignID, MapId: mapID})); err != nil {
		t.Fatalf("SetCurrentMap() error = %v", err)
	}
	return mapID
}

// hideToken hides a character's token on the map.
func (n *npcTable) hideToken(t *testing.T, mapID, characterID string) {
	t.Helper()
	if _, err := n.h.pool.Exec(t.Context(), `UPDATE map_tokens SET hidden = true WHERE map_id = $1 AND character_id = $2`, mapID, characterID); err != nil {
		t.Fatalf("hide the token: %v", err)
	}
}

// candidate is the picker's entry of a creature, nil when it is not listed.
func (n *npcTable) candidate(t *testing.T, characterID string) *playv1.NpcCheckCandidate {
	t.Helper()
	res, err := n.master.play.ListNpcCheckPicker(t.Context(), connect.NewRequest(&playv1.ListNpcCheckPickerRequest{CampaignId: n.campaignID}))
	if err != nil {
		t.Fatalf("ListNpcCheckPicker() error = %v", err)
	}
	for _, c := range res.Msg.GetCandidates() {
		if c.GetCharacterId() == characterID {
			return c
		}
	}
	return nil
}

// TestNpcCheckPickerSaysWhetherTheMapMeasures: a creature is measured by the map when its token and the token of a bard that would be
// asked are both on the grid of the session's current map; with only the stage, only a token of the creature, a bard that would not be asked, or a
// creature of the bestiary, the checkbox decides.
func TestNpcCheckPickerSaysWhetherTheMapMeasures(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		ask     charactersv1.CuttingWordsAsk
		columns int // the map's grid, none for a map with no grid
		tokens  func(n *npcTable, mapID string)
		stage   bool
		want    bool
	}{
		"both tokens on the map": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, gridColumns, func(n *npcTable, m string) {
			n.h.placeToken(m, n.bri.GetId(), 250, 250)
			n.h.placeToken(m, n.npcID, 1750, 250)
		}, false, true},
		"the token of a bard that is not asked": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ONLY_ATTACKS, gridColumns, func(n *npcTable, m string) {
			n.h.placeToken(m, n.bri.GetId(), 250, 250)
			n.h.placeToken(m, n.npcID, 1750, 250)
		}, false, false},
		"the token of a character that is no bard": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, gridColumns, func(n *npcTable, m string) {
			n.h.placeToken(m, n.toren.GetId(), 250, 250)
			n.h.placeToken(m, n.npcID, 1750, 250)
		}, false, false},
		"no token of a bard": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, gridColumns, func(n *npcTable, m string) {
			n.h.placeToken(m, n.npcID, 1750, 250)
		}, false, false},
		"both tokens on a map with no grid": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, 0, func(n *npcTable, m string) {
			n.h.placeToken(m, n.bri.GetId(), 250, 250)
			n.h.placeToken(m, n.npcID, 1750, 250)
		}, false, false},
		"no token of the creature, on the stage": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, gridColumns, func(n *npcTable, m string) {
			n.h.placeToken(m, n.bri.GetId(), 250, 250)
		}, true, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, tc.ask)
			tc.tokens(n, n.mapWith(t, tc.columns))
			if tc.stage {
				n.putOnStage(t, false)
			}
			c := n.candidate(t, n.npcID)
			if c == nil {
				t.Fatal("the creature is not listed")
			}
			if c.GetMapMeasures() != tc.want {
				t.Errorf("map_measures = %v, want %v", c.GetMapMeasures(), tc.want)
			}
		})
	}
}

// putOnStage opens a scene and puts the NPC on its stage, hidden from the players or not.
func (n *npcTable) putOnStage(t *testing.T, hidden bool) {
	t.Helper()
	point, _ := n.h.newScene(n.h.newMap(n.campaignID, gridColumns), "A clareira", true, 1)
	n.openScene(t, point)
	if _, err := n.master.play.PutOnStage(t.Context(), connect.NewRequest(&playv1.PutOnStageRequest{CampaignId: n.campaignID, CharacterId: n.npcID, Hidden: hidden})); err != nil {
		t.Fatalf("PutOnStage() error = %v", err)
	}
}

// TestNpcCheckPickerMarksTheCreaturesHiddenFromThePlayers: a stage NPC the master hid, and a creature whose token is hidden on the map,
// are listed as hidden; the others are not. A creature of the bestiary has no placement at all.
func TestNpcCheckPickerMarksTheCreaturesHiddenFromThePlayers(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]bool{"shown on the stage": false, "hidden on the stage": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
			n.putOnStage(t, tc)
			c := n.candidate(t, n.npcID)
			if c == nil || c.GetSource() != playv1.NpcCheckSource_NPC_CHECK_SOURCE_STAGE || c.GetHidden() != tc || c.GetMapMeasures() {
				t.Errorf("the stage NPC = %v, want a stage candidate, hidden %v, measured by no map", c, tc)
			}
		})
	}
	t.Run("hidden token", func(t *testing.T) {
		t.Parallel()
		n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
		mapID := n.mapWith(t, gridColumns)
		n.h.placeToken(mapID, n.npcID, 1750, 250)
		n.hideToken(t, mapID, n.npcID)
		if c := n.candidate(t, n.npcID); c == nil || !c.GetHidden() {
			t.Errorf("the creature with a hidden token = %v, want it listed as hidden", c)
		}
	})
	// A creature on the stage with a token on the current map is listed once, as a stage candidate: it is hidden when either hides it.
	for name, tc := range map[string]struct{ stage, token bool }{
		"shown on the stage, hidden token": {false, true},
		"hidden on the stage, shown token": {true, false},
		"shown on the stage, shown token":  {false, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
			n.putOnStage(t, tc.stage)
			mapID := n.mapWith(t, gridColumns)
			n.h.placeToken(mapID, n.npcID, 1750, 250)
			if tc.token {
				n.hideToken(t, mapID, n.npcID)
			}
			c := n.candidate(t, n.npcID)
			if c == nil || c.GetSource() != playv1.NpcCheckSource_NPC_CHECK_SOURCE_STAGE || c.GetHidden() != (tc.stage || tc.token) {
				t.Errorf("the creature on the stage and on the map = %v, want a stage candidate, hidden %v", c, tc.stage || tc.token)
			}
		})
	}
	t.Run("a creature of the bestiary", func(t *testing.T) {
		t.Parallel()
		n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
		res, err := n.master.play.ListNpcCheckPicker(t.Context(), connect.NewRequest(&playv1.ListNpcCheckPickerRequest{CampaignId: n.campaignID, MonsterKey: "monster:wolf"}))
		if err != nil || len(res.Msg.GetCandidates()) != 1 || res.Msg.GetCandidates()[0].GetMapMeasures() || res.Msg.GetCandidates()[0].GetHidden() {
			t.Errorf("the wolf = %v, %v; want one candidate, not hidden and measured by no map", res, err)
		}
	})
}

// TestNpcCheckOfACreatureHiddenOnTheMapAsksNoBard: the creature's token is hidden on the map: no player sees it, so no bard is asked,
// whatever the box says and however near the bard stands; the check settles at once, and nobody reads a question.
func TestNpcCheckOfACreatureHiddenOnTheMapAsksNoBard(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	mapID := n.mapWith(t, gridColumns)
	n.h.placeToken(mapID, n.bri.GetId(), 250, 250)
	n.h.placeToken(mapID, n.lia.GetId(), 250, 750)
	n.h.placeToken(mapID, n.npcID, 750, 250)
	// The positive control: the same table with the token shown asks both bards.
	shown, err := n.roll(t, 15, true)
	if err != nil || !shown.GetHeld() || len(shown.GetCheck().GetAsks()) != 2 {
		t.Fatalf("the creature in sight = %v, %v; want both bards asked", shown, err)
	}
	if _, err := n.master.play.SkipNpcCheckAsks(t.Context(), connect.NewRequest(&playv1.SkipNpcCheckAsksRequest{CampaignId: n.campaignID, HoldId: shown.GetCheck().GetHoldId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("SkipNpcCheckAsks() error = %v", err)
	}
	n.hideToken(t, mapID, n.npcID)
	res, err := n.roll(t, 15, true)
	if err != nil || res.GetHeld() || len(res.GetCheck().GetAsks()) != 0 || !res.GetCheck().GetSettled() || res.GetCheck().GetTotal() != 15 {
		t.Fatalf("the creature hidden on the map = %v, %v; want a check settled at once with nobody asked", res, err)
	}
	for _, u := range []*user{n.bia, n.ana} {
		if got := n.listHeldRolls(t, u); len(got) != 0 {
			t.Errorf("a bard reads %v, want nothing", got)
		}
	}
}

// TestNpcCheckOfACreatureHiddenOnTheMapAsksNoBardWithNoTokenOfTheirs: the creature's own token is read first. Hidden, it opens no question
// to any bard, whatever the box says: not to a bard with no token on the map, and not on a map with no grid, where nothing is measured.
// The positive control of each table is the same creature with its token shown: the box then asks the bards.
func TestNpcCheckOfACreatureHiddenOnTheMapAsksNoBardWithNoTokenOfTheirs(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		columns int
		bards   bool // the bards have a token too
	}{
		"a gridded map, the bards have no token":      {gridColumns, false},
		"a map with no grid, the bards have no token": {0, false},
		"a map with no grid, the bards have a token":  {0, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
			mapID := n.mapWith(t, tc.columns)
			n.h.placeToken(mapID, n.npcID, 1750, 250)
			if tc.bards {
				n.h.placeToken(mapID, n.bri.GetId(), 250, 250)
				n.h.placeToken(mapID, n.lia.GetId(), 250, 750)
			}
			shown, err := n.roll(t, 15, true)
			if err != nil || !shown.GetHeld() || len(shown.GetCheck().GetAsks()) != 2 {
				t.Fatalf("the creature with its token shown = %v, %v; want the box to ask both bards", shown, err)
			}
			if _, err := n.skip(n.master, shown.GetCheck().GetHoldId(), newKey()); err != nil {
				t.Fatalf("SkipNpcCheckAsks() error = %v", err)
			}
			n.hideToken(t, mapID, n.npcID)
			res, err := n.roll(t, 15, true)
			if err != nil || res.GetHeld() || len(res.GetCheck().GetAsks()) != 0 || !res.GetCheck().GetSettled() || res.GetCheck().GetTotal() != 15 {
				t.Fatalf("the creature with its token hidden = %v, %v; want a check settled at once with nobody asked", res, err)
			}
			for _, u := range []*user{n.bia, n.ana} {
				if got := n.listHeldRolls(t, u); len(got) != 0 {
					t.Errorf("a bard reads %v, want nothing", got)
				}
			}
		})
	}
}

// TestNpcCheckPickerBonusesAreThoseOfTheRoll: what the picker shows the master for a check, its bonus and whether the creature has a
// number for it, is what RollNpcCheck then adds to the d20 and reads as modifier_known, for the same key: for a creature with a full
// sheet, for an NPC typed by hand (no number, +0) and for an SRD creature (read from its stat block).
func TestNpcCheckPickerBonusesAreThoseOfTheRoll(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	mage := n.master.createCharacter(t, n.campaignID, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Mago")
	mapID := n.mapWith(t, gridColumns)
	n.h.placeToken(mapID, n.npcID, 1750, 250)
	n.h.placeToken(mapID, mage.GetId(), 1750, 750)
	const wolf = "monster:wolf"
	res, err := n.master.play.ListNpcCheckPicker(t.Context(), connect.NewRequest(&playv1.ListNpcCheckPickerRequest{CampaignId: n.campaignID, MonsterKey: wolf}))
	if err != nil {
		t.Fatalf("ListNpcCheckPicker() error = %v", err)
	}
	for name, tc := range map[string]struct {
		npc       *playv1.RollNpcCheckRequest
		candidate func(*playv1.NpcCheckCandidate) bool
		known     bool // what the creature is expected to have a number for, so the test cannot pass with nothing known anywhere
	}{
		"a creature with a full sheet": {
			&playv1.RollNpcCheckRequest{Npc: &playv1.RollNpcCheckRequest_NpcCharacterId{NpcCharacterId: mage.GetId()}},
			func(c *playv1.NpcCheckCandidate) bool { return c.GetCharacterId() == mage.GetId() }, true,
		},
		"an NPC typed by hand": {
			&playv1.RollNpcCheckRequest{Npc: &playv1.RollNpcCheckRequest_NpcCharacterId{NpcCharacterId: n.npcID}},
			func(c *playv1.NpcCheckCandidate) bool { return c.GetCharacterId() == n.npcID }, false,
		},
		"an SRD creature": {
			&playv1.RollNpcCheckRequest{Npc: &playv1.RollNpcCheckRequest_MonsterKey{MonsterKey: wolf}},
			func(c *playv1.NpcCheckCandidate) bool { return c.GetMonsterKey() == wolf && c.GetCharacterId() == "" }, true,
		},
	} {
		cand := slices.IndexFunc(res.Msg.GetCandidates(), tc.candidate)
		if cand < 0 {
			t.Fatalf("%s: not listed by the picker", name)
		}
		checks := res.Msg.GetCandidates()[cand].GetChecks()
		for _, key := range []string{"ability:int", "ability:cha", "skill:arcana", "skill:perception", "skill:stealth"} {
			i := slices.IndexFunc(checks, func(o *playv1.NpcCheckOption) bool { return o.GetKey() == key })
			if i < 0 {
				t.Fatalf("%s: the picker has no check %s", name, key)
			}
			req := proto.Clone(tc.npc).(*playv1.RollNpcCheckRequest)
			req.CampaignId, req.CheckKey, req.IdempotencyKey = n.campaignID, key, newKey()
			req.Roll = &playv1.RollNpcCheckRequest_D20Face{D20Face: 10}
			rolled, err := n.master.play.RollNpcCheck(t.Context(), connect.NewRequest(req))
			if err != nil {
				t.Fatalf("%s: RollNpcCheck(%s) error = %v", name, key, err)
			}
			if got, want := checks[i].GetBonus(), rolled.Msg.GetCheck().GetD20().GetModifier(); got != want {
				t.Errorf("%s, %s: the picker's bonus = %d, the roll adds %d", name, key, got, want)
			}
			if got, want := checks[i].GetKnown(), rolled.Msg.GetCheck().GetModifierKnown(); got != want {
				t.Errorf("%s, %s: the picker's known = %v, the roll's modifier_known = %v", name, key, got, want)
			}
			if tc.known && !checks[i].GetKnown() {
				t.Errorf("%s, %s: no number, want the creature to have one", name, key)
			}
			if !tc.known && (checks[i].GetKnown() || checks[i].GetBonus() != 0) {
				t.Errorf("%s, %s: bonus %d, known %v; want +0 and unknown", name, key, checks[i].GetBonus(), checks[i].GetKnown())
			}
		}
	}
}

// checkCountingRoster counts the reads a roster is asked for the checks of an SRD creature, per call.
type checkCountingRoster struct {
	CombatRoster
	checkCalls, batchCalls atomic.Int32
}

func (c *checkCountingRoster) CreatureChecks(ctx context.Context, tx pgx.Tx, campaignID, creatureID, monsterKey string, keys []string) ([]link.SceneOption, error) {
	c.batchCalls.Add(1)
	return c.CombatRoster.CreatureChecks(ctx, tx, campaignID, creatureID, monsterKey, keys)
}

func (c *checkCountingRoster) CreatureCheck(ctx context.Context, tx pgx.Tx, campaignID, creatureID, monsterKey, key string) (link.SceneOption, error) {
	c.checkCalls.Add(1)
	return c.CombatRoster.CreatureCheck(ctx, tx, campaignID, creatureID, monsterKey, key)
}

// TestNpcCheckPickerReadsTheChecksOfACreatureOnce: the picker reads the checks of a creature with a monster key in one read, not one for
// each of its twenty-four checks, and the numbers it lists are the same (TestNpcCheckPickerBonusesAreThoseOfTheRoll).
func TestNpcCheckPickerReadsTheChecksOfACreatureOnce(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	counter := &checkCountingRoster{CombatRoster: n.h.svc.roster}
	n.h.svc.roster = counter
	res, err := n.master.play.ListNpcCheckPicker(t.Context(), connect.NewRequest(&playv1.ListNpcCheckPickerRequest{CampaignId: n.campaignID, MonsterKey: "monster:wolf"}))
	if err != nil || len(res.Msg.GetCandidates()) != 1 || len(res.Msg.GetCandidates()[0].GetChecks()) != len(cutting.CheckKeys()) {
		t.Fatalf("the picker with the wolf = %v, %v; want one candidate with all its checks", res, err)
	}
	if singles, batches := counter.checkCalls.Load(), counter.batchCalls.Load(); singles != 0 || batches != 1 {
		t.Errorf("the picker made %d single reads and %d reads of all the checks of the wolf, want one read for the creature", singles, batches)
	}
}
