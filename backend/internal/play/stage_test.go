package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Characterization of PutOnStage's refusals (stage.go) that need the play
// module's own combat: the NPC the app keeps for a monster is not a character
// the master can put on the stage. The rest of the stage is tested with the
// scenes (internal/maps/stage_test.go).

func (a *armed) putOnStage(t *testing.T, characterID string) error {
	t.Helper()
	_, err := a.master.play.PutOnStage(t.Context(), connect.NewRequest(&playv1.PutOnStageRequest{CampaignId: a.campaignID, CharacterId: characterID}))
	return err
}

func TestPutOnStage_RefusalsWithTheirMessages(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	var monsterNPC string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT character_id FROM combatants WHERE encounter_id = $1 AND label = $2`, c.e.GetId(), c.hob).Scan(&monsterNPC); err != nil {
		t.Fatalf("find the monster's NPC: %v", err)
	}
	a.endEncounter(t, c.refresh(t))

	wantRefusal(t, "no scene", a.putOnStage(t, a.goblin.GetId()), connect.CodeFailedPrecondition, "no scene is open, so there is no stage")
	point, _ := a.h.newScene(a.mapID, "A clareira", true, 1)
	a.openScene(t, point)
	wantRefusal(t, "a monster's NPC", a.putOnStage(t, monsterNPC), connect.CodeInvalidArgument, "a monster's NPC is not a character the master can put on the stage")
	wantRefusal(t, "a player's character", a.putOnStage(t, a.toren.GetId()), connect.CodeNotFound, "character not found")
	wantRefusal(t, "not an ID", a.putOnStage(t, "nope"), connect.CodeNotFound, "character not found")
	wantRefusal(t, "an unknown character", a.putOnStage(t, newKey()), connect.CodeNotFound, "character not found")
	if err := a.putOnStage(t, a.goblin.GetId()); err != nil {
		t.Fatalf("PutOnStage(goblin) error = %v", err)
	}
	if err := a.putOnStage(t, a.goblin.GetId()); err != nil {
		t.Fatalf("PutOnStage(goblin) again error = %v", err)
	}
	// A player may not.
	_, err := a.caio.play.PutOnStage(t.Context(), connect.NewRequest(&playv1.PutOnStageRequest{CampaignId: a.campaignID, CharacterId: a.goblin.GetId()}))
	wantRefusal(t, "a player", err, connect.CodePermissionDenied, "")
}
