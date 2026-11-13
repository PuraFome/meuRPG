package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The answers to a READIED window beyond the attack (ready_answer.go): a move, another action, a pass, their undo and
// replay, the fire's refusals, the window closing by itself, an NPC readier and a fire inside an opportunity offer.

// readyMove has Toren ready a move and ends his turn: Pensantus acts.
func (a *armed) readyMove(t *testing.T) *playv1.Encounter {
	t.Helper()
	return a.readyThenPass(t, func(r *playv1.ReadyActionRequest) {
		r.Kind, r.ActionKey, r.TargetCombatantId = playv1.ReadiedKind_READIED_KIND_MOVE, "", ""
	})
}

// readiedOpen says the master reads a READIED window open for the combatant.
func (a *armed) readiedOpen(t *testing.T, label string) bool {
	t.Helper()
	w := a.windowOf(t, a.master, kindReadied)
	return w != nil && w.GetReactorId() == a.id(t, label)
}

// A readied move is walked with MoveCombatant by the readier off its turn while its window is open: no farther than the
// move it readied and its speed now, through Move's own refusals (a grappled readier has no speed), with no opportunity
// offer and no action. The use spends the reaction and fires the row; its undo puts it all back and the window opens again.
// "Usar" on AnswerReaction is refused: it has no square.
func TestFireReadiedMoveIsWalkedWithMoveCombatant(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMove(t)
	a.execSQL(t, `UPDATE readied_actions SET move_ft = 10`)
	_, err := a.frozenMove(t, a.caio, "Toren", 3, 5)
	wantBlockedBy(t, "Toren's move off his turn before the fire", err, blockedNotYourTurn)
	w := a.mustFire(t, e, "Toren")

	_, err = a.answerReaction(t, a.caio, e, w.GetId(), useTyped(10))
	wantCode(t, "Usar on a readied move", err, connect.CodeInvalidArgument)
	before := a.events(t)
	_, err = a.frozenMove(t, a.caio, "Toren", 3, 6)
	wantBlockedBy(t, "a readied move of 15 ft with 10 readied", err, reasonTooFar)
	a.execSQL(t, `UPDATE combatants SET conditions = ARRAY['condition:grappled'] WHERE label = 'Toren'`)
	_, err = a.frozenMove(t, a.caio, "Toren", 3, 4)
	wantBlockedBy(t, "a grappled readier's move", err, reasonTooFar)
	a.execSQL(t, `UPDATE combatants SET conditions = '{}' WHERE label = 'Toren'`)
	if n := a.events(t); n != before || a.readiedStatus(t, "Toren") != readiedWaiting || !a.readiedOpen(t, "Toren") {
		t.Fatalf("after the refusals: events %d (want %d), rows %q, window open %v", n, before, a.readiedStatus(t, "Toren"), a.readiedOpen(t, "Toren"))
	}
	if byLabel(t, a.get(t, a.master), "Toren").GetReactionUsed() {
		t.Fatal("a refused move spent the reaction")
	}

	res, err := a.frozenMove(t, a.caio, "Toren", 3, 5) // out of the goblin's reach
	if err != nil {
		t.Fatalf("the readied move: %v", err)
	}
	if res.GetProvoked() || len(a.get(t, a.master).GetOpportunityOffers()) != 0 {
		t.Error("the readied move offered an opportunity attack")
	}
	toren := byLabel(t, a.get(t, a.caio), "Toren")
	if !toren.GetReactionUsed() || toren.GetReactionKind() != kindReadied || a.squareOf(t, "Toren") != [2]int32{3, 5} {
		t.Errorf("Toren after the move: reaction %v (%v) at %v", toren.GetReactionUsed(), toren.GetReactionKind(), a.squareOf(t, "Toren"))
	}
	if got := a.readiedStatus(t, "Toren"); got != readiedFired || a.readiedOpen(t, "Toren") {
		t.Errorf("rows = %q, window open %v; want one fired, closed", got, a.readiedOpen(t, "Toren"))
	}
	if lines := a.firedLines(t, a.master, e); len(lines) != 1 || lines[0].GetReadiedFired().GetKind() != playv1.ReadiedKind_READIED_KIND_MOVE {
		t.Errorf("the fired lines = %v", lines)
	}
	if _, err := a.frozenMove(t, a.caio, "Toren", 3, 6); err == nil {
		t.Error("Toren moved again off his turn after the use")
	}

	a.mustUndo(t, e)
	toren = byLabel(t, a.get(t, a.caio), "Toren")
	if toren.GetReactionUsed() || a.squareOf(t, "Toren") != [2]int32{3, 3} || a.readiedStatus(t, "Toren") != readiedWaiting || !a.readiedOpen(t, "Toren") {
		t.Errorf("after the undo: reaction %v at %v, rows %q, window open %v", toren.GetReactionUsed(), a.squareOf(t, "Toren"), a.readiedStatus(t, "Toren"), a.readiedOpen(t, "Toren"))
	}
	if lines := a.firedLines(t, a.master, e); len(lines) != 0 {
		t.Errorf("the fired lines after the undo = %v", lines)
	}
}

