package play

import (
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The refusals the transaction of each core handler reads from the combat, with their exact
// code, reason and message, and what a refused call leaves behind (nothing). The requests
// that fail before the transaction are in combat_core_request_test.go.

// coreFixture is an armed table with a combat in SETUP: Toren, Pensantus and the two copies of
// the Goblin have an initiative, Brisa has none. The Goblins tie.
type coreFixture struct {
	*armed
	e *playv1.Encounter
}

func newCoreFixture(t *testing.T) *coreFixture {
	t.Helper()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId(), Count: 2}},
		npcRolls: []int{14, 14},
		players:  map[string]int32{"Toren": 10, "Pensantus": 8},
		setup:    true,
	})
	return &coreFixture{armed: a, e: e}
}

func (x *coreFixture) label(t *testing.T, label string) string {
	t.Helper()
	return byLabel(t, x.get(t, x.master), label).GetId()
}

// events counts the session's events: a refused call adds none.
func (x *coreFixture) eventCount(t *testing.T) int {
	t.Helper()
	return x.events(t)
}

func (x *coreFixture) submit(t *testing.T, u *user, label string, edit func(*playv1.SubmitInitiativeRequest)) error {
	t.Helper()
	r := &playv1.SubmitInitiativeRequest{CampaignId: x.campaignID, EncounterId: x.e.GetId(), CombatantId: x.label(t, label), IdempotencyKey: newKey()}
	edit(r)
	_, err := u.combat.SubmitInitiative(t.Context(), connect.NewRequest(r))
	return err
}

func (x *coreFixture) begin(t *testing.T) error {
	t.Helper()
	_, err := x.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{
		CampaignId: x.campaignID, EncounterId: x.e.GetId(), IdempotencyKey: newKey(),
	}))
	return err
}

func (x *coreFixture) addCombatants(t *testing.T, parts ...*playv1.Participant) error {
	t.Helper()
	_, err := x.master.combat.AddCombatants(t.Context(), connect.NewRequest(&playv1.AddCombatantsRequest{
		CampaignId: x.campaignID, EncounterId: x.e.GetId(), IdempotencyKey: newKey(), Participants: parts,
	}))
	return err
}

func (x *coreFixture) hide(t *testing.T, label string, hidden bool) error {
	t.Helper()
	_, err := x.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: x.campaignID, EncounterId: x.e.GetId(), CombatantId: x.label(t, label), IdempotencyKey: newKey(), Hidden: hidden,
	}))
	return err
}

func (x *coreFixture) remove(t *testing.T, u *user, label string) error {
	t.Helper()
	_, err := u.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
		CampaignId: x.campaignID, EncounterId: x.e.GetId(), CombatantId: x.label(t, label), IdempotencyKey: newKey(),
	}))
	return err
}

func (x *coreFixture) order(t *testing.T, labels ...string) error {
	t.Helper()
	ids := make([]string, len(labels))
	for i, l := range labels {
		ids[i] = x.label(t, l)
	}
	_, err := x.master.combat.SetInitiativeOrder(t.Context(), connect.NewRequest(&playv1.SetInitiativeOrderRequest{
		CampaignId: x.campaignID, EncounterId: x.e.GetId(), IdempotencyKey: newKey(), CombatantIds: ids,
	}))
	return err
}

func (x *coreFixture) conditions(t *testing.T, u *user, label string, edit func(*playv1.SetCombatantConditionsRequest)) error {
	t.Helper()
	r := &playv1.SetCombatantConditionsRequest{CampaignId: x.campaignID, EncounterId: x.e.GetId(), CombatantId: x.label(t, label), IdempotencyKey: newKey()}
	edit(r)
	_, err := u.combat.SetCombatantConditions(t.Context(), connect.NewRequest(r))
	return err
}

