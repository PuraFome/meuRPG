package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

const wholenessKey = "feature:wholeness-of-body"

// wholenessFight is a combat where Toren (first in the order) has these classes and Brisa is a fighter; Pensantus
// is the other player.
func wholenessFight(t *testing.T, classes ...*charactersv1.ClassLevel) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: abilities(10, 16, 14, 14, 8), RaceKey: "race:human", Classes: classes,
		}}}
		res, err := a.caio.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Toren", Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Toren) error = %v", err)
		}
		a.toren = res.Msg.GetCharacter()
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	return a, a.threeAndAGoblin(t)
}

func openHand(level int32) *charactersv1.ClassLevel {
	return classLevel("class:monk", level, "subclass:open-hand")
}

// TestWholenessOfBodyHealsThreeTimesTheMonkLevel: the action heals 3 x the monk level with no roll, up to the
// maximum, spends the action and the use, and the master and the owner see the numbers.
func TestWholenessOfBodyHealsThreeTimesTheMonkLevel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		level int32
		want  int32
	}{{"level 6", 6, 18}, {"level 11", 11, 33}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e := wholenessFight(t, openHand(tc.level))
			a.correct(t, a.toren, hpIs(1))
			o := a.mustOptions(t, a.caio, e, "Toren")
			if w := featureOption(o, wholenessKey); w == nil || !w.GetEnabled() || w.GetUsesLeft() != 1 || w.GetAction().GetEconomy() != rulesv1.ActionEconomy_ACTION_ECONOMY_ACTION {
				t.Fatalf("Integridade Corporal = %v, want an action with one use", w)
			}
			res, err := a.feature(t, a.caio, e, "Toren", wholenessKey, noTakeRoll) // nothing to roll or type
			if err != nil {
				t.Fatalf("TakeAction(wholeness of body) error = %v", err)
			}
			if res.GetHealed() != tc.want || res.GetRoll() != nil {
				t.Errorf("healed %d with roll %v, want %d and no roll", res.GetHealed(), res.GetRoll(), tc.want)
			}
			v := a.vitals(t, a.toren)
			if got := v.GetHitPointsCurrent(); got != 1+tc.want {
				t.Errorf("Toren = %d PV, want %d", got, 1+tc.want)
			}
			if used, total := resourceUsed(v, "wholeness_of_body"); used != 1 || total != 1 {
				t.Errorf("wholeness_of_body used = %d of %d, want 1 of 1", used, total)
			}
			if c := byLabel(t, res.GetEncounter(), "Toren"); !c.GetActionUsed() || c.GetBonusActionUsed() {
				t.Errorf("economy after it = action %v, bonus %v; want only the action spent", c.GetActionUsed(), c.GetBonusActionUsed())
			}
			_, err = a.feature(t, a.caio, e, "Toren", wholenessKey, noTakeRoll)
			wantBlockedBy(t, "Integridade Corporal with the action used", err, blockedActionUsed)
		})
	}
}

// TestWholenessOfBodyIsCappedAtTheMaximum: only what is missing is healed; at full hit points the use is spent like
// Second Wind's.
func TestWholenessOfBodyIsCappedAtTheMaximum(t *testing.T) {
	t.Parallel()
	a, e := wholenessFight(t, openHand(6))
	max := a.vitals(t, a.toren).GetHitPointsMax()
	a.correct(t, a.toren, hpIs(max-5))
	res, err := a.feature(t, a.caio, e, "Toren", wholenessKey, noTakeRoll)
	if err != nil {
		t.Fatalf("TakeAction(wholeness of body) error = %v", err)
	}
	if res.GetHealed() != 5 || a.vitals(t, a.toren).GetHitPointsCurrent() != max {
		t.Errorf("healed %d, Toren at %d; want 5 healed up to %d", res.GetHealed(), a.vitals(t, a.toren).GetHitPointsCurrent(), max)
	}
}

// TestWholenessOfBodyLiftsAMonkAtZeroHitPoints: a heal from 0 gets the monk up with the death saves reset.
func TestWholenessOfBodyLiftsAMonkAtZeroHitPoints(t *testing.T) {
	t.Parallel()
	a, e := wholenessFight(t, openHand(6))
	a.correct(t, a.toren, hpIs(0))
	if _, err := a.feature(t, a.caio, e, "Toren", wholenessKey, noTakeRoll); err != nil {
		t.Fatalf("TakeAction(wholeness of body) error = %v", err)
	}
	if cur, _, defeated := a.hp(t, "Toren"); cur != 18 || defeated {
		t.Errorf("Toren = %d PV, defeated %v; want 18 and up", cur, defeated)
	}
}