// "Deixar passar": the row is ignored, the reaction stays, the line says so; the undo makes it wait with its window open
// again, and "Usar" then runs the attack. The undo of that attack makes it wait again.
func TestFireReadiedPassAndTheUndoOfEachAnswer(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyThenPass(t, nil)
	w := a.mustFire(t, e, "Toren")
	res := a.mustAnswer(t, a.caio, e, w.GetId(), passAnswer)
	if r := res.GetResult(); r.GetKind() != kindReadied || r.GetUsed() {
		t.Errorf("the pass's result = %v", r)
	}
	if toren := byLabel(t, a.get(t, a.caio), "Toren"); toren.GetReactionUsed() || toren.GetReadied() != nil {
		t.Errorf("Toren after the pass: reaction used %v, readied %v", toren.GetReactionUsed(), toren.GetReadied())
	}
	if got := a.readiedStatus(t, "Toren"); got != readiedIgnored || a.readiedOpen(t, "Toren") {
		t.Errorf("rows = %q, window open %v; want one ignored, closed", got, a.readiedOpen(t, "Toren"))
	}
	lines := a.firedLines(t, a.caio, e)
	if len(lines) != 1 || lines[0].GetReadiedFired().GetStatus() != playv1.ReadiedStatus_READIED_STATUS_IGNORED {
		t.Errorf("the pass's lines = %v", lines)
	}

	a.mustUndo(t, e)
	if got := a.readiedStatus(t, "Toren"); got != readiedWaiting || !a.readiedOpen(t, "Toren") || len(a.firedLines(t, a.master, e)) != 0 {
		t.Fatalf("after the undo of the pass: rows %q, window open %v, lines %v", got, a.readiedOpen(t, "Toren"), a.firedLines(t, a.master, e))
	}
	w = a.windowOf(t, a.master, kindReadied)
	a.mustAnswer(t, a.caio, e, w.GetId(), useTyped(2))
	if got := a.readiedStatus(t, "Toren"); got != readiedFired {
		t.Fatalf("rows after the use = %q", got)
	}
	a.mustUndo(t, e)
	toren := byLabel(t, a.get(t, a.caio), "Toren")
	if toren.GetReactionUsed() || a.readiedStatus(t, "Toren") != readiedWaiting || !a.readiedOpen(t, "Toren") || len(a.firedLines(t, a.master, e)) != 0 {
		t.Errorf("after the undo of the attack: reaction %v, rows %q, window open %v", toren.GetReactionUsed(), a.readiedStatus(t, "Toren"), a.readiedOpen(t, "Toren"))
	}
	for _, r := range a.log(t, a.master, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
				t.Errorf("an attack line after the undo: %v", en)
			}
		}
	}
}

