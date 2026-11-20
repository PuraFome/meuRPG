package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A check that waits is read as things stand when its question is read or answered, not as they stood when it was rolled: the creature
// that was hidden since, and the bard that is no longer in play.

// twoAsked rolls a check the two bards are asked about: Brisa (Bia's) first, then Lia (Ana's).
func (n *npcTable) twoAsked(t *testing.T) (hold string, first, second *user) {
	t.Helper()
	res, err := n.roll(t, 15, true)
	if err != nil || !res.GetHeld() || len(res.GetCheck().GetAsks()) != 2 {
		t.Fatalf("RollNpcCheck() = %v, %v; want a held check of two asks", res, err)
	}
	first, second = n.firstAsked(res), n.ana
	if first == n.ana {
		second = n.bia
	}
	return res.GetCheck().GetHoldId(), first, second
}

// changed says the stream heard that the held rolls changed.
func (n *npcTable) changed(t *testing.T, h heardBy) bool {
	t.Helper()
	for _, ev := range h.heard(t, n.h) {
		if ev.GetHeldRollsChanged() != nil {
			return true
		}
	}
	return false
}

// label is what the bard's offer calls the creature, "" when the bard reads no offer.
func (n *npcTable) label(t *testing.T, u *user) string {
	t.Helper()
	offers := n.listHeldRolls(t, u)
	if len(offers) == 0 {
		return ""
	}
	if len(offers) != 1 {
		t.Fatalf("the bard reads %d offers, want at most 1", len(offers))
	}
	return offers[0].GetRollerLabel()
}

// withTheCreatureOnTheStage opens a scene to the whole table and puts the Barão on its stage, shown.
func (n *npcTable) withTheCreatureOnTheStage(t *testing.T) {
	t.Helper()
	n.putOnStage(t, false)
}

func (n *npcTable) hideOnStage(t *testing.T) {
	t.Helper()
	if _, err := n.master.play.SetStageNpcHidden(t.Context(), connect.NewRequest(&playv1.SetStageNpcHiddenRequest{CampaignId: n.campaignID, CharacterId: n.npcID, Hidden: true})); err != nil {
		t.Fatalf("SetStageNpcHidden() error = %v", err)
	}
}

// withTheCreatureOnTheMap puts the Barão's token on the map, away from the bards (who have none), and returns the map.
func (n *npcTable) withTheCreatureOnTheMap(t *testing.T) string {
	t.Helper()
	mapID := n.mapWith(t, gridColumns)
	n.h.placeToken(mapID, n.npcID, 1750, 250)
	return mapID
}

// TestNpcCheckOfACreatureHiddenOnTheStageNowIsAnUnnamedCreature: the offer is built when it is read, and the creature the master hid on the
// stage since the roll reads "uma criatura" to the bard asked now and to the next one, after the bard before it answered; before, the
// bards read its name.
func TestNpcCheckOfACreatureHiddenOnTheStageNowIsAnUnnamedCreature(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	n.withTheCreatureOnTheStage(t)
	hold, first, second := n.twoAsked(t)
	if got := n.label(t, first); got != "Barão" {
		t.Fatalf("the first bard reads %q before the creature is hidden, want its name", got)
	}
	n.hideOnStage(t)
	if got := n.label(t, first); got != unnamedCreature {
		t.Errorf("the bard asked now reads %q after the creature was hidden on the stage, want %q", got, unnamedCreature)
	}
	if _, err := n.pass(first, hold, newKey()); err != nil {
		t.Fatalf("the first bard's pass error = %v", err)
	}
	if got := n.label(t, second); got != unnamedCreature {
		t.Errorf("the next bard reads %q after the creature was hidden on the stage, want %q", got, unnamedCreature)
	}
}

