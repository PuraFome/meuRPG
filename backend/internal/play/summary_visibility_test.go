package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Characterization of GetSessionSummary's visibility rules (summary.go): the
// reserve, a player with two characters and a player with none.

// endedSummaryTable plays one combat where Toren kills the goblin with a critical and
// ends the session.
func endedSummaryTable(t *testing.T) (*armed, *playv1.GameSession) {
	t.Helper()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId(), Hidden: new(false)}},
		npcRolls: []int{1},
		players:  map[string]int32{"Toren": 20, "Pensantus": 15, "Brisa": 10},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {11, 3}},
	})
	a.h.roller.queue(20, 8, 8)
	res := a.mustAttack(t, a.master, e, "Toren", battleaxe, "Goblin", inAppRoll)
	dmg := a.mustDamage(t, a.master, e, res.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
	if dmg.GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED {
		if _, err := a.settle(t, a.master, e, dmg.GetId(), true); err != nil {
			t.Fatalf("ApplyPendingDamage() error = %v", err)
		}
	}
	a.endEncounter(t, e)
	listed, err := a.master.play.ListGameSessions(t.Context(), connect.NewRequest(&playv1.ListGameSessionsRequest{CampaignId: a.campaignID}))
	if err != nil {
		t.Fatal(err)
	}
	ended := a.master.end(t, listed.Msg.GetGameSessions()[0])

	return a, ended
}

func TestSummary_APlayerWithSeveralOrNoCharacters(t *testing.T) {
	t.Parallel()
	a, ended := endedSummaryTable(t)
	if _, err := a.master.characters.MarkCharacterDead(t.Context(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: a.campaignID, CharacterId: a.toren.GetId()})); err != nil {
		t.Fatalf("MarkCharacterDead() error = %v", err)
	}
	// Brisa now belongs to Toren's player too; Pensantus belongs to nobody.
	for _, q := range []struct{ user, id string }{{a.caio.id, a.bri.GetId()}, {"", a.pens.GetId()}} {
		var uid *string
		if q.user != "" {
			uid = &q.user
		}
		if _, err := a.h.pool.Exec(t.Context(), `UPDATE characters SET player_user_id = $1 WHERE id = $2`, uid, q.id); err != nil {
			t.Fatalf("move the character: %v", err)
		}
	}
	sum, err := a.summary(t, a.caio, ended.GetId())
	if err != nil {
		t.Fatalf("GetSessionSummary() error = %v", err)
	}
	// The result is the sum of the characters the player played, named after the latest one.
	if mine := sum.GetMine(); mine.GetHighlights().GetDamageDealt() != 7 || mine.GetHighlights().GetFinalBlows() != 1 || mine.GetHighlights().GetCriticalHits() != 1 || mine.GetHighlights().GetName() != "Brisa" {
		t.Errorf("Toren's player's result = %v, want Toren's numbers under Brisa's name", mine)
	}
	none, err := a.summary(t, a.ana, ended.GetId())
	if err != nil {
		t.Fatalf("GetSessionSummary() error = %v", err)
	}
	if none.GetMine() != nil {
		t.Errorf("a player with no character got a result: %v", none.GetMine())
	}
	if got := summaryCategories(none); !slices.Equal(got, []string{"MOST_DAMAGE 7: Toren", "FINAL_BLOW 1: Toren", "CRITICAL_HITS 1: Toren"}) {
		t.Errorf("categories = %q, want Toren's", got)
	}
}

func TestSummary_ReservedCharactersAreInvisibleToPlayers(t *testing.T) {
	t.Parallel()
	a, ended := endedSummaryTable(t)
	// Toren goes back to the reserve: the players do not see him in the sessions he played, the master does.
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE characters SET reserved = true, player_user_id = NULL WHERE id = $1`, a.toren.GetId()); err != nil {
		t.Fatalf("reserve the character: %v", err)
	}
	for who, u := range map[string]*user{"Toren's old player": a.caio, "Pensantus's player": a.ana} {
		got, err := a.summary(t, u, ended.GetId())
		if err != nil {
			t.Fatalf("%s: GetSessionSummary() error = %v", who, err)
		}
		if cats := summaryCategories(got); len(cats) != 0 {
			t.Errorf("%s: categories = %q, want none: the only numbers are the reserved character's", who, cats)
		}
		if own := got.GetMine().GetHighlights().GetName(); (who == "Pensantus's player") != (own == "Pensantus") || got.GetMine().GetHighlights().GetDamageDealt() != 0 {
			t.Errorf("%s: result = %v, want Pensantus's own empty result for her player and none for the old player of the reserved Toren", who, got.GetMine())
		}
	}
	m, err := a.summary(t, a.master, ended.GetId())
	if err != nil {
		t.Fatalf("GetSessionSummary() error = %v", err)
	}
	if len(m.GetPlayers()) == 0 || m.GetPlayers()[0].GetHighlights().GetName() != "Toren" || len(m.GetCategories()) == 0 {
		t.Errorf("the master's summary = %v, want the reserved Toren still in it", m)
	}
}
