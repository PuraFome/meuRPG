package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// readyTriggerMax is the longest trigger, in characters once trimmed (readied_actions_trigger_valid).
const readyTriggerMax = 200

// readyCall is what ReadyAction reads from its request.
type readyCall struct {
	take    takeActionCall
	trigger string
	kind    string // "" for a kind the request did not name
	key     string
	target  string
}

// readReadyCall reads the ids of the request; the trigger and the action are checked in the transaction, after the turn.
func readReadyCall(req *playv1.ReadyActionRequest, key, encID string) (readyCall, error) {
	call := readyCall{
		take:    takeActionCall{key: key, encounterID: encID, actionKey: actionReady},
		trigger: req.GetTriggerText(), kind: readiedKinds[req.GetKind()], key: req.GetActionKey(),
	}
	var err error
	if call.take.combatantID, err = parseCombatID(req.GetCombatantId(), "combatant"); err != nil {
		return call, err
	}
	if t := req.GetTargetCombatantId(); t != "" {
		if call.target, err = parseCombatID(t, "target"); err != nil {
			return call, err
		}
	}
	return call, nil
}

// readyFlow is one run of ReadyAction's transaction: the steps Ready shares with TakeAction (take), and its own.
type readyFlow struct {
	take *takeActionFlow
	call readyCall
	// name is the held action's name, for the event.
	name string
}

func newReadyFlow(s *Service, c *combatTx, m authz.Membership, call readyCall) *readyFlow {
	return &readyFlow{take: newTakeActionFlow(s, c, m, call.take, &takeActionOut{}), call: call}
}

// run checks, in order, the combatant, its turn and its action, then what Ready holds; spends the action and writes the
// row. It answers the event the write stores.
func (f *readyFlow) run(ctx context.Context) (actionEvent, error) {
	t := f.take
	if err := t.locate(ctx); err != nil {
		return actionEvent{}, err
	}
	if err := t.choose(); err != nil {
		return actionEvent{}, err
	}
	if err := t.mayTake(ctx); err != nil {
		return actionEvent{}, err
	}
	if err := f.check(ctx); err != nil {
		return actionEvent{}, err
	}
	if err := t.begin(ctx); err != nil {
		return actionEvent{}, err
	}
	if err := t.spend(ctx); err != nil {
		return actionEvent{}, err
	}
	row, err := f.insert(ctx)
	if err != nil {
		return actionEvent{}, err
	}
	t.made.Readied = readiedEventOf(row, f.name)
	if err := touch(ctx, t.c); err != nil {
		return actionEvent{}, err
	}
	t.c.characterID = &t.who.CharacterID
	return t.made, nil
}

// check is what Ready asks besides the turn: a speed for a move, the trigger, an action the combatant may hold, a target
// it sees, and no other readied action waiting.
func (f *readyFlow) check(ctx context.Context) error {
	if f.call.kind == readiedMove && speedDFt(f.take.who) == 0 {
		return errReadyAction("a combatant with no speed cannot ready a move")
	}
	trigger := strings.TrimSpace(f.call.trigger)
	if trigger == "" || utf8.RuneCountInString(trigger) > readyTriggerMax {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_READY_TRIGGER, "the trigger must have 1 to 200 characters")
	}
	f.call.trigger = trigger
	if err := f.checkAction(); err != nil {
		return err
	}
	if err := f.checkTarget(ctx); err != nil {
		return err
	}
	return f.notReadied(ctx)
}

// checkAction finds the held action among the combatant's options and keeps its name.
func (f *readyFlow) checkAction() error {
	opts := f.take.opts
	switch f.call.kind {
	case readiedAttack:
		i := slices.IndexFunc(opts.GetAttacks(), func(a *rulesv1.AttackOption) bool { return a.GetAttack().GetKey() == f.call.key })
		if f.call.key == "" || i < 0 {
			return errReadyAction("the attack is not one the combatant has")
		}
		f.name = opts.GetAttacks()[i].GetAttack().GetNamePt()
	case readiedOther:
		i := slices.IndexFunc(opts.GetStandardActions(), func(a *rulesv1.ActionOption) bool { return a.GetAction().GetKey() == f.call.key })
		if i < 0 || slices.Contains(unreadiable, f.call.key) {
			return errReadyAction("the action is not a standard action that can be readied")
		}
		f.name = opts.GetStandardActions()[i].GetAction().GetNamePt()
	case readiedMove:
		f.call.key = ""
	default:
		// A spell (READIED_KIND_SPELL) is not offered until its hold and release are written: the casting time gate
		// (READY_SPELL_TIME), the slot and the concentration go here. Until then it is refused like any action that
		// cannot be readied.
		return errReadyAction("this kind of action cannot be readied")
	}
	return nil
}

