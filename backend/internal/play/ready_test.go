package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Ready (ReadyService, ready.go): what it checks and in which order, what it spends, the one waiting row, the loss at the
// start of the readier's next turn, the undo, CancelReadied and who reads the readied action (RN-10).

var (
	blockedReadyTrigger = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_READY_TRIGGER
	blockedReadyAction  = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_READY_ACTION
)

const readyTrigger = "Se o goblin sair de trás da caixa"

// readyAttack is a request that readies Toren's battleaxe against the goblin.
func (a *armed) readyAttack(t *testing.T, e *playv1.Encounter) *playv1.ReadyActionRequest {
	t.Helper()
	return &playv1.ReadyActionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Toren"), IdempotencyKey: newKey(),
		TriggerText: readyTrigger, Kind: playv1.ReadiedKind_READIED_KIND_ATTACK, ActionKey: battleaxe, TargetCombatantId: a.id(t, "Goblin"),
	}
}

func (a *armed) ready(t *testing.T, u *user, req *playv1.ReadyActionRequest) (*playv1.Encounter, error) {
	t.Helper()
	res, err := u.ready.ReadyAction(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetEncounter(), nil
}

func (a *armed) mustReady(t *testing.T, u *user, req *playv1.ReadyActionRequest) *playv1.Encounter {
	t.Helper()
	out, err := a.ready(t, u, req)
	if err != nil {
		t.Fatalf("ReadyAction() error = %v", err)
	}
	return out
}

func (a *armed) cancelReadied(t *testing.T, u *user, e *playv1.Encounter, id string) error {
	t.Helper()
	_, err := u.ready.CancelReadied(t.Context(), connect.NewRequest(&playv1.CancelReadiedRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), ReadiedId: id, IdempotencyKey: newKey(),
	}))
	return err
}