// TestCoreSetupRefusals freezes what the combat core refuses while the combat is in SETUP.
func TestCoreSetupRefusals(t *testing.T) {
	t.Parallel()
	x := newCoreFixture(t)
	const (
		invalid   = connect.CodeInvalidArgument
		notFound  = connect.CodeNotFound
		forbidden = connect.CodePermissionDenied
	)
	inApp := func(r *playv1.SubmitInitiativeRequest) {
		r.Roll = &playv1.SubmitInitiativeRequest_RollInApp{RollInApp: true}
	}

	// SubmitInitiative.
	wantCoreRefusal(t, "another player's combatant", x.submit(t, x.ana, "Toren", inApp), forbidden, "only the combatant's player or the master may do this")
	err := x.submit(t, x.caio, "Toren", inApp)
	wantBlockedBy(t, "an initiative set already", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_INITIATIVE_ALREADY_SET)
	wantCoreRefusal(t, "an initiative set already", err, connect.CodeFailedPrecondition, "your initiative is already set")
	wantCoreRefusal(t, "a face of 0", x.submit(t, x.bia, "Brisa", func(r *playv1.SubmitInitiativeRequest) {
		r.Roll = &playv1.SubmitInitiativeRequest_D20Face{D20Face: 0}
	}), invalid, "d20_face must be 1 to 20")
	wantCoreRefusal(t, "a face of 21", x.submit(t, x.bia, "Brisa", func(r *playv1.SubmitInitiativeRequest) {
		r.Roll = &playv1.SubmitInitiativeRequest_D20Face{D20Face: 21}
	}), invalid, "d20_face must be 1 to 20")
	// The master may roll again for a combatant that has an initiative.
	if err := x.submit(t, x.master, "Toren", func(r *playv1.SubmitInitiativeRequest) {
		r.Roll = &playv1.SubmitInitiativeRequest_D20Face{D20Face: 10}
	}); err != nil {
		t.Fatalf("the master's second initiative for Toren: %v", err)
	}
	before := x.eventCount(t)

	// BeginCombat: Brisa has no initiative.
	err = x.begin(t)
	b := wantBlockedBy(t, "a combat with someone's initiative missing", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_INITIATIVE_MISSING)
	wantCoreRefusal(t, "a combat with someone's initiative missing", err, connect.CodeFailedPrecondition, "missing the initiative of: [Brisa]")
	if ids := b.GetCombatantIds(); len(ids) != 1 || ids[0] != x.label(t, "Brisa") {
		t.Errorf("the missing combatants = %v, want Brisa's", ids)
	}

	// SetInitiativeOrder.
	wantCoreRefusal(t, "combatants that are not tied", x.order(t, "Goblin 1", "Toren"), invalid, "combatant_ids must be tied: the same initiative total and bonus")
	_, err = x.master.combat.SetInitiativeOrder(t.Context(), connect.NewRequest(&playv1.SetInitiativeOrderRequest{
		CampaignId: x.campaignID, EncounterId: x.e.GetId(), IdempotencyKey: newKey(), CombatantIds: []string{x.label(t, "Goblin 1"), newKey()},
	}))
	wantCoreRefusal(t, "a combatant that is not in the combat", err, notFound, "combatant not found")

	// SetCombatantHidden.
	wantCoreRefusal(t, "a player's combatant", x.hide(t, "Toren", true), invalid, "a player's combatant is never hidden")

	// AddCombatants.
	wantCoreRefusal(t, "a player's character that is in already", x.addCombatants(t, &playv1.Participant{CharacterId: x.toren.GetId()}),
		invalid, "a player's character joins only before the combat begins, and once")
	var many []*playv1.Participant
	for _, name := range []string{"Arqueiro", "Lanceiro", "Mago", "Chefe"} {
		many = append(many, &playv1.Participant{CharacterId: x.master.npc(t, x.campaignID, name, 5, 10).GetId(), Count: maxNPCCopies})
	}
	wantCoreRefusal(t, "more combatants than a combat holds", x.addCombatants(t, many...), invalid, "a combat has at most 40 combatants")

	// StandUp and RemoveCombatant in SETUP: a player may leave a combat that has not begun.
	_, err = x.caio.combat.StandUp(t.Context(), connect.NewRequest(&playv1.StandUpRequest{
		CampaignId: x.campaignID, EncounterId: x.e.GetId(), CombatantId: x.label(t, "Toren"), IdempotencyKey: newKey(),
	}))
	wantBlockedBy(t, "StandUp in SETUP", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ACTIVE)
	wantCoreRefusal(t, "StandUp in SETUP", err, connect.CodeFailedPrecondition, "the combat is not running")

	// SetCombatantConditions.
	wantCoreRefusal(t, "a player marks conditions", x.conditions(t, x.caio, "Toren", func(r *playv1.SetCombatantConditionsRequest) {
		r.Conditions = &playv1.ConditionList{Keys: []string{condProne}}
	}), forbidden, "only the master marks conditions")
	wantCoreRefusal(t, "another player's combatant", x.conditions(t, x.ana, "Toren", func(r *playv1.SetCombatantConditionsRequest) {
		r.EndConcentration = true
	}), forbidden, "only the combatant's player or the master may do this")
	if err := x.conditions(t, x.master, "Toren", func(r *playv1.SetCombatantConditionsRequest) { r.Conditions = &playv1.ConditionList{} }); err != nil {
		t.Fatalf("an empty list of conditions on a combatant without any: %v", err)
	}
	if err := x.conditions(t, x.caio, "Toren", func(r *playv1.SetCombatantConditionsRequest) { r.EndConcentration = true }); err != nil {
		t.Fatalf("ending a concentration that is not there: %v", err)
	}

	if got := x.eventCount(t); got != before {
		t.Errorf("the refused calls and the ones that changed nothing left %d events, want %d", got-before, 0)
	}
}

