package play

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The branches of the monk's riders that the first tests did not reach (UseHitRider): what
// each choice does after the target's save, and each refusal with its code and message.
// Frozen as they are; a finding is commented where the behaviour looks odd.

// wantRiderRefusal fails unless err is a connect error with this code and message.
func wantRiderRefusal(t *testing.T, call string, err error, code connect.Code, msg string) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("%s error = %v, want a connect error %v %q", call, err, code, msg)
	}
	if ce.Code() != code || ce.Message() != msg {
		t.Errorf("%s error = %v %q, want %v %q", call, ce.Code(), ce.Message(), code, msg)
	}
}

// riderLineOf is the master's log line of the rider answered with this choice.
func (a *armed) riderLineOf(t *testing.T, choice playv1.HitRiderChoice) *playv1.CombatLogHitRider {
	t.Helper()
	for _, r := range a.log(t, a.master, a.get(t, a.master)).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_HIT_RIDER && en.GetHitRider().GetChoice() == choice {
				return en.GetHitRider()
			}
		}
	}
	t.Fatalf("the log has no rider line for %v", choice)
	return nil
}

// riderMonkFight is a combat of a monk (Toren) of level 5 and the Open Hand, whose Attack action
// (two strikes at level 5) and Flurry of Blows strike all hit the Goblin: the offers by kind.
func riderMonkFight(t *testing.T) (*armed, *playv1.Encounter, map[playv1.HitRiderKind]string) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:monk", 5, "subclass:open-hand"), abilities(10, 16, 14, 14, 8), nil, nil, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.threeAndAGoblin(t)
	// The Attack action's hit offers the Stunning Strike; the Flurry of Blows strike after it offers
	// the Open Hand technique (and another Stunning Strike, which the tests leave open).
	wantHit(t, "the Attack action", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	offers := map[playv1.HitRiderKind]string{}
	for _, o := range a.get(t, a.caio).GetHitRiders() {
		offers[o.GetKind()] = o.GetId()
	}
	if len(offers) != 1 {
		t.Fatalf("offers after the first strike = %v, want a Stunning Strike", offers)
	}
	wantHit(t, "the Extra Attack", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	if _, err := a.action(t, a.caio, e, "Toren", flurry); err != nil {
		t.Fatalf("Flurry of Blows error = %v", err)
	}
	wantHit(t, "a flurry strike", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	open := ridersOfKind(a.get(t, a.caio), playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND)
	if len(open) != 1 || offers[playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE] == "" {
		t.Fatalf("offers after the hits = %v, want an Open Hand and a Stunning Strike offer", a.get(t, a.caio).GetHitRiders())
	}
	offers[playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND] = open[0].GetId()
	return a, e, offers
}

// TestTheTargetsSaveDecidesWhatARiderDoes: the Goblin's save is rolled by the app (the
// scripted d20) against the ki save DC (8 + 3 + 2 = 13): a failure applies the choice, a
// success applies nothing; the push is the master's to place, so even a failure changes no
// condition (frozen: the server only says "failed"). No reactions has no save at all.
func TestTheTargetsSaveDecidesWhatARiderDoes(t *testing.T) {
	t.Parallel()
	const (
		failedSave = 1
		savedSave  = 20
	)
	tests := []struct {
		name     string
		kind     playv1.HitRiderKind
		choice   playv1.HitRiderChoice
		d20      int
		want     playv1.HitRiderLogResult
		wantCond string
	}{
		{"prone, failed save", playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND, playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE, failedSave, playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED, "condition:prone"},
		{"prone, saved", playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND, playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE, savedSave, playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_SAVED, ""},
		{"push, failed save", playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND, playv1.HitRiderChoice_HIT_RIDER_CHOICE_PUSH, failedSave, playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED, ""},
		{"push, saved", playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND, playv1.HitRiderChoice_HIT_RIDER_CHOICE_PUSH, savedSave, playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_SAVED, ""},
		{"stun, failed save", playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE, playv1.HitRiderChoice_HIT_RIDER_CHOICE_STUN, failedSave, playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED, "condition:stunned"},
		{"stun, saved", playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE, playv1.HitRiderChoice_HIT_RIDER_CHOICE_STUN, savedSave, playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_SAVED, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e, offers := riderMonkFight(t)
			a.h.roller.queue(tc.d20)
			after, err := a.useRider(t, a.caio, e, offers[tc.kind], tc.choice)
			if err != nil {
				t.Fatalf("UseHitRider() error = %v", err)
			}
			line := a.lastRiderLine(t)
			if line.GetResult() != tc.want || line.GetChoice() != tc.choice {
				t.Errorf("the rider line = %v / %v, want %v / %v", line.GetChoice(), line.GetResult(), tc.choice, tc.want)
			}
			if !line.GetNumbers() || line.GetD20() != int32(tc.d20) || line.GetDc() != 13 {
				t.Errorf("the master's numbers = %v d20 %d dc %d, want the save's d20 %d against DC 13", line.GetNumbers(), line.GetD20(), line.GetDc(), tc.d20)
			}
			for _, cond := range []string{"condition:prone", "condition:stunned"} {
				if got := riderTargetHas(after, "Goblin", cond); got != (cond == tc.wantCond) {
					t.Errorf("the Goblin has %s = %v, want %v", cond, got, cond == tc.wantCond)
				}
			}
		})
	}
}

// TestARiderRefusalsKeepTheirCodesAndMessages: who may answer, a choice of another rider, a
// monk without ki, a rider whose feature the sheet lost.
func TestARiderRefusalsKeepTheirCodesAndMessages(t *testing.T) {
	t.Parallel()
	a, e, offers := riderMonkFight(t)
	open := offers[playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND]
	stun := offers[playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE]
	const fake = "00000000-0000-4000-8000-000000000001"

	// Another player is told the same whether the offer exists or not (RN-10).
	_, errReal := a.useRider(t, a.ana, e, open, playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE)
	_, errFake := a.useRider(t, a.ana, e, fake, playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE)
	// Finding (combat_riders.go:215-220 against :205-207): both are permission_denied, but the words differ,
	// so a player can tell a real offer from an invented one by the message. Frozen.
	wantRiderRefusal(t, "another player answering a real offer", errReal, connect.CodePermissionDenied, "only the combatant's player or the master may do this")
	wantRiderRefusal(t, "another player answering a missing offer", errFake, connect.CodePermissionDenied, "only the monk's player or the master may answer")
	// The master learns that the offer is not there.
	_, err := a.useRider(t, a.master, e, fake, playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE)
	wantRiderRefusal(t, "the master answering a missing offer", err, connect.CodeNotFound, "offer not found")

	// A choice that does not belong to the offer.
	_, err = a.useRider(t, a.caio, e, stun, playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE)
	wantRiderRefusal(t, "prone on a Stunning Strike offer", err, connect.CodeInvalidArgument, "the choice is not one this offer has")
	_, err = a.useRider(t, a.caio, e, open, playv1.HitRiderChoice_HIT_RIDER_CHOICE_UNSPECIFIED)
	wantRiderRefusal(t, "no choice", err, connect.CodeInvalidArgument, "the choice is not one this offer has")

	// No ki left for the stun: the offer stays open.
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE character_vitals SET resources_used = jsonb_set(resources_used, '{ki}', '5') WHERE character_id = $1`, a.toren.GetId()); err != nil {
		t.Fatalf("spend the ki: %v", err)
	}
	_, err = a.useRider(t, a.caio, e, stun, playv1.HitRiderChoice_HIT_RIDER_CHOICE_STUN)
	wantBlockedBy(t, "UseHitRider(stun) without ki", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_UNSPECIFIED)
	wantRiderRefusal(t, "UseHitRider(stun) without ki", err, connect.CodeFailedPrecondition, "no ki point left")

}

// TestARiderWhoseFeatureTheSheetLacksIsRefused: an Open Hand offer on a monk of no Way (a row
// that no hit makes today) is refused when the choice needs the feature.
func TestARiderWhoseFeatureTheSheetLacksIsRefused(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:monk", 5, ""), abilities(10, 16, 14, 14, 8), nil, nil, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.threeAndAGoblin(t)
	wantHit(t, "a melee hit", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	offer := ridersOfKind(a.get(t, a.caio), playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE)[0]
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE hit_riders SET kind = 'open_hand' WHERE id = $1`, offer.GetId()); err != nil {
		t.Fatalf("change the offer: %v", err)
	}
	_, err := a.useRider(t, a.caio, e, offer.GetId(), playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE)
	wantBlockedBy(t, "UseHitRider() without the feature", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_UNSPECIFIED)
	wantRiderRefusal(t, "UseHitRider() without the feature", err, connect.CodeFailedPrecondition, "the combatant does not have this feature")
}

