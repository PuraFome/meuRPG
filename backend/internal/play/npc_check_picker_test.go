package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The picker of the master's check says, for each creature, what the app needs to know before it asks the master anything: whether the
// map measures the creature against a bard (so the checkbox "Os bardos veem a criatura e ela os ouve" is not asked), and whether the
// creature is hidden from the players (so the checkbox starts unticked).

// mapWith makes the table's map the session's current one and returns its id.
func (n *npcTable) mapWith(t *testing.T) string {
	t.Helper()
	mapID := n.h.newMap(n.campaignID, gridColumns)
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
// asked are both on the session's current map; with only the stage, only a token of the creature, a bard that would not be asked, or a
// creature of the bestiary, the checkbox decides.
func TestNpcCheckPickerSaysWhetherTheMapMeasures(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		ask    charactersv1.CuttingWordsAsk
		tokens func(n *npcTable, mapID string)
		stage  bool
		want   bool
	}{
		"both tokens on the map": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, func(n *npcTable, m string) {
			n.h.placeToken(m, n.bri.GetId(), 250, 250)
			n.h.placeToken(m, n.npcID, 1750, 250)
		}, false, true},
		"the token of a bard that is not asked": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ONLY_ATTACKS, func(n *npcTable, m string) {
			n.h.placeToken(m, n.bri.GetId(), 250, 250)
			n.h.placeToken(m, n.npcID, 1750, 250)
		}, false, false},
		"the token of a character that is no bard": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, func(n *npcTable, m string) {
			n.h.placeToken(m, n.toren.GetId(), 250, 250)
			n.h.placeToken(m, n.npcID, 1750, 250)
		}, false, false},
		"no token of a bard": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, func(n *npcTable, m string) {
			n.h.placeToken(m, n.npcID, 1750, 250)
		}, false, false},
		"no token of the creature, on the stage": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, func(n *npcTable, m string) {
			n.h.placeToken(m, n.bri.GetId(), 250, 250)
		}, true, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, tc.ask)
			tc.tokens(n, n.mapWith(t))
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
		mapID := n.mapWith(t)
		n.h.placeToken(mapID, n.npcID, 1750, 250)
		n.hideToken(t, mapID, n.npcID)
		if c := n.candidate(t, n.npcID); c == nil || !c.GetHidden() {
			t.Errorf("the creature with a hidden token = %v, want it listed as hidden", c)
		}
	})
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
	mapID := n.mapWith(t)
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