// TestCoreSubmitInitiativeFollowsTheTableDice freezes the forced dice mode on a player's
// initiative: the master is never held to it.
func TestCoreSubmitInitiativeFollowsTheTableDice(t *testing.T) {
	t.Parallel()
	x := newCoreFixture(t)
	x.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	err := x.submit(t, x.bia, "Brisa", func(r *playv1.SubmitInitiativeRequest) {
		r.Roll = &playv1.SubmitInitiativeRequest_RollInApp{RollInApp: true}
	})
	wantBlockedBy(t, "an app roll when the table rolls real dice", err, blockedWrongDice)
	wantCoreRefusal(t, "an app roll when the table rolls real dice", err, connect.CodeFailedPrecondition, "this is not how the campaign has you roll your dice")
	x.forceDice(t, campaignsv1.DiceMode_DICE_MODE_APP)
	err = x.submit(t, x.bia, "Brisa", func(r *playv1.SubmitInitiativeRequest) {
		r.Roll = &playv1.SubmitInitiativeRequest_D20Face{D20Face: 7}
	})
	wantBlockedBy(t, "real dice when the table rolls in the app", err, blockedWrongDice)
	x.h.roller.queue(9)
	if err := x.submit(t, x.master, "Brisa", func(r *playv1.SubmitInitiativeRequest) {
		r.Roll = &playv1.SubmitInitiativeRequest_RollInApp{RollInApp: true}
	}); err != nil {
		t.Fatalf("the master's app roll while the table rolls in the app: %v", err)
	}
}

// TestCoreRunningCombatRefusals freezes what the core refuses once the combat has begun.
func TestCoreRunningCombatRefusals(t *testing.T) {
	t.Parallel()
	x := newCoreFixture(t)
	x.h.roller.queue(5)
	if err := x.submit(t, x.master, "Brisa", func(r *playv1.SubmitInitiativeRequest) {
		r.Roll = &playv1.SubmitInitiativeRequest_RollInApp{RollInApp: true}
	}); err != nil {
		t.Fatalf("Brisa's initiative: %v", err)
	}
	if err := x.begin(t); err != nil {
		t.Fatalf("BeginCombat() error = %v", err)
	}
	before := x.eventCount(t)

	err := x.begin(t)
	wantBlockedBy(t, "a second BeginCombat", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_IN_SETUP)
	wantCoreRefusal(t, "a second BeginCombat", err, connect.CodeFailedPrecondition, "the combat already began or ended")
	err = x.submit(t, x.master, "Toren", func(r *playv1.SubmitInitiativeRequest) {
		r.Roll = &playv1.SubmitInitiativeRequest_D20Face{D20Face: 4}
	})
	wantBlockedBy(t, "an initiative after the begin", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_IN_SETUP)
	wantCoreRefusal(t, "an initiative after the begin", err, connect.CodeFailedPrecondition, "initiative is rolled before the combat begins")
	wantCoreRefusal(t, "a player's character that joins late", x.addCombatants(t, &playv1.Participant{CharacterId: x.toren.GetId()}),
		connect.CodeInvalidArgument, "a player's character joins only before the combat begins, and once")
	err = x.remove(t, x.master, "Toren")
	wantBlockedBy(t, "a player removed from a running combat", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_PLAYER_IN_COMBAT)
	wantCoreRefusal(t, "a player removed from a running combat", err, connect.CodeFailedPrecondition, "a player's combatant cannot leave a combat that is running")
	if got := x.eventCount(t); got != before {
		t.Errorf("the refused calls left %d events, want none", got-before)
	}
}