// TestARiderOfAnotherRoundIsOver: an offer is open until the round ends (frozen: the offer's
// round is compared with the combat's).
func TestARiderOfAnotherRoundIsOver(t *testing.T) {
	t.Parallel()
	a, e, offers := riderMonkFight(t)
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE hit_riders SET round = round - 1 WHERE id = $1`, offers[playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND]); err != nil {
		t.Fatalf("age the offer: %v", err)
	}
	_, err := a.useRider(t, a.caio, e, offers[playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND], playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE)
	wantBlockedBy(t, "UseHitRider() on an offer of another round", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_UNSPECIFIED)
	wantRiderRefusal(t, "UseHitRider() on an offer of another round", err, connect.CodeFailedPrecondition, "the offer was answered or its round is over")
}

// TestAStunnedTargetFailsADexterityRiderSaveWithoutARoll: a stunned creature fails its
// Strength and Dexterity saves, so the Open Hand prone needs no d20 (the line has no numbers) and
// the roll the script holds is left for the next roll.
func TestAStunnedTargetFailsADexterityRiderSaveWithoutARoll(t *testing.T) {
	t.Parallel()
	a, e, offers := riderMonkFight(t)
	a.h.roller.queue(1)
	if _, err := a.useRider(t, a.caio, e, offers[playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE], playv1.HitRiderChoice_HIT_RIDER_CHOICE_STUN); err != nil {
		t.Fatalf("UseHitRider(stun) error = %v", err)
	}
	a.h.roller.queue(20) // would save, if it were rolled
	after, err := a.useRider(t, a.caio, e, offers[playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND], playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE)
	if err != nil {
		t.Fatalf("UseHitRider(prone) error = %v", err)
	}
	line := a.riderLineOf(t, playv1.HitRiderChoice_HIT_RIDER_CHOICE_PRONE)
	if line.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED || line.GetNumbers() || line.GetD20() != 0 {
		t.Errorf("the rider line = %v numbers %v d20 %d, want a failed save without a roll", line.GetResult(), line.GetNumbers(), line.GetD20())
	}
	if !riderTargetHas(after, "Goblin", "condition:prone") {
		t.Error("the stunned Goblin is not prone")
	}
}

// TestADeclinedRiderLineSaysSo: the log line of a declined rider has the decline choice and
// result and no numbers.
func TestADeclinedRiderLineSaysSo(t *testing.T) {
	t.Parallel()
	a, e, offers := riderMonkFight(t)
	if _, err := a.useRider(t, a.caio, e, offers[playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND], playv1.HitRiderChoice_HIT_RIDER_CHOICE_DECLINE); err != nil {
		t.Fatalf("UseHitRider(decline) error = %v", err)
	}
	line := a.lastRiderLine(t)
	if line.GetChoice() != playv1.HitRiderChoice_HIT_RIDER_CHOICE_DECLINE || line.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_DECLINED || line.GetNumbers() {
		t.Errorf("the rider line = %v / %v numbers %v, want a plain decline", line.GetChoice(), line.GetResult(), line.GetNumbers())
	}
}
