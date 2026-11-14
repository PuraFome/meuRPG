package leaktest

import (
	"testing"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// The Inspiration spent on a check outside the combat's rolls (RN-10, RN-43): its source "Inspiração" is the roll's own, so the
// roller and the master read it and no other player, and no one outside the campaign, does. Each path gets a world of its own:
// Caio's Toren is granted the Inspiration, spends it on the path, and then every read of the matrix is asked of everyone.
// The positive controls are Caio's own answer to the spend and the master's reads; the canary is the word itself, which no
// other text of the fixture holds.

const favorSourceWord = "Inspiração"

// toTurnOf ends turns until it is the character's.
func (w *world) toTurnOf(characterID string) {
	w.t.Helper()
	ctx := w.t.Context()
	for range 60 {
		cur := must(w.master.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() == w.combatantOf(cur, characterID).GetId() {
			return
		}
		must(w.master.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: w.campaign, EncounterId: cur.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound()})))
	}
	w.t.Fatalf("it never became the turn of %s", characterID)
}

type favorPath struct {
	name string
	call func(w *world, key string) (procedure string, req proto.Message)
}

func favorPaths() []favorPath {
	return []favorPath{
		{"a scene check", func(w *world, key string) (string, proto.Message) {
			return playv1connect.PlayServiceRollSceneCheckProcedure, &playv1.RollSceneCheckRequest{
				CampaignId: w.campaign, ActionId: w.openSceneAction(), IdempotencyKey: key, SpendFavor: true,
				Roll: &playv1.RollSceneCheckRequest_RollInApp{RollInApp: true},
			}
		}},
		{"a group check", func(w *world, key string) (string, proto.Message) {
			open := must(w.master.contests.GetGroupCheck(w.t.Context(), rq(&playv1.GetGroupCheckRequest{CampaignId: w.campaign}))).GetGroupCheck()
			must(w.master.contests.CloseGroupCheck(w.t.Context(), rq(&playv1.CloseGroupCheckRequest{CampaignId: w.campaign, GroupCheckId: open.GetId(), IdempotencyKey: newKey()}))) // the fixture's, which Caio rolled already
			asked := must(w.master.contests.RequestGroupCheck(w.t.Context(), rq(&playv1.RequestGroupCheckRequest{
				CampaignId: w.campaign, IdempotencyKey: newKey(), SkillKey: "skill:stealth", Dc: groupCheckDC, CharacterIds: []string{w.toren.GetId()},
			}))).GetGroupCheck()
			return playv1connect.ContestServiceRollGroupCheckProcedure, &playv1.RollGroupCheckRequest{
				CampaignId: w.campaign, IdempotencyKey: key, GroupCheckId: asked.GetId(), Roll: appRoll(), SpendFavor: true,
			}
		}},
		{"a hint try", func(w *world, key string) (string, proto.Message) {
			return playv1connect.PuzzleServiceTryPuzzleHintProcedure, &playv1.TryPuzzleHintRequest{
				CampaignId: w.campaign, PuzzleId: w.puzzles["riddle-shown"].GetId(), IdempotencyKey: key, SpendFavor: true,
				Roll: &playv1.TryPuzzleHintRequest_RollInApp{RollInApp: true},
			}
		}},
		{"a trap search", func(w *world, key string) (string, proto.Message) {
			return playv1connect.PlayServiceSearchForTrapsProcedure, &playv1.SearchForTrapsRequest{
				CampaignId: w.campaign, IdempotencyKey: key, SpendFavor: true, Skill: playv1.TrapSearchSkill_TRAP_SEARCH_SKILL_INVESTIGATION,
				Roll: &playv1.SearchForTrapsRequest_RollInApp{RollInApp: true},
			}
		}},
		{"a contest answer", func(w *world, key string) (string, proto.Message) {
			w.toTurnOf(w.hiddenNPC.GetId())
			started := must(w.master.contests.StartContest(w.t.Context(), rq(&playv1.StartContestRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), Roll: appRoll(),
				InitiatorId: w.combatant(w.hiddenNPC).GetId(), TargetId: w.combatant(w.toren).GetId(), Purpose: playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE,
			})))
			return playv1connect.ContestServiceRespondContestProcedure, &playv1.RespondContestRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: key, ContestId: started.GetContest().GetId(), SpendFavor: true,
				Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, Roll: appRoll(),
			}
		}},
		{"a hide", func(w *world, key string) (string, proto.Message) {
			w.toTurnOf(w.toren.GetId())
			return playv1connect.ContestServiceHideProcedure, &playv1.HideRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: key, SpendFavor: true, Roll: appRoll(),
				CombatantId: w.combatant(w.toren).GetId(),
			}
		}},
	}
}

func TestFavorSpentOnACheckReachesOnlyTheRollerAndTheMaster(t *testing.T) {
	for _, p := range favorPaths() {
		t.Run(p.name, func(t *testing.T) {
			w := newWorld(t)
			word := w.secrets.add(&canary{needle: favorSourceWord, kind: "favor-source", readers: names([]*person{w.caio})})
			w.grantFavor(w.toren.GetId())
			w.hints.Settle(t)
			procedure, req := p.call(w, newKey())
			own := w.caio.call(procedure, req)
			if !own.ok() {
				t.Fatalf("Caio's spend on %s was refused: HTTP %d %s", p.name, own.status, own.body)
			}
			got := newAnswers()
			got.keep(w.caio, own)
			w.hints.Settle(t)
			for _, r := range reads {
				w.runRead(t, r, got)
			}
			// The positive controls: the roller read it in the answer to the call, the master in the reads.
			if !got.holds(w.caio, word) {
				t.Errorf("Caio's answer to %s has no %q: the control that the spend shows is missing", p.name, favorSourceWord)
			}
			if !got.holds(w.master, word) {
				t.Errorf("the master reads no %q of %s in any read: the control that it shows to the master is missing", favorSourceWord, p.name)
			}
			for _, other := range []*person{w.ana, w.pending, w.stranger} {
				if got.holds(other, word) {
					t.Errorf("%s reads %q of %s", other.name, favorSourceWord, p.name)
				}
			}
		})
	}
}