// FireReadied is the master's: a player is denied. It refuses a readier whose reaction is spent, unconscious, defeated or
// out of the order, a row that does not wait, and a second fire of the same row; none writes anything and the row waits.
func TestFireReadiedRefusals(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyThenPass(t, nil)
	id := byLabel(t, a.get(t, a.master), "Toren").GetReadied().GetId()
	before := a.events(t)
	for _, u := range []*user{a.caio, a.ana} {
		if _, err := a.fire(t, u, e, id); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s's fire: %v, want permission_denied", u.id, err)
		}
	}
	_, err := a.fire(t, a.master, e, newKey())
	wantRefusal(t, "a fire of nothing", err, connect.CodeNotFound, "readied action not found")
	for _, c := range []struct {
		name, set, reset string
		reason           playv1.EncounterBlockedReason
	}{
		{"the reaction spent", `reaction_used = true`, `reaction_used = false`, blockedReactionUsed},
		{"unconscious", `conditions = ARRAY['condition:unconscious']`, `conditions = '{}'`, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CANNOT_ACT},
		{"defeated", `defeated = true`, `defeated = false`, blockedDown},
	} {
		a.execSQL(t, `UPDATE combatants SET `+c.set+` WHERE label = 'Toren'`)
		_, err := a.fire(t, a.master, e, id)
		wantBlockedBy(t, "a fire with "+c.name, err, c.reason)
		a.execSQL(t, `UPDATE combatants SET `+c.reset+` WHERE label = 'Toren'`)
	}
	a.execSQL(t, `UPDATE combatants SET dismissed = true WHERE label = 'Toren'`)
	_, err = a.fire(t, a.master, e, id)
	wantRefusal(t, "a fire of a dismissed readier", err, connect.CodeNotFound, "readied action not found")
	a.execSQL(t, `UPDATE combatants SET dismissed = false WHERE label = 'Toren'`)
	if n := a.events(t); n != before || a.readiedStatus(t, "Toren") != readiedWaiting || len(a.windows(t, a.master)) != 0 {
		t.Fatalf("after the refusals: events %d (want %d), rows %q, windows %v", n, before, a.readiedStatus(t, "Toren"), a.windows(t, a.master))
	}
	a.mustFire(t, e, "Toren")
	_, err = a.fire(t, a.master, e, id)
	wantBlockedBy(t, "a second fire", err, blockedReactionPending)
	if n := len(a.windows(t, a.master)); n != 1 {
		t.Errorf("windows after the second fire = %d, want 1", n)
	}
}

// The same keys give the same answers: a second FireReadied opens no second window, a second "Usar" spends nothing more.
func TestFireReadiedIsIdempotent(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyThenPass(t, nil)
	req := &playv1.FireReadiedRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), ReadiedId: byLabel(t, a.get(t, a.master), "Toren").GetReadied().GetId(), IdempotencyKey: newKey(),
	}
	for range 2 {
		if _, err := a.master.ready.FireReadied(t.Context(), connect.NewRequest(req)); err != nil {
			t.Fatalf("FireReadied() error = %v", err)
		}
	}
	if n := len(a.windows(t, a.master)); n != 1 {
		t.Fatalf("windows after two fires under one key = %d", n)
	}
	w := a.windowOf(t, a.master, kindReadied)
	key := newKey()
	use := func(r *playv1.AnswerReactionRequest) { useTyped(2)(r); r.IdempotencyKey = key }
	a.mustAnswer(t, a.caio, e, w.GetId(), use)
	after := a.events(t)
	if res := a.mustAnswer(t, a.caio, e, w.GetId(), use); !res.GetResult().GetUsed() {
		t.Errorf("the replay's result = %v", res.GetResult())
	}
	if n := a.events(t); n != after || len(a.firedLines(t, a.master, e)) != 1 {
		t.Errorf("after the replay: events %d (want %d), fired lines %d", n, after, len(a.firedLines(t, a.master, e)))
	}
}