// readiedStatus is the status of the readied rows of a combatant, oldest first.
func (a *armed) readiedStatus(t *testing.T, label string) string {
	t.Helper()
	rows, err := a.h.pool.Query(t.Context(), `SELECT r.status FROM readied_actions r JOIN combatants c ON c.id = r.combatant_id WHERE c.label = $1 ORDER BY r.created_at, r.id`, label)
	if err != nil {
		t.Fatalf("read the readied actions: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return strings.Join(out, ",")
}

// logKinds are the lines of a kind in u's log, with the readied detail each carries.
func (a *armed) readiedLines(t *testing.T, u *user, e *playv1.Encounter, kind playv1.CombatLogKind) []*playv1.CombatLogEntry {
	t.Helper()
	var out []*playv1.CombatLogEntry
	for _, r := range a.log(t, u, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == kind {
				out = append(out, en)
			}
		}
	}
	return out
}

// Toren readies his battleaxe: the action is spent, the row waits, and the readied action is his player's and the
// master's to read; Ana reads that Toren readied an action, and nothing of what or when.
func TestReadyActionHoldsAnAttackForItsTrigger(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	a.mustReady(t, a.caio, a.readyAttack(t, e))

	if got := a.readiedStatus(t, "Toren"); got != readiedWaiting {
		t.Errorf("Toren's readied rows = %q, want one waiting", got)
	}
	for _, u := range []*user{a.caio, a.master} {
		toren := byLabel(t, a.get(t, u), "Toren")
		if !toren.GetActionUsed() {
			t.Errorf("Toren's action is free after Ready")
		}
		r := toren.GetReadied()
		if r.GetTriggerText() != readyTrigger || r.GetKind() != playv1.ReadiedKind_READIED_KIND_ATTACK || r.GetActionKey() != battleaxe ||
			r.GetTargetCombatantId() != a.id(t, "Goblin") || r.GetStatus() != playv1.ReadiedStatus_READIED_STATUS_WAITING || r.GetRound() != 1 {
			t.Errorf("Toren's readied action as %s = %v", u.id, r)
		}
		lines := a.readiedLines(t, u, e, playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION_READIED)
		if len(lines) != 1 || lines[0].GetReadied().GetTriggerText() != readyTrigger || lines[0].GetReadied().GetActionKey() != battleaxe {
			t.Errorf("the readied lines of %s = %v, want one with the trigger", u.id, lines)
		}
	}
	if r := byLabel(t, a.get(t, a.ana), "Toren").GetReadied(); r != nil {
		t.Errorf("Ana reads Toren's readied action: %v", r)
	}
	lines := a.readiedLines(t, a.ana, e, playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION_READIED)
	if len(lines) != 1 || lines[0].GetReadied() != nil || lines[0].GetActorLabel() != "Toren" || lines[0].GetKey() != "" {
		t.Errorf("Ana's readied lines = %v, want one with the actor alone", lines)
	}
}

// The checks in order: the turn, the action, a move with no speed, the trigger, the action Ready holds, the target; none
// writes anything.
func TestReadyActionRefusals(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t) // Toren acts
	edit := func(f func(r *playv1.ReadyActionRequest)) *playv1.ReadyActionRequest {
		r := a.readyAttack(t, e)
		f(r)
		return r
	}
	long := strings.Repeat("a", readyTriggerMax+1)
	cases := []struct {
		name   string
		u      *user
		req    *playv1.ReadyActionRequest
		reason playv1.EncounterBlockedReason
		code   connect.Code
	}{
		{"Pensantus off her turn, with no trigger", a.ana, edit(func(r *playv1.ReadyActionRequest) { r.CombatantId = a.id(t, "Pensantus"); r.TriggerText = "" }), blockedNotYourTurn, 0},
		{"another player's combatant", a.ana, a.readyAttack(t, e), 0, connect.CodePermissionDenied},
		{"an empty trigger", a.caio, edit(func(r *playv1.ReadyActionRequest) { r.TriggerText = "   " }), blockedReadyTrigger, 0},
		{"a trigger of 201 characters", a.caio, edit(func(r *playv1.ReadyActionRequest) { r.TriggerText = long }), blockedReadyTrigger, 0},
		{"an attack Toren does not have", a.caio, edit(func(r *playv1.ReadyActionRequest) { r.ActionKey = rapier }), blockedReadyAction, 0},
		{"an attack with no key", a.caio, edit(func(r *playv1.ReadyActionRequest) { r.ActionKey = "" }), blockedReadyAction, 0},
		{"the Attack action as another action", a.caio, edit(func(r *playv1.ReadyActionRequest) {
			r.Kind = playv1.ReadiedKind_READIED_KIND_OTHER
			r.ActionKey = standardAttack
		}), blockedReadyAction, 0},
		{"Ready as another action", a.caio, edit(func(r *playv1.ReadyActionRequest) {
			r.Kind = playv1.ReadiedKind_READIED_KIND_OTHER
			r.ActionKey = actionReady
		}), blockedReadyAction, 0},
		{"a feature as another action", a.caio, edit(func(r *playv1.ReadyActionRequest) {
			r.Kind = playv1.ReadiedKind_READIED_KIND_OTHER
			r.ActionKey = "feature:second-wind"
		}), blockedReadyAction, 0},
		{"a spell (not offered yet)", a.caio, edit(func(r *playv1.ReadyActionRequest) {
			r.Kind = playv1.ReadiedKind_READIED_KIND_SPELL
			r.SpellKey = "spell:fire-bolt"
		}), blockedReadyAction, 0},
		{"no kind", a.caio, edit(func(r *playv1.ReadyActionRequest) { r.Kind = playv1.ReadiedKind_READIED_KIND_UNSPECIFIED }), blockedReadyAction, 0},
		{"a target nobody has", a.caio, edit(func(r *playv1.ReadyActionRequest) { r.TargetCombatantId = newKey() }), 0, connect.CodeNotFound},
	}
	before := a.events(t)
	for _, c := range cases {
		_, err := a.ready(t, c.u, c.req)
		if c.code != 0 {
			if connect.CodeOf(err) != c.code {
				t.Errorf("%s: error = %v, want %v", c.name, err, c.code)
			}
			continue
		}
		wantBlockedBy(t, c.name, err, c.reason)
	}
	// A move with no speed is refused before the trigger is read.
	a.execSQL(t, `UPDATE combatants SET conditions = ARRAY['condition:grappled'] WHERE label = 'Toren'`)
	_, err := a.ready(t, a.caio, edit(func(r *playv1.ReadyActionRequest) {
		r.Kind = playv1.ReadiedKind_READIED_KIND_MOVE
		r.ActionKey = ""
		r.TriggerText = ""
	}))
	wantBlockedBy(t, "a grappled Toren readying a move", err, blockedReadyAction)
	if n := a.events(t); n != before {
		t.Errorf("events after the refusals = %d, want %d", n, before)
	}
	if got := a.readiedStatus(t, "Toren"); got != "" {
		t.Errorf("rows after the refusals = %q, want none", got)
	}
	if byLabel(t, a.get(t, a.master), "Toren").GetActionUsed() {
		t.Error("a refusal spent Toren's action")
	}
}