// TestNpcCheckOfACreatureHiddenOnTheMapNowSettlesAsASkip: the master hid the creature's token on the map after the roll: the check is
// settled as the skip settles it by the next read or answer that finds it (the hide is a write of the maps module, and the check's own
// paths settle it), so the bard asked now and the next one read nothing, whichever finds it first.
func TestNpcCheckOfACreatureHiddenOnTheMapNowSettlesAsASkip(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		finds func(n *npcTable, hold string, first, second *user) // what finds the hidden token first
	}{
		"the bard asked now reads":   {func(n *npcTable, _ string, first, _ *user) { n.mustNothing(first) }},
		"the next bard reads":        {func(n *npcTable, _ string, _, second *user) { n.mustNothing(second) }},
		"the master reads the check": {func(n *npcTable, _ string, _, _ *user) { n.mustLatest(n.h.t) }},
		"the bard asked now answers": {func(n *npcTable, hold string, first, _ *user) {
			n.mustRefuse(first, hold, connect.CodeNotFound)
		}},
		"the next bard answers": {func(n *npcTable, hold string, _, second *user) {
			n.mustRefuse(second, hold, connect.CodeNotFound)
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
			mapID := n.withTheCreatureOnTheMap(t)
			hold, first, second := n.twoAsked(t)
			if got := n.label(t, first); got == "" {
				t.Fatal("the first bard reads no offer before the token is hidden")
			}
			master, firstEars, secondEars := listen(t, n.h, n.campaignID, n.master.id, true), listen(t, n.h, n.campaignID, first.id, false), listen(t, n.h, n.campaignID, second.id, false)
			before := n.uses(t, n.charOf(first))
			n.hideToken(t, mapID, n.npcID)
			tc.finds(n, hold, first, second)
			if row, ok := n.hold(t, hold); !ok || !row.answered {
				t.Fatalf("the hold = %+v, %v after the token was found hidden; want it settled", row, ok)
			}
			if got := n.resolvedEvents(t); len(got) != 1 || !got[0].skipped || got[0].total != 15 || got[0].actor != n.master.id {
				t.Errorf("the resolved events = %+v, want one skip of the held 15, in the master's name", got)
			}
			for _, u := range []*user{first, second} {
				if got := n.listHeldRolls(t, u); len(got) != 0 {
					t.Errorf("a bard reads %v after the token was hidden, want nothing", got)
				}
			}
			if got := n.mustLatest(t); !got.GetSettled() || got.GetTotal() != 15 || got.GetAsks()[0].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_CLOSED {
				t.Errorf("GetNpcCheck() = %v, want it settled at 15 with the asks closed", got)
			}
			if after := n.uses(t, n.charOf(first)); after != before {
				t.Errorf("the first bard's uses %d -> %d, want none spent", before, after)
			}
			if m, f, s := n.changed(t, master), n.changed(t, firstEars), n.changed(t, secondEars); !m || !f || s {
				t.Errorf("the master, the bard asked now and the next bard hear the change as master %v, first %v, second %v; want true, true, false", m, f, s)
			}
		})
	}
}

// mustNothing is a read of the bard's offers that must find no offer.
func (n *npcTable) mustNothing(u *user) {
	n.h.t.Helper()
	if got := n.listHeldRolls(n.h.t, u); len(got) != 0 {
		n.h.t.Errorf("the bard reads %v, want nothing", got)
	}
}

// mustRefuse is a pass of the bard that must be refused with the code.
func (n *npcTable) mustRefuse(u *user, hold string, code connect.Code) {
	n.h.t.Helper()
	if _, err := n.pass(u, hold, newKey()); connect.CodeOf(err) != code {
		n.h.t.Errorf("the bard's pass error = %v, want %v", err, code)
	}
}

// outOfPlay takes a bard's character out of play, as the master would: into the reserve, dead, deleted from the reserve, or without a player.
var outOfPlay = map[string]func(t *testing.T, n *npcTable, c *charactersv1.Character){
	"in the reserve": func(t *testing.T, n *npcTable, c *charactersv1.Character) {
		t.Helper()
		n.reserve(t, c)
	},
	"dead": func(t *testing.T, n *npcTable, c *charactersv1.Character) {
		t.Helper()
		if _, err := n.master.characters.MarkCharacterDead(t.Context(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: n.campaignID, CharacterId: c.GetId()})); err != nil {
			t.Fatalf("MarkCharacterDead() error = %v", err)
		}
	},
	"deleted": func(t *testing.T, n *npcTable, c *charactersv1.Character) {
		t.Helper()
		n.reserve(t, c)
		if _, err := n.master.characters.DeleteReservedCharacter(t.Context(), connect.NewRequest(&charactersv1.DeleteReservedCharacterRequest{CampaignId: n.campaignID, CharacterId: c.GetId()})); err != nil {
			t.Fatalf("DeleteReservedCharacter() error = %v", err)
		}
	},
	"without a player": func(t *testing.T, n *npcTable, c *charactersv1.Character) {
		t.Helper()
		if _, err := n.h.pool.Exec(t.Context(), `UPDATE characters SET player_user_id = NULL WHERE id = $1`, c.GetId()); err != nil {
			t.Fatalf("take the player off the character: %v", err)
		}
	},
}

