package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Who answers a held roll is checked before anything about the roll is said: a caller who may not answer
// it is refused whatever the state of the roll, so that a hold id tells nothing to whoever it does not belong
// to (RN-10).

// TestWhoMayNotAnswerIsRefusedBeforeTheStateIsSaid: the die and the Lucky question of a weapon attack that another
// answer settled already are refused to another player with permission_denied, not with the state of the roll.
func TestWhoMayNotAnswerIsRefusedBeforeTheStateIsSaid(t *testing.T) {
	t.Parallel()
	t.Run("the die", func(t *testing.T) {
		t.Parallel()
		a := newLuckyTable(t)
		e := a.luckyInspired(t)
		held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(14))
		if err != nil || held.GetInspirationOffer().GetHoldId() == "" {
			t.Fatalf("Pipo's attack = %v, %v; want the question about the die", held, err)
		}
		hold := held.GetInspirationOffer().GetHoldId()
		if _, err := a.answer(t, a.caio, e, hold, keepTheDie); err != nil {
			t.Fatalf("the player's answer: error = %v", err)
		}
		for name, edit := range map[string]func(*playv1.AnswerBardicInspirationRequest){"keeping it": keepTheDie, "using it": useDieInApp} {
			_, err := a.answer(t, a.ana, e, hold, edit)
			wantCode(t, "another player's answer, "+name+", to a roll answered already", err, connect.CodePermissionDenied)
		}
	})
	t.Run("Lucky", func(t *testing.T) {
		t.Parallel()
		a := newLuckyTable(t)
		e := a.luckyFight(t)
		held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(1))
		if err != nil || held.GetLuckyOffer().GetHoldId() == "" {
			t.Fatalf("Pipo's attack = %v, %v; want Lucky's question", held, err)
		}
		hold := held.GetLuckyOffer().GetHoldId()
		if _, err := a.answerLucky(t, a.caio, e, hold, keepTheOne); err != nil {
			t.Fatalf("the player's answer: error = %v", err)
		}
		for name, edit := range map[string]func(*playv1.AnswerLuckyRequest){"keeping the 1": keepTheOne, "rolling again": rerollInApp} {
			_, err := a.answerLucky(t, a.ana, e, hold, edit)
			wantCode(t, "another player's answer, "+name+", to a roll answered already", err, connect.CodePermissionDenied)
		}
	})
	t.Run("a roll out of the attack's path", func(t *testing.T) {
		t.Parallel()
		a, e := brisaFight(t, true)
		a.h.roller.queue(9, 13)
		bolt, err := a.castGuidingBolt(t, e)
		if err != nil || bolt.GetInspirationOffer().GetHoldId() == "" {
			t.Fatalf("Brisa's Guiding Bolt = %v, %v; want the question about her die", bolt, err)
		}
		hold := bolt.GetInspirationOffer().GetHoldId()
		if _, err := a.answerOutside(t, a.bia, hold, false, false, 0); err != nil {
			t.Fatalf("the player's answer: error = %v", err)
		}
		_, err = a.answerOutside(t, a.ana, hold, false, false, 0)
		wantCode(t, "another player's answer to a roll answered already", err, connect.CodeNotFound)
	})
}

// TestTheDieOfAnAnswerIsTheRollersOwn: the copies of an NPC share a character, and each holds a die of its own: the
// answer to one copy's attack uses that copy's die, not another's.
func TestTheDieOfAnAnswerIsTheRollersOwn(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId(), Count: 2}},
		npcRolls: []int{18, 18},
		players:  map[string]int32{"Toren": 5, "Pensantus": 5, "Brisa": 5},
		reveal:   []string{"Goblin 1", "Goblin 2"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin 1": {4, 3}, "Goblin 2": {3, 4}, "Pensantus": {10, 3}, "Brisa": {8, 8}},
	})
	for label, sides := range map[string]int{"Goblin 1": 6, "Goblin 2": 12} {
		a.giveCombatantDie(t, label, e)
		if _, err := a.h.pool.Exec(t.Context(), `UPDATE combatants SET inspiration_sides = $2 WHERE id = $1`, a.id(t, label), sides); err != nil {
			t.Fatalf("the die of %s: %v", label, err)
		}
	}
	first := a.mustAttack(t, a.master, e, "Goblin 1", sword, "Toren", d20(8))
	second := a.mustAttack(t, a.master, e, "Goblin 2", sword, "Toren", d20(9))
	if first.GetInspirationOffer().GetDie().GetSides() != 6 || second.GetInspirationOffer().GetDie().GetSides() != 12 {
		t.Fatalf("the dice offered = d%d and d%d, want d6 and d12", first.GetInspirationOffer().GetDie().GetSides(), second.GetInspirationOffer().GetDie().GetSides())
	}
	// A 10 is a face of the second copy's d12, not of the first one's d6.
	typed10 := func(r *playv1.AnswerBardicInspirationRequest) {
		r.Use, r.Roll = true, &playv1.AnswerBardicInspirationRequest_TypedFace{TypedFace: 10}
	}
	res, err := a.answer(t, a.master, e, second.GetInspirationOffer().GetHoldId(), typed10)
	if err != nil {
		t.Fatalf("the second copy's answer with a 10: error = %v", err)
	}
	if r := res.GetAttack().GetRoll(); len(r.GetBonusDice()) != 1 || r.GetBonusDice()[0].GetFace() != 10 || r.GetBonusDice()[0].GetSides() != 12 {
		t.Errorf("the second copy's roll = %v, want its d12's 10 added", r)
	}
	var sides *int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT inspiration_sides FROM combatants WHERE id = $1`, a.id(t, "Goblin 1")).Scan(&sides); err != nil || sides == nil || *sides != 6 {
		t.Errorf("the first copy's die = %v (%v), want its d6 still held", sides, err)
	}
	_, err = a.answer(t, a.master, e, first.GetInspirationOffer().GetHoldId(), typed10)
	wantCode(t, "the first copy's answer with a 10", err, connect.CodeInvalidArgument)
}

// TestAHeldAttackKeepsTheDieTypedAsAListOfOne: a single d20 typed as `d20_faces` of one face (the request allows it) is
// kept as such in the request the hold keeps, or the answer would run the attack again with no roll at all.
func TestAHeldAttackKeepsTheDieTypedAsAListOfOne(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		req   *playv1.RollAttackRequest
		faces []int
		want  []int32
	}{
		"a list of one":         {&playv1.RollAttackRequest{D20Faces: []int32{14}}, []int{14}, []int32{14}},
		"a pair":                {&playv1.RollAttackRequest{D20Faces: []int32{14, 3}}, []int{14, 3}, []int32{14, 3}},
		"a pair rolled in app":  {&playv1.RollAttackRequest{Roll: &playv1.RollAttackRequest_RollInApp{RollInApp: true}}, []int{14, 3}, []int32{14, 3}},
		"one die rolled in app": {&playv1.RollAttackRequest{Roll: &playv1.RollAttackRequest_RollInApp{RollInApp: true}}, []int{14}, nil},
		"one die typed":         {&playv1.RollAttackRequest{Roll: &playv1.RollAttackRequest_D20Face{D20Face: 14}}, []int{14}, nil},
	} {
		pinAttackFaces(tc.req, tc.faces)
		if got := tc.req.GetD20Faces(); !slices.Equal(got, tc.want) {
			t.Errorf("%s: kept d20_faces = %v, want %v", name, got, tc.want)
		}
	}
}