// unreadiable are the standard actions Ready cannot hold: the attack is readied by its key, a spell by its own kind, and
// Ready itself holds nothing.
var unreadiable = []string{standardAttack, standardCast, actionReady}

// checkTarget checks that the creature the player named is one the caller sees.
func (f *readyFlow) checkTarget(ctx context.Context) error {
	if f.call.target == "" {
		return nil
	}
	cs, err := listCombatants(ctx, f.take.c)
	if err != nil {
		return err
	}
	_, err = findCombatant(cs, f.call.target, f.take.v)
	return err
}

// notReadied refuses a second readied action while one waits (Action Surge gives a second action).
func (f *readyFlow) notReadied(ctx context.Context) error {
	_, err := f.take.c.q.GetWaitingReadiedOf(ctx, f.take.who.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read the readied action: %w", err)
	}
	return errReadyAction("the combatant holds a readied action already")
}

// insert writes the row.
func (f *readyFlow) insert(ctx context.Context) (playdb.ReadiedAction, error) {
	who := f.take.who
	p := playdb.InsertReadiedActionParams{
		EncounterID: f.take.c.enc.ID, CombatantID: who.ID, Round: f.take.c.enc.Round, TriggerText: f.call.trigger,
		Kind: f.call.kind, ActionKey: nilIfEmpty(f.call.key), TargetID: nilIfEmpty(f.call.target), CreatedAt: f.take.c.now,
	}
	if f.call.kind == readiedMove {
		p.MoveFt = readyMoveFt(who)
	}
	row, err := f.take.c.q.InsertReadiedAction(ctx, p)
	if err != nil {
		return row, fmt.Errorf("write the readied action: %w", err)
	}
	return row, nil
}

// readyMoveFt is the speed a readied move may go, in feet: the combatant's speed now, without this turn's Dash.
func readyMoveFt(who playdb.Combatant) int32 {
	who.Dashed = false
	return clamp32(speedDFt(who)/dftPerFt, 0, math.MaxInt32)
}

// errReadyAction refuses an action Ready cannot hold.
func errReadyAction(msg string) error {
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_READY_ACTION, msg)
}

// cancelReadiedTx gives up the readied action id: only one that waits, the caller's own combatant's (or the master's),
// on the readier's turn. The action spent to ready it stays spent.
func (s *Service) cancelReadiedTx(ctx context.Context, c *combatTx, m authz.Membership, id string) (actionEvent, error) {
	if err := notEnded(c.enc); err != nil {
		return actionEvent{}, err
	}
	row, err := c.q.GetWaitingReadiedAction(ctx, playdb.GetWaitingReadiedActionParams{ID: id, EncounterID: c.enc.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return actionEvent{}, errReadiedNotFound()
	}
	if err != nil {
		return actionEvent{}, fmt.Errorf("read the readied action: %w", err)
	}
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return actionEvent{}, err
	}
	v := c.viewer(m, cs)
	who, err := findCombatant(cs, row.CombatantID, v)
	if err != nil || v.mayAct(who) != nil {
		return actionEvent{}, errReadiedNotFound()
	}
	if err := s.mustActNow(ctx, c, who); err != nil {
		return actionEvent{}, err
	}
	if err := c.q.SetReadiedActionStatus(ctx, playdb.SetReadiedActionStatusParams{ID: row.ID, Status: readiedCanceled}); err != nil {
		return actionEvent{}, fmt.Errorf("give up the readied action: %w", err)
	}
	if err := touch(ctx, c); err != nil {
		return actionEvent{}, err
	}
	c.characterID = &who.CharacterID
	ev := readiedLostEvent(row, who, c.enc.Round)
	ev.Readied.Canceled = true
	return ev, nil
}

// readiedLostEvent is the event of a readied action that ended without firing.
func readiedLostEvent(row playdb.ReadiedAction, who playdb.Combatant, round int32) actionEvent {
	return actionEvent{Round: round, Secret: who.Hidden, Actor: who.ID, Readied: readiedEventOf(row, "")}
}