// The economy: the action is spent, a second Ready is refused as the action is used, and the master, who may act with
// the action used, is refused too while one waits: a combatant has one readied action. A trigger of 200 characters,
// spaces around it trimmed, is kept.
func TestReadyActionSpendsTheActionAndHoldsOne(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	trigger := strings.Repeat("é", readyTriggerMax)
	req := a.readyAttack(t, e)
	req.Kind, req.ActionKey, req.TargetCombatantId, req.TriggerText = playv1.ReadiedKind_READIED_KIND_MOVE, "", "", "  "+trigger+"  "
	got := a.mustReady(t, a.caio, req)
	toren := byLabel(t, got, "Toren")
	if r := toren.GetReadied(); r.GetTriggerText() != trigger || r.GetMoveFt() != 30 || r.GetActionKey() != "" {
		t.Errorf("the readied move = %v, want the trimmed trigger and 30 ft", r)
	}
	_, err := a.ready(t, a.caio, a.readyAttack(t, e))
	wantBlockedBy(t, "a second Ready", err, blockedActionUsed)
	_, err = a.ready(t, a.master, a.readyAttack(t, e))
	wantBlockedBy(t, "the master's second Ready", err, blockedReadyAction)
	if got := a.readiedStatus(t, "Toren"); got != readiedWaiting {
		t.Errorf("rows = %q, want one waiting", got)
	}
	other := a.readyAttack(t, e)
	other.IdempotencyKey = req.GetIdempotencyKey() // the key of the first Ready: the first answer, nothing new
	if _, err := a.ready(t, a.caio, req); err != nil {
		t.Errorf("the retry of the first Ready: %v", err)
	}
	if _, err := a.ready(t, a.caio, other); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("another request under the first key: %v, want invalid_argument", err)
	}
}

// TakeAction refuses Ready, which names a trigger, and the options say where it is taken.
func TestReadyIsNotTakenWithTakeAction(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	_, err := a.action(t, a.caio, e, "Toren", actionReady)
	wantRefusal(t, "TakeAction(standard:ready)", err, connect.CodeInvalidArgument, "standard:ready is taken with ReadyService.ReadyAction, which names the trigger")
	found := false
	for _, o := range a.mustOptions(t, a.caio, e, "Toren").GetOptions().GetStandardActions() {
		if o.GetAction().GetKey() == actionReady {
			found = o.GetViaReadyAction()
		} else if o.GetViaReadyAction() {
			t.Errorf("%s is marked via_ready_action", o.GetAction().GetKey())
		}
	}
	if !found {
		t.Error("Ready is not marked via_ready_action")
	}
}

