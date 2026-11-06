package leaktest

import (
	"bytes"
	"net/http"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// The master's undo is refused when something changed a combatant's hit points after the action
// (UNDO_TARGET_CHANGED). The refusal names the combatant for the master and sends no number; a player
// cannot ask for the undo, and what the players read afterwards is the same as before: no hit points
// of an NPC, which are the master's (RN-10, RN-20). The master's refusal is the positive control.
func TestARefusedUndoSendsTheNumbersOfTheNPCToNoOne(t *testing.T) {
	w := newWorld(t)
	hound := w.monsterOnTurn("monster:hell-hound", w.secrets.public("undo-name"), false)
	master := w.master
	must(master.combat.AdjustCombatantHitPoints(t.Context(), rq(&playv1.AdjustCombatantHitPointsRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: hound.GetId(), IdempotencyKey: newKey(),
		Change: &playv1.AdjustCombatantHitPointsRequest_Damage{Damage: 1},
	})))
	const changedTemp = 4217 // what the change after the action wrote: a number no other answer matches
	if _, err := w.stack.pool.Exec(t.Context(), `UPDATE combatants SET hp_temp = $2 WHERE id = $1`, hound.GetId(), changedTemp); err != nil {
		t.Fatalf("change the hit points: %v", err)
	}
	log := must(master.combat.ListCombatLog(t.Context(), rq(&playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()})))
	undo := &playv1.UndoLastActionRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), ExpectedEventId: log.GetUndoableEventId(), IdempotencyKey: newKey()}
	needle := []byte("4217")

	for _, p := range []*person{w.ana, w.caio} {
		r := p.call(playv1connect.CombatServiceUndoLastActionProcedure, undo)
		if r.status != http.StatusForbidden {
			t.Errorf("%s: the undo answered %d %s, want 403", p.name, r.status, r.body)
		}
		if bytes.Contains(r.body, needle) {
			t.Errorf("%s: the refusal carries the hit points: %s", p.name, r.body)
		}
	}
	refused := master.call(playv1connect.CombatServiceUndoLastActionProcedure, undo)
	if refused.status != http.StatusBadRequest || !bytes.Contains(refused.body, []byte("UNDO_TARGET_CHANGED")) || !bytes.Contains(refused.body, []byte(hound.GetId())) {
		t.Errorf("master: the refusal is %d %s, want UNDO_TARGET_CHANGED naming the hound", refused.status, refused.body)
	}
	if bytes.Contains(refused.body, needle) {
		t.Errorf("the master's refusal carries the hit points: %s", refused.body)
	}
	for _, p := range []*person{w.ana, w.caio} {
		for _, r := range w.readsOf(p) {
			if bytes.Contains(r.body, needle) {
				t.Errorf("%s reads the hound's hit points after the refused undo: %s", p.name, r.body)
			}
		}
	}
}