// TestCoreEndedCombatRefusals freezes that a combat that ended answers every change with
// ENCOUNTER_ENDED, and that ending it twice changes nothing and writes no event.
func TestCoreEndedCombatRefusals(t *testing.T) {
	t.Parallel()
	x := newCoreFixture(t)
	end := func() error {
		_, err := x.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{
			CampaignId: x.campaignID, EncounterId: x.e.GetId(), IdempotencyKey: newKey(),
		}))
		return err
	}
	if err := end(); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	ended := x.eventCount(t)
	if err := end(); err != nil {
		t.Fatalf("a second EndEncounter() error = %v", err)
	}
	if got := x.eventCount(t); got != ended {
		t.Errorf("a second EndEncounter wrote %d events, want none", got-ended)
	}
	standUp := func() error {
		_, err := x.caio.combat.StandUp(t.Context(), connect.NewRequest(&playv1.StandUpRequest{
			CampaignId: x.campaignID, EncounterId: x.e.GetId(), CombatantId: x.label(t, "Toren"), IdempotencyKey: newKey(),
		}))
		return err
	}
	deathSave := func() error {
		_, err := x.caio.combat.RollDeathSave(t.Context(), connect.NewRequest(&playv1.RollDeathSaveRequest{
			CampaignId: x.campaignID, EncounterId: x.e.GetId(), CombatantId: x.label(t, "Toren"), IdempotencyKey: newKey(),
			Roll: &playv1.RollDeathSaveRequest_RollInApp{RollInApp: true},
		}))
		return err
	}
	confirm := func() error {
		_, err := x.master.combat.ConfirmDeath(t.Context(), connect.NewRequest(&playv1.ConfirmDeathRequest{
			CampaignId: x.campaignID, EncounterId: x.e.GetId(), CombatantId: x.label(t, "Toren"), IdempotencyKey: newKey(),
		}))
		return err
	}
	surprise := func() error {
		_, err := x.master.contests.SetSurprised(t.Context(), connect.NewRequest(&playv1.SetSurprisedRequest{
			CampaignId: x.campaignID, EncounterId: x.e.GetId(), CombatantId: x.label(t, "Toren"), IdempotencyKey: newKey(), Surprised: true,
		}))
		return err
	}
	monsters := func() error {
		_, err := x.addMonsters(t, x.e, newKey(), nil)
		return err
	}
	for name, call := range map[string]func() error{
		"SubmitInitiative": func() error {
			return x.submit(t, x.caio, "Toren", func(r *playv1.SubmitInitiativeRequest) {
				r.Roll = &playv1.SubmitInitiativeRequest_RollInApp{RollInApp: true}
			})
		},
		"SetInitiativeOrder": func() error { return x.order(t, "Goblin 1", "Goblin 2") },
		"SetCombatantHidden": func() error { return x.hide(t, "Goblin 1", false) },
		"AddCombatants":      func() error { return x.addCombatants(t, &playv1.Participant{CharacterId: x.goblin.GetId()}) },
		"RemoveCombatant":    func() error { return x.remove(t, x.master, "Goblin 1") },
		"SetCombatantConditions": func() error {
			return x.conditions(t, x.master, "Toren", func(r *playv1.SetCombatantConditionsRequest) { r.EndConcentration = true })
		},
		"StandUp":       standUp,
		"RollDeathSave": deathSave,
		"ConfirmDeath":  confirm,
		"SetSurprised":  surprise,
		"AddMonsters":   monsters,
	} {
		err := call()
		wantBlockedBy(t, name+" on an ended combat", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ENCOUNTER_ENDED)
		wantCoreRefusal(t, name+" on an ended combat", err, connect.CodeFailedPrecondition, "the combat has ended")
	}
	err := x.begin(t)
	wantBlockedBy(t, "BeginCombat on an ended combat", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_IN_SETUP)
	if got := x.eventCount(t); got != ended {
		t.Errorf("the refused calls wrote %d events, want none", got-ended)
	}
}

// TestCoreBeginCombatNeedsSomeone freezes that a combat emptied in SETUP cannot begin.
func TestCoreBeginCombatNeedsSomeone(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Toren": 10, "Pensantus": 9, "Brisa": 8}, setup: true,
	})
	for _, label := range []string{"Goblin", "Toren", "Pensantus", "Brisa"} {
		_, err := a.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(),
		}))
		if err != nil {
			t.Fatalf("RemoveCombatant(%s) in SETUP: %v", label, err)
		}
	}
	_, err := a.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(),
	}))
	wantCoreRefusal(t, "a combat with nobody", err, connect.CodeInvalidArgument, "the combat has no combatants")
}