// reserve gives a character back to the reserve: no owner, out of the party.
func (n *npcTable) reserve(t *testing.T, c *charactersv1.Character) {
	t.Helper()
	if _, err := n.h.pool.Exec(t.Context(), `UPDATE characters SET reserved = true, player_user_id = NULL WHERE id = $1`, c.GetId()); err != nil {
		t.Fatalf("give the character back to the reserve: %v", err)
	}
}

// TestNpcCheckLetsPassTheBardNoLongerInPlay: the bard asked now was put in the reserve, died, was deleted or lost its player: the next read
// or answer that finds it closes its question as a pass and asks the next bard, who and the master are told; a check whose last bard is gone
// settles as the last answer settles it, not as a skip.
func TestNpcCheckLetsPassTheBardNoLongerInPlay(t *testing.T) {
	t.Parallel()
	for name, out := range outOfPlay {
		for how, finds := range map[string]func(n *npcTable, hold string, second *user){
			"the next bard reads":    func(n *npcTable, _ string, second *user) { n.mustOne(second) },
			"the next bard answers":  func(n *npcTable, hold string, second *user) { n.mustCut(second, hold) },
			"the master reads check": func(n *npcTable, _ string, _ *user) { n.mustLatest(n.h.t) },
		} {
			t.Run(name+"/"+how, func(t *testing.T) {
				t.Parallel()
				n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
				hold, first, second := n.twoAsked(t)
				master, nextEars := listen(t, n.h, n.campaignID, n.master.id, true), listen(t, n.h, n.campaignID, second.id, false)
				out(t, n, n.charOf(first))
				finds(n, hold, second)
				got := n.mustLatest(t)
				if got.GetSettled() && how == "the next bard answers" {
					if got.GetAsks()[0].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_PASSED || got.GetTotal() != 12 || got.GetCut() != 3 {
						t.Errorf("the settled check = %v, want the first bard passed and a 3 cut off the 15", got)
					}
					return
				}
				if got.GetSettled() || got.GetAsks()[0].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_PASSED || got.GetAsks()[1].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_WAITING {
					t.Errorf("GetNpcCheck() = %v, want the first bard passed and the second asked, waiting", got)
				}
				if len(n.listHeldRolls(t, second)) != 1 {
					t.Error("the next bard reads no question")
				}
				if !n.changed(t, master) || !n.changed(t, nextEars) {
					t.Error("the master and the next bard were not told that the question moved")
				}
			})
		}
	}
	t.Run("the last bard", func(t *testing.T) {
		t.Parallel()
		n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
		hold, first, second := n.twoAsked(t)
		if _, err := n.answerAs(first, hold, newKey(), typedCut(4)); err != nil {
			t.Fatalf("the first bard's cut error = %v", err)
		}
		outOfPlay["dead"](t, n, n.charOf(second))
		got := n.mustLatest(t)
		if !got.GetSettled() || got.GetTotal() != 11 || got.GetCut() != 4 || got.GetAsks()[1].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_PASSED {
			t.Errorf("GetNpcCheck() = %v, want it settled at 11 with the last bard passed", got)
		}
		if ev := n.resolvedEvents(t); len(ev) != 1 || ev[0].skipped || ev[0].total != 11 {
			t.Errorf("the resolved events = %+v, want one check settled at 11, not skipped", ev)
		}
	})
	t.Run("every bard", func(t *testing.T) {
		t.Parallel()
		n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
		_, first, second := n.twoAsked(t)
		outOfPlay["dead"](t, n, n.charOf(first))
		outOfPlay["without a player"](t, n, n.charOf(second))
		got := n.mustLatest(t)
		if !got.GetSettled() || got.GetTotal() != 15 || got.GetAsks()[0].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_PASSED || got.GetAsks()[1].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_PASSED {
			t.Errorf("GetNpcCheck() = %v, want it settled at 15 with both bards passed", got)
		}
	})
}

// mustOne is a read of the bard's offers that must find one.
func (n *npcTable) mustOne(u *user) {
	n.h.t.Helper()
	if got := n.listHeldRolls(n.h.t, u); len(got) != 1 {
		n.h.t.Errorf("the bard reads %d offers, want 1", len(got))
	}
}

// mustCut is a cut of 3 by the bard that must be taken.
func (n *npcTable) mustCut(u *user, hold string) {
	n.h.t.Helper()
	if _, err := n.answerAs(u, hold, newKey(), typedCut(3)); err != nil {
		n.h.t.Errorf("the bard's cut error = %v", err)
	}
}