// TestWholenessOfBodyHasOneUseUntilALongRest: a spent use is NO_USES with its recharge, for the master too, and a
// long rest gives it back.
func TestWholenessOfBodyHasOneUseUntilALongRest(t *testing.T) {
	t.Parallel()
	a, e := wholenessFight(t, openHand(6))
	a.correct(t, a.toren, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.HitPointsCurrent = new(int32(1))
		r.ResourcesUsed = []*playv1.ResourceUsed{{Key: "wholeness_of_body", Used: 1}}
	})
	o := a.mustOptions(t, a.caio, e, "Toren")
	if w := featureOption(o, wholenessKey); w == nil || w.GetEnabled() || w.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NO_USES || w.GetReason().GetRecharge() != rulesv1.Recharge_RECHARGE_LONG_REST {
		t.Fatalf("Integridade Corporal with no use = %v, want NO_USES with a long rest", w)
	}
	_, err := a.feature(t, a.caio, e, "Toren", wholenessKey, noTakeRoll)
	if b := wantBlockedBy(t, "Integridade Corporal with no uses", err, blockedNoUses); b.GetRecharge() != rulesv1.Recharge_RECHARGE_LONG_REST {
		t.Errorf("recharge = %v, want a long rest", b.GetRecharge())
	}
	_, err = a.feature(t, a.master, e, "Toren", wholenessKey, noTakeRoll)
	wantBlockedBy(t, "Integridade Corporal with no uses, by the master", err, blockedNoUses)
	if got := a.vitals(t, a.toren).GetHitPointsCurrent(); got != 1 {
		t.Errorf("Toren = %d PV after the refusals, want 1", got)
	}
	a.endEncounter(t, e)
	if err := a.rpcLongRest(t); err != nil {
		t.Fatalf("TakeRest(long) error = %v", err)
	}
	if used, _ := resourceUsed(a.vitals(t, a.toren), "wholeness_of_body"); used != 0 {
		t.Errorf("wholeness_of_body used = %d after a long rest, want 0", used)
	}
}

// TestWholenessOfBodyIsOnlyTheOpenHands: a monk of another tradition, a monk below level 6 and a fighter have no such
// action, and asking for it is refused.
func TestWholenessOfBodyIsOnlyTheOpenHands(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		classes []*charactersv1.ClassLevel
	}{
		{"a monk with no tradition", []*charactersv1.ClassLevel{classLevel("class:monk", 6, "")}},
		{"an Open Hand monk of level 5", []*charactersv1.ClassLevel{openHand(5)}},
		{"a fighter", []*charactersv1.ClassLevel{classLevel("class:fighter", 6, "")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e := wholenessFight(t, tc.classes...)
			if w := featureOption(a.mustOptions(t, a.caio, e, "Toren"), wholenessKey); w != nil {
				t.Errorf("Integridade Corporal = %v, want no such action", w)
			}
			_, err := a.feature(t, a.caio, e, "Toren", wholenessKey, noTakeRoll)
			wantCode(t, "Integridade Corporal without the feature", err, connect.CodeInvalidArgument)
		})
	}
}

// TestWholenessOfBodyCountsTheMonkLevelsOnly: a monk 6 / fighter 5 heals 3 x 6.
func TestWholenessOfBodyCountsTheMonkLevelsOnly(t *testing.T) {
	t.Parallel()
	a, e := wholenessFight(t, openHand(6), classLevel("class:fighter", 5, ""))
	a.correct(t, a.toren, hpIs(1))
	res, err := a.feature(t, a.caio, e, "Toren", wholenessKey, noTakeRoll)
	if err != nil {
		t.Fatalf("TakeAction(wholeness of body) error = %v", err)
	}
	if res.GetHealed() != 18 {
		t.Errorf("healed %d, want 18 (3 x the 6 monk levels)", res.GetHealed())
	}
}

// TestWholenessOfBodyIsTheOwnersOrTheMasters: another player is refused, the master acts for the absent player and
// the log is read with the numbers by the owner and the master only.
func TestWholenessOfBodyIsTheOwnersOrTheMasters(t *testing.T) {
	t.Parallel()
	a, e := wholenessFight(t, openHand(6))
	a.correct(t, a.toren, hpIs(1))
	_, err := a.feature(t, a.ana, e, "Toren", wholenessKey, noTakeRoll)
	wantCode(t, "another player using Toren's feature", err, connect.CodePermissionDenied)
	if _, err := a.feature(t, a.master, e, "Toren", wholenessKey, noTakeRoll); err != nil {
		t.Fatalf("TakeAction(wholeness of body) by the master: %v", err)
	}
	wantActedByMaster(t, a, e, "Toren", a.caio, a.ana, playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION)
	for _, tc := range []struct {
		who  *user
		name string
		sees bool
	}{{a.master, "the master", true}, {a.caio, "the owner", true}, {a.ana, "another player", false}} {
		healed := false
		for _, round := range a.log(t, tc.who, e).GetRounds() {
			for _, entry := range round.GetEntries() {
				if d := entry.GetDamage(); entry.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION && d.GetHealing() {
					healed = d.GetAmount() == 18 && d.GetRoll() == nil
				}
			}
		}
		if healed != tc.sees {
			t.Errorf("%s reads the heal of 18 = %v, want %v", tc.name, healed, tc.sees)
		}
	}
}

// TestWholenessOfBodyUndo: the undo gives back the hit points, the use and the action.
func TestWholenessOfBodyUndo(t *testing.T) {
	t.Parallel()
	a, e := wholenessFight(t, openHand(6))
	a.correct(t, a.toren, hpIs(1))
	if _, err := a.feature(t, a.caio, e, "Toren", wholenessKey, noTakeRoll); err != nil {
		t.Fatalf("TakeAction(wholeness of body) error = %v", err)
	}
	a.undoLast(t, e)
	v := a.vitals(t, a.toren)
	if used, _ := resourceUsed(v, "wholeness_of_body"); used != 0 || v.GetHitPointsCurrent() != 1 {
		t.Errorf("after the undo: used %d, %d PV; want 0 and 1", used, v.GetHitPointsCurrent())
	}
	if c := byLabel(t, a.get(t, a.caio), "Toren"); c.GetActionUsed() {
		t.Error("the undo did not give the action back")
	}
}
