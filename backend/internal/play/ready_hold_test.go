package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A readied action against the turn in progress: a readied move's opportunity offers, and a readied attack whose roll is
// held for the windows any attack roll opens.

// rogueReadier is a fight with Reator, Ana's rogue 5, beside the goblin at (4,4), on Reator's turn.
func rogueReadier(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil)
		a.pens = rogueFive(t, a)
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Reator": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Reator": {4, 4}, "Brisa": {8, 8}},
	})
	e = a.onTurnOf(t, e, "Reator")
	return a, e
}

// The rogue readies a move and then Disengages with Cunning Action: Disengage covers only the turn it is taken on, so the
// readied move on the goblin's turn still offers the goblin its attack. The offer holds the end of the goblin's turn, the
// master's too, until it is answered; then the turn ends and no offer is left.
func TestAReadiedMoveProvokesAfterADisengageAndHoldsTheTurn(t *testing.T) {
	t.Parallel()
	a, e := rogueReadier(t)
	a.mustReady(t, a.ana, &playv1.ReadyActionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Reator"), IdempotencyKey: newKey(),
		TriggerText: readyTrigger, Kind: playv1.ReadiedKind_READIED_KIND_MOVE,
	})
	if _, err := a.action(t, a.ana, e, "Reator", "feature:cunning-action:disengage"); err != nil {
		t.Fatalf("Cunning Action (Disengage): %v", err)
	}
	e = a.mustEndTurn(t, a.master, e) // the goblin's turn
	a.mustFire(t, e, "Reator")
	res, err := a.frozenMove(t, a.ana, "Reator", 4, 6)
	if err != nil {
		t.Fatalf("the readied move: %v", err)
	}
	offers := a.get(t, a.master).GetOpportunityOffers()
	if !res.GetProvoked() || len(offers) != 1 || offers[0].GetReactorId() != a.id(t, "Goblin") {
		t.Fatalf("the readied move after a Disengage: provoked %v, offers %v; want the goblin's", res.GetProvoked(), offers)
	}
	_, err = a.endTurn(t, a.master, e, false)
	wantBlockedBy(t, "the master's EndTurn while the readied move's offer waits", err, reasonOpportunityPending)
	if _, err := a.master.combat.DeclineOpportunity(t.Context(), connect.NewRequest(&playv1.DeclineOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: offers[0].GetId(), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("the master's decline: %v", err)
	}
	a.mustEndTurn(t, a.master, e)
	if got := a.get(t, a.master).GetOpportunityOffers(); len(got) != 0 {
		t.Errorf("offers after the turn = %v", got)
	}
}

// heldReadiedAttack is the goblin's readied attack on Toren, fired on the next turn and used by the master: its roll waits
// for Reator's window of the kind. It returns the READIED window.
func heldReadiedAttack(t *testing.T, a *armed, e *playv1.Encounter, kind playv1.ReactionKind) (*playv1.Encounter, *playv1.ReactionWindow, *playv1.ReactionWindow) {
	t.Helper()
	a.readyGoblinAttack(t, e)
	e = a.mustEndTurn(t, a.master, e)
	w := a.mustFire(t, e, "Goblin")
	a.mustAnswer(t, a.master, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		useTyped(15)(r)
		r.CreatureIds = []string{a.id(t, "Toren")}
	})
	pre := a.windowOf(t, a.ana, kind)
	if pre == nil {
		t.Fatalf("Reator has no %v window", kind)
	}
	return e, w, pre
}

// While the roll of a readied attack is held, its READIED window takes no answer (a second "Usar", or "Deixar passar"):
// both are REACTION_PENDING, and the master's screen reads it as not to be answered now, so it never passes by itself.
func TestAHeldReadiedAttacksWindowTakesNoAnswer(t *testing.T) {
	t.Parallel()
	a, e := reactorFight(t, loreBard)
	e, w, pre := heldReadiedAttack(t, a, e, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	_, err := a.answerReaction(t, a.master, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		useTyped(15)(r)
		r.CreatureIds = []string{a.id(t, "Toren")}
	})
	wantBlockedBy(t, "a second Usar while the roll is held", err, blockedReactionPending)
	_, err = a.answerReaction(t, a.master, e, w.GetId(), passAnswer)
	wantBlockedBy(t, "Deixar passar while the roll is held", err, blockedReactionPending)
	if mine := a.windowOf(t, a.master, kindReadied); mine == nil || mine.GetAnswerNow() {
		t.Errorf("the master's READIED window while the roll is held = %v, want answer_now false", mine)
	}
	a.mustAnswer(t, a.ana, e, pre.GetId(), passAnswer)
	if got := a.readiedStatus(t, "Goblin"); got != readiedFired {
		t.Errorf("rows after the roll = %q", got)
	}
}

