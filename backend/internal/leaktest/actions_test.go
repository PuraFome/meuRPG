package leaktest

import (
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1/mapsv1connect"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// actions are the calls a player makes that answer with something: a roll, a search, a move
// of a puzzle. They run before the reads (the master sees what the players did, and the
// reads then hold it), through the same runner as the reads: everyone is asked, and only who
// may act gets an answer. Each answer is checked like a read, so a roll that tells a DC,
// a search that names a trap nobody knew, or a refusal that says what the thing is, fails.
var actions = []read{
	{
		procedure: playv1connect.PlayServiceRollSceneCheckProcedure, allow: members, notMaster: true,
		why: "the roll is the player's; the DC and whether it passed stay the master's while \"Mostrar a CD\" is off",
		req: func(w *world) proto.Message {
			a := w.openSceneAction()
			return &playv1.RollSceneCheckRequest{CampaignId: w.campaign, ActionId: a, IdempotencyKey: newKey(), Roll: &playv1.RollSceneCheckRequest_D20Face{D20Face: 12}}
		},
	},
	{
		procedure: playv1connect.PlayServiceSearchForTrapsProcedure, label: "Investigation", allow: onlyCaio, notMaster: true,
		why: "a search that finds nothing reads like a failed roll: it never says there is a trap (Ana's character is down in the combat: she cannot search)",
		req: func(w *world) proto.Message {
			return &playv1.SearchForTrapsRequest{CampaignId: w.campaign, Skill: playv1.TrapSearchSkill_TRAP_SEARCH_SKILL_INVESTIGATION, IdempotencyKey: newKey(), Roll: &playv1.SearchForTrapsRequest_D20Face{D20Face: 20}}
		},
	},
	{
		procedure: playv1connect.PuzzleServiceTryPuzzleHintProcedure, allow: members, notMaster: true,
		why: "the hint is won by a check against a DC the master set: a failed try says \"failed\", never the DC or the hint",
		req: func(w *world) proto.Message {
			return &playv1.TryPuzzleHintRequest{CampaignId: w.campaign, PuzzleId: w.puzzles["riddle-shown"].GetId(), IdempotencyKey: newKey(), Roll: &playv1.TryPuzzleHintRequest_D20Face{D20Face: 1}}
		},
	},
	{
		procedure: playv1connect.PuzzleServiceMakePuzzleMoveProcedure, allow: members, notMaster: true,
		why: "a move answers with the state of the wheels, never the solution",
		req: func(w *world) proto.Message {
			return &playv1.MakePuzzleMoveRequest{
				CampaignId: w.campaign, PuzzleId: w.puzzles["shown"].GetId(), IdempotencyKey: newKey(),
				Move: &playv1.PuzzleMove{Kind: &playv1.PuzzleMove_Lock{Lock: &playv1.LockMove{Wheel: 1, Delta: 1}}},
			}
		},
	},
	{
		procedure: playv1connect.PuzzleServiceMakePuzzleMoveProcedure, label: "a puzzle never shown", allow: masterOnlyRead, notMaster: true,
		why: "a puzzle that is not shown does not exist for a player",
		req: func(w *world) proto.Message {
			return &playv1.MakePuzzleMoveRequest{
				CampaignId: w.campaign, PuzzleId: w.puzzles["riddle-hidden"].GetId(), IdempotencyKey: newKey(),
				Move: &playv1.PuzzleMove{Kind: &playv1.PuzzleMove_Riddle{Riddle: &playv1.RiddleMove{Answer: "qualquer"}}},
			}
		},
	},
	{
		procedure: mapsv1connect.MapServiceSetCarriedLightProcedure, label: "Ana's character", allow: onlyAna,
		why: "the light a character carries is told to its player and the master only",
		req: func(w *world) proto.Message {
			return &mapsv1.SetCarriedLightRequest{CampaignId: w.campaign, MapId: w.fogMap, CharacterId: w.pens.GetId(), LightKey: "light:torch"}
		},
	},
}

// openSceneAction is the id of the open scene's first action, which a player rolls.
func (w *world) openSceneAction() string {
	r := must(w.master.play.GetOpenScene(w.t.Context(), rq(&playv1.GetOpenSceneRequest{CampaignId: w.campaign})))
	return r.GetScene().GetActions()[0].GetId()
}