// A use refused by the attack's own checks spends nothing and the player may still pass: a target out of reach.
func TestFireReadiedAttackRefusedLeavesTheWindow(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyThenPass(t, nil)
	w := a.mustFire(t, e, "Toren")
	before := a.events(t)
	_, err := a.answerReaction(t, a.caio, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		useTyped(15)(r)
		r.CreatureIds = []string{a.id(t, "Pensantus")} // an ally, out of reach
	})
	if err == nil {
		t.Fatal("the attack on a creature out of reach was taken")
	}
	if n := a.events(t); n != before || byLabel(t, a.get(t, a.master), "Toren").GetReactionUsed() || !a.readiedOpen(t, "Toren") {
		t.Fatalf("after the refusal: events %d (want %d), window open %v", n, before, a.readiedOpen(t, "Toren"))
	}
	a.mustAnswer(t, a.caio, e, w.GetId(), passAnswer)
	if got := a.readiedStatus(t, "Toren"); got != readiedIgnored {
		t.Errorf("rows after the pass = %q", got)
	}
}

// An NPC readier: the master answers its window and names the target; Toren's player may not; Ana reads that the master is
// awaited. Its defeat closes another fired window by itself, and the row is lost.
func TestFireReadiedOfAnNPCIsTheMasters(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	e = a.onTurnOf(t, e, "Goblin")
	a.readyGoblinAttack(t, e)
	e = a.mustEndTurn(t, a.master, e) // Toren, round 2
	w := a.mustFire(t, e, "Goblin")
	if a.windowOf(t, a.caio, kindReadied) != nil {
		t.Error("Toren's player reads the goblin's window")
	}
	if got := a.get(t, a.ana).GetReactionWait().GetTitlePt(); got != "Esperando o mestre" {
		t.Errorf("Ana's wait = %q", got)
	}
	if _, err := a.answerReaction(t, a.caio, e, w.GetId(), passAnswer); err == nil {
		t.Error("Toren's player answered the goblin's window")
	}
	res := a.mustAnswer(t, a.master, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		useTyped(18)(r)
		r.CreatureIds = []string{a.id(t, "Toren")}
	})
	if !res.GetResult().GetByMaster() || !res.GetResult().GetUsed() {
		t.Errorf("the master's use = %v", res.GetResult())
	}
	if got := a.readiedStatus(t, "Goblin"); got != readiedFired {
		t.Errorf("the goblin's rows = %q", got)
	}

	b := newArmed(t)
	f := b.threeAndAGoblin(t)
	f = b.onTurnOf(t, f, "Goblin")
	b.readyGoblinAttack(t, f)
	f = b.mustEndTurn(t, b.master, f)
	b.mustFire(t, f, "Goblin")
	if _, err := b.adjustHP(t, f, "Goblin", damageHP(999)); err != nil {
		t.Fatalf("damage the goblin: %v", err)
	}
	if b.windowOf(t, b.master, kindReadied) != nil {
		t.Error("the defeated goblin's window is still open")
	}
	if got := b.readiedStatus(t, "Goblin"); got != readiedLost {
		t.Errorf("the goblin's rows after its defeat = %q", got)
	}
	if _, err := b.endTurn(t, b.caio, f, false); err != nil {
		t.Errorf("Toren's EndTurn after the window closed: %v", err)
	}
}