// The readied action is lost when Toren's next turn starts: the row is lost, the line says so (the trigger the master's and
// Toren's player's), and the reaction he never spent is his.
func TestReadiedActionIsLostAtTheStartOfTheNextTurn(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	a.mustReady(t, a.caio, a.readyAttack(t, e))
	for range 4 { // Toren, Pensantus, Brisa, the goblin: Toren's turn again
		e = a.mustEndTurn(t, a.master, e)
	}
	if got := a.get(t, a.master); got.GetCurrentCombatantId() != a.id(t, "Toren") || got.GetRound() != 2 {
		t.Fatalf("on turn %s in round %d, want Toren in round 2", got.GetCurrentCombatantId(), got.GetRound())
	}
	if got := a.readiedStatus(t, "Toren"); got != readiedLost {
		t.Errorf("rows = %q, want one lost", got)
	}
	toren := byLabel(t, a.get(t, a.caio), "Toren")
	if toren.GetReadied() != nil || toren.GetReactionUsed() {
		t.Errorf("Toren after the loss: readied %v, reaction used %v", toren.GetReadied(), toren.GetReactionUsed())
	}
	for _, u := range []*user{a.caio, a.master} {
		lines := a.readiedLines(t, u, e, playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_LOST)
		if len(lines) != 1 || lines[0].GetReadiedLost().GetTriggerText() != readyTrigger || lines[0].GetReadiedLost().GetCanceled() {
			t.Errorf("%s's lost lines = %v", u.id, lines)
		}
	}
	lines := a.readiedLines(t, a.ana, e, playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_LOST)
	if len(lines) != 1 || lines[0].GetReadiedLost() != nil {
		t.Errorf("Ana's lost lines = %v, want one with the actor alone", lines)
	}
	// Toren may ready again on his new turn.
	a.mustReady(t, a.caio, a.readyAttack(t, e))
}

// The master's undo of a Ready gives the action back and takes the row and its line away.
func TestUndoOfReadyGivesTheActionBack(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	a.mustReady(t, a.caio, a.readyAttack(t, e))
	a.mustUndo(t, e)
	toren := byLabel(t, a.get(t, a.caio), "Toren")
	if toren.GetActionUsed() || toren.GetReadied() != nil {
		t.Errorf("Toren after the undo: action used %v, readied %v", toren.GetActionUsed(), toren.GetReadied())
	}
	if got := a.readiedStatus(t, "Toren"); got != "" {
		t.Errorf("rows after the undo = %q, want none", got)
	}
	for _, u := range []*user{a.caio, a.master, a.ana} {
		if lines := a.readiedLines(t, u, e, playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION_READIED); len(lines) != 0 {
			t.Errorf("%s still reads %d readied lines", u.id, len(lines))
		}
	}
	a.mustReady(t, a.caio, a.readyAttack(t, e))
}

// CancelReadied: the owner gives it up on his turn and the action stays spent; another player is told it does not exist,
// and off the readier's turn it is refused.
func TestCancelReadied(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	id := byLabel(t, a.mustReady(t, a.caio, a.readyAttack(t, e)), "Toren").GetReadied().GetId()
	wantRefusal(t, "Ana's cancel", a.cancelReadied(t, a.ana, e, id), connect.CodeNotFound, "readied action not found")
	wantRefusal(t, "a cancel of nothing", a.cancelReadied(t, a.caio, e, newKey()), connect.CodeNotFound, "readied action not found")
	e = a.mustEndTurn(t, a.master, e)
	wantBlockedBy(t, "a cancel off Toren's turn", a.cancelReadied(t, a.caio, e, id), blockedNotYourTurn)

	b := newArmed(t)
	f := b.threeAndAGoblin(t)
	id = byLabel(t, b.mustReady(t, b.caio, b.readyAttack(t, f)), "Toren").GetReadied().GetId()
	if err := b.cancelReadied(t, b.caio, f, id); err != nil {
		t.Fatalf("CancelReadied() error = %v", err)
	}
	toren := byLabel(t, b.get(t, b.caio), "Toren")
	if toren.GetReadied() != nil || !toren.GetActionUsed() {
		t.Errorf("Toren after the cancel: readied %v, action used %v", toren.GetReadied(), toren.GetActionUsed())
	}
	if got := b.readiedStatus(t, "Toren"); got != readiedCanceled {
		t.Errorf("rows = %q, want one canceled", got)
	}
	lines := b.readiedLines(t, b.caio, f, playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_LOST)
	if len(lines) != 1 || !lines[0].GetReadiedLost().GetCanceled() {
		t.Errorf("the cancel's lines = %v", lines)
	}
}

// The end of the combat drops the readied actions that wait.
func TestEndOfCombatDropsTheReadiedActions(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	a.mustReady(t, a.caio, a.readyAttack(t, e))
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	if got := a.readiedStatus(t, "Toren"); got != "" {
		t.Errorf("rows after the end = %q, want none", got)
	}
}