// With "Sempre" a player's readied attack on an enemy waits for the master's check as any attack does.
func TestAReadiedAttackWaitsForTheMastersCheck(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	a.setRules(t, enemyReactionsAlways)
	e := a.readyThenPass(t, nil)
	w := a.mustFire(t, e, "Toren")
	if res := a.mustAnswer(t, a.caio, e, w.GetId(), useTyped(15)); res.GetResult().GetUsed() {
		t.Errorf("the result says used while the roll waits: %v", res.GetResult())
	}
	check := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
	if check == nil {
		t.Fatal("no master's check before the readied attack's roll")
	}
	a.mustAnswer(t, a.master, e, check.GetId(), passAnswer)
	if got := a.readiedStatus(t, "Toren"); got != readiedFired || !byLabel(t, a.get(t, a.master), "Toren").GetReactionUsed() {
		t.Errorf("after the check: rows %q", got)
	}
}

// A held readied attack that can no longer happen is dropped: a target that fell meanwhile leaves the reaction free, the
// row waiting and the READIED window open, to be used again or passed; a readier that fell loses all three.
func TestADroppedReadiedAttack(t *testing.T) {
	t.Parallel()
	t.Run("the target fell", func(t *testing.T) {
		t.Parallel()
		a, e := protectionFight(t, protector())
		e, w, pre := heldReadiedAttack(t, a, e, playv1.ReactionKind_REACTION_KIND_PROTECTION)
		a.execSQL(t, `UPDATE combatants SET defeated = true WHERE label = 'Toren'`)
		a.mustAnswer(t, a.ana, e, pre.GetId(), passAnswer)
		goblin := byLabel(t, a.get(t, a.master), "Goblin")
		open := a.windowOf(t, a.master, kindReadied)
		if goblin.GetReactionUsed() || a.readiedStatus(t, "Goblin") != readiedWaiting || open == nil || open.GetId() != w.GetId() || !open.GetAnswerNow() {
			t.Fatalf("after the drop: reaction %v, rows %q, window %v", goblin.GetReactionUsed(), a.readiedStatus(t, "Goblin"), open)
		}
		a.mustAnswer(t, a.master, e, w.GetId(), passAnswer)
		if got := a.readiedStatus(t, "Goblin"); got != readiedIgnored {
			t.Errorf("rows after the pass = %q", got)
		}
	})
	t.Run("the readier fell", func(t *testing.T) {
		t.Parallel()
		a, e := protectionFight(t, protector())
		e, _, _ = heldReadiedAttack(t, a, e, playv1.ReactionKind_REACTION_KIND_PROTECTION)
		if _, err := a.adjustHP(t, e, "Goblin", damageHP(999)); err != nil {
			t.Fatalf("damage the goblin: %v", err)
		}
		if a.windowOf(t, a.master, kindReadied) != nil || a.readiedStatus(t, "Goblin") != readiedLost {
			t.Errorf("after the readier fell: window %v, rows %q", a.windowOf(t, a.master, kindReadied), a.readiedStatus(t, "Goblin"))
		}
	})
}

// catch_window_id is a UUID: anything else is invalid_argument before anything is read.
func TestACatchWindowIDIsAUUID(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyThenPass(t, nil)
	_, err := a.attack(t, a.ana, e, "Pensantus", fireBolt, "Goblin", func(r *playv1.RollAttackRequest) {
		d20(10)(r)
		r.CatchWindowId = "not-a-window"
	})
	wantCode(t, "RollAttack with a malformed catch_window_id", err, connect.CodeInvalidArgument)
}