// Another action readied (Dodge) runs on the TakeAction path charged to the reaction; the master fires it while another
// creature's opportunity offer waits, and the two are answered apart.
func TestFireReadiedOtherInsideAnOpportunityOffer(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	e = a.mustEndTurn(t, a.master, e) // Pensantus
	a.mustReady(t, a.ana, &playv1.ReadyActionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Pensantus"), IdempotencyKey: newKey(),
		TriggerText: readyTrigger, Kind: playv1.ReadiedKind_READIED_KIND_OTHER, ActionKey: "standard:dodge",
	})
	e = a.onTurnOf(t, e, "Goblin")
	if _, err := a.frozenMove(t, a.master, "Goblin", 7, 3); err != nil {
		t.Fatalf("the goblin's move: %v", err)
	}
	offers := a.get(t, a.master).GetOpportunityOffers()
	if len(offers) != 1 || offers[0].GetReactorId() != a.id(t, "Toren") {
		t.Fatalf("the offers = %v, want Toren's", offers)
	}
	w := a.mustFire(t, e, "Pensantus")
	res := a.mustAnswer(t, a.ana, e, w.GetId(), useAnswer(0))
	if !res.GetResult().GetUsed() {
		t.Errorf("the use = %v", res.GetResult())
	}
	pens := byLabel(t, a.get(t, a.ana), "Pensantus")
	dodging := false
	for _, s := range pens.GetStates() {
		dodging = dodging || s.GetKind() == playv1.CombatantStateKind_COMBATANT_STATE_KIND_DODGING
	}
	if !dodging || !pens.GetReactionUsed() || pens.GetReactionKind() != kindReadied {
		t.Errorf("Pensantus after the use: dodging %v, reaction %v (%v)", dodging, pens.GetReactionUsed(), pens.GetReactionKind())
	}
	if got := a.readiedStatus(t, "Pensantus"); got != readiedFired {
		t.Errorf("Pensantus's rows = %q", got)
	}
	if got := a.get(t, a.master).GetOpportunityOffers(); len(got) != 1 {
		t.Fatalf("the offer after the use = %v, want it waiting", got)
	}
	if _, err := a.caio.combat.DeclineOpportunity(t.Context(), connect.NewRequest(&playv1.DeclineOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: offers[0].GetId(), IdempotencyKey: newKey(),
	})); err != nil {
		t.Errorf("Toren's decline: %v", err)
	}
	a.mustUndo(t, e) // the decline
	a.mustUndo(t, e) // the dodge
	if pens := byLabel(t, a.get(t, a.ana), "Pensantus"); pens.GetReactionUsed() || len(pens.GetStates()) != 0 || a.readiedStatus(t, "Pensantus") != readiedWaiting {
		t.Errorf("after the undo of the dodge: reaction %v, states %v, rows %q", pens.GetReactionUsed(), pens.GetStates(), a.readiedStatus(t, "Pensantus"))
	}
}

// A dismissed readier loses its row at the next change, as a defeated one; the undo of that change leaves it lost, and the
// Ready of a lost row cannot be undone (its line and row stay).
func TestADismissedReadierLosesItAndItsReadyStays(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	a.mustReady(t, a.caio, a.readyAttack(t, e))
	a.execSQL(t, `UPDATE combatants SET dismissed = true WHERE label = 'Toren'`)
	if _, err := a.adjustHP(t, e, "Goblin", damageHP(1)); err != nil {
		t.Fatalf("damage the goblin: %v", err)
	}
	if got := a.readiedStatus(t, "Toren"); got != readiedLost {
		t.Fatalf("Toren's rows after his dismissal = %q, want one lost", got)
	}
	lost := a.readiedLines(t, a.master, e, playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_LOST)
	if len(lost) != 1 || lost[0].GetActorLabel() != "Toren" {
		t.Errorf("the lost lines = %v", lost)
	}
	a.mustUndo(t, e) // the damage
	a.execSQL(t, `UPDATE combatants SET dismissed = false WHERE label = 'Toren'`)
	if id := a.log(t, a.master, e).GetUndoableEventId(); id != "" {
		t.Errorf("the Ready of a lost row can be undone (%s)", id)
	}
	if got := a.readiedStatus(t, "Toren"); got != readiedLost || len(a.readiedLines(t, a.master, e, playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION_READIED)) != 1 {
		t.Errorf("after the undo: rows %q", got)
	}
}
