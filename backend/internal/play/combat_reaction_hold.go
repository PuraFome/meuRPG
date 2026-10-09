package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// A held action (PM-04): a cast, an attack or a damage roll that a reaction
// window can change waits for the windows of its group. The request itself is
// kept (reaction_holds.request) before anything of it happens, and when every
// window of the group is answered or closed the server replays it, in the
// transaction of the last answer, as the action's own call would have run, with
// what the answers changed: a Counterspell that fails the spell, a Cutting Words
// die taken off a roll, half the damage of an Uncanny Dodge. Nothing is spent and
// nothing is rolled before the replay, so a held action that is dropped (the combat
// ended, or what it needed is gone) leaves no trace.

// replayState is what a replay of a held action carries: what the answers did.
type replayState struct {
	hold playdb.ReactionHold
	// countered: a Counterspell failed the spell. The cast spends its slot and its
	// action, and does nothing else.
	countered bool
	// reduction is what Cutting Words took off the roll (an attack's total, a
	// damage), and deflected what Deflect Missiles took off the damage.
	reduction int32
	deflected int32
	// halve: Uncanny Dodge halved the damage.
	halve bool
	// rolled is the damage roll made when the hold was written, so that the
	// reactor decided with the number it would take (Uncanny Dodge, Deflect
	// Missiles) and the replay lands that very roll.
	rolled *heldRoll
	// parts and partRolls are the parts of a damage with extras as they were rolled when
	// the hold was written: the replay lands those dice, not new ones.
	parts     []partRecord
	partRolls []partRoll
}

// answeredRef is an answer's event, kept to be completed once the replay it released
// says what the roll became.
type answeredRef struct {
	window string
	ev     *reactionEvent
}

// heldRoll is a damage roll made before the replay.
type heldRoll struct {
	Faces    []int32 `json:"faces,omitempty"`
	Total    int32   `json:"total"`
	Physical bool    `json:"physical,omitempty"`
}

// holdData is what a hold keeps besides the request.
type holdData struct {
	Rolled    *heldRoll    `json:"rolled,omitempty"`
	Parts     []partRecord `json:"parts,omitempty"`
	PartRolls []partRoll   `json:"part_rolls,omitempty"`
}

// The ambient write: a replay runs the action's own handler inside the
// transaction of the answer that released it. The handler calls s.write as always;
// write sees the ambient and runs the change in the open transaction instead of
// opening one, writes the event, and returns errAmbientDone so that the handler
// stops before it reads and publishes (the answer's handler does that, after the
// commit).
type ambient struct{ c *combatTx }

type ambientKey struct{}

func withAmbient(ctx context.Context, c *combatTx) context.Context {
	return context.WithValue(ctx, ambientKey{}, &ambient{c: c})
}

func ambientOf(ctx context.Context) *ambient {
	a, _ := ctx.Value(ambientKey{}).(*ambient)
	return a
}

// ambientDoneError is the error a replayed change ends with once its event is written.
type ambientDoneError struct{ event actionEvent }

func (*ambientDoneError) Error() string { return "the held action was replayed" }

// writeAmbient is write inside an open transaction (see ambient).
func (s *Service) writeAmbient(ctx context.Context, a *ambient, w combatWrite, do func(c *combatTx) (any, error)) (combatResult, error) {
	c := a.c
	kind, stamped, characterID := c.kind, c.stamped, c.characterID
	defer func() { c.kind, c.stamped, c.characterID = kind, stamped, characterID }()
	c.kind, c.characterID = w.kind, nil
	payload, err := do(c)
	if err != nil {
		return combatResult{}, err
	}
	ev, ok := payload.(actionEvent)
	if !ok {
		return combatResult{}, fmt.Errorf("a held action wrote an event that is not an action: %T", payload)
	}
	// The event of the replayed action carries no key: the request's own key
	// belongs to the event that held it.
	if err := insertEvent(ctx, c, w.kind, &w.m.UserID, nil, ev); err != nil {
		return combatResult{}, err
	}
	return combatResult{}, connect.NewError(connect.CodeUnknown, &ambientDoneError{event: ev})
}

// holdAction keeps a request that windows hold, opens the windows and returns the
// event that stands for the held change: it carries the request's idempotency key
// (a retry answers with it) and never reaches the log.
func (s *Service) holdAction(ctx context.Context, c *combatTx, m authz.Membership, kind string, actor playdb.Combatant, msg proto.Message, data holdData, specs []windowSpec, ev actionEvent) (actionEvent, error) {
	body, err := proto.Marshal(msg)
	if err != nil {
		return ev, fmt.Errorf("keep the held request: %w", err)
	}
	group := newGroup()
	dataBody, err := json.Marshal(data)
	if err != nil {
		return ev, err
	}
	hold, err := c.q.InsertReactionHold(ctx, playdb.InsertReactionHoldParams{
		EncounterID: c.enc.ID, GroupID: group, Kind: kind, ActorID: actor.ID, ActorUserID: m.UserID, ActorIsMaster: m.Role == authz.RoleMaster,
		Request: body, Data: dataBody, CreatedAt: c.now,
	})
	if err != nil {
		return ev, fmt.Errorf("hold the action: %w", err)
	}
	if _, err := s.openWindows(ctx, c, group, &hold.ID, specs); err != nil {
		return ev, err
	}
	ev.Reaction = &reactionEvent{Hold: true, Group: group}
	return ev, nil
}

// releaseHold replays a held action once its windows are all closed. A change
// the replay refuses (a client error: the target is gone, the slot was spent) drops
// the hold, as the table would: the action does not happen and nothing was spent.
func (s *Service) releaseHold(ctx context.Context, c *combatTx, h playdb.ReactionHold) error {
	windows, err := c.q.ListReactionWindowsOfGroup(ctx, playdb.ListReactionWindowsOfGroupParams{EncounterID: c.enc.ID, GroupID: h.GroupID})
	if err != nil {
		return fmt.Errorf("list the windows of a held action: %w", err)
	}
	st := &replayState{hold: h}
	var data holdData
	_ = json.Unmarshal(h.Data, &data)
	st.rolled = data.Rolled
	st.parts, st.partRolls = data.Parts, data.PartRolls
	for _, w := range windows {
		if w.Status != windowAnswered {
			continue
		}
		out := windowOutcomeOf(w)
		if !out.Used {
			continue
		}
		switch reaction.Kind(w.Kind) {
		case reaction.CounterspellKind:
			st.countered = st.countered || out.Countered
		case reaction.CuttingWords:
			st.reduction += out.Reduction
		case reaction.UncannyDodgeKind:
			st.halve = true
		case reaction.DeflectKind:
			st.deflected += out.Reduction
		}
	}
	if err := c.q.SetReactionHoldState(ctx, playdb.SetReactionHoldStateParams{ID: h.ID, State: "released"}); err != nil {
		return fmt.Errorf("release a held action: %w", err)
	}
	m := authz.Membership{CampaignID: c.session.CampaignID, UserID: h.ActorUserID, Role: authz.RolePlayer}
	if h.ActorIsMaster {
		m.Role = authz.RoleMaster
	}
	c.replay = st
	defer func() { c.replay = nil }()
	ctx = withAmbient(ctx, c)
	switch h.Kind {
	case "cast":
		var req playv1.CastSpellRequest
		if err := proto.Unmarshal(h.Request, &req); err != nil {
			return fmt.Errorf("read a held cast: %w", err)
		}
		_, err = s.castSpell(ctx, m, connect.NewRequest(&req))
	case "attack":
		var req playv1.RollAttackRequest
		if err := proto.Unmarshal(h.Request, &req); err != nil {
			return fmt.Errorf("read a held attack: %w", err)
		}
		_, err = s.rollAttack(ctx, m, connect.NewRequest(&req))
	case "damage":
		var req playv1.RollDamageRequest
		if err := proto.Unmarshal(h.Request, &req); err != nil {
			return fmt.Errorf("read a held damage roll: %w", err)
		}
		_, err = s.rollDamage(ctx, m, connect.NewRequest(&req))
	default:
		return fmt.Errorf("a held action of kind %q", h.Kind)
	}
	var done *ambientDoneError
	switch {
	case errors.As(err, &done):
		c.tellActorVitals(done.event)
		s.completeAnswers(c, windows, st, done.event)
		return nil
	case err != nil && clientError(err):
		// The action cannot happen any more: it is dropped, and the master reads that it was, since the
		// answers may have spent slots and reactions already.
		if err := c.q.SetReactionHoldState(ctx, playdb.SetReactionHoldStateParams{ID: h.ID, State: "dropped"}); err != nil {
			return err
		}
		ev := actionEvent{Round: c.enc.Round, Secret: true, Actor: h.ActorID, Reaction: &reactionEvent{Kind: reactionHeldDropped, Used: true, Group: h.GroupID, Roll: h.Kind, Why: err.Error()}}
		return insertEvent(ctx, c, eventReactionAnswered, &c.actorUserID, nil, ev)
	case err != nil:
		return err
	}
	return fmt.Errorf("the replay of a held %s ended without an event", h.Kind)
}

// clientError says an error is the caller's to fix (a refusal), not a failure of
// the server: only these drop a held action; anything else is retried with the
// transaction.
func clientError(err error) bool {
	switch connect.CodeOf(err) {
	case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeNotFound, connect.CodePermissionDenied, connect.CodeOutOfRange, connect.CodeAlreadyExists:
		return true
	}
	return false
}

// tellActorVitals queues the vitals of the player characters a replayed action
// changed (its actor's slot, a target's hit points) to be told after the commit.
func (c *combatTx) tellActorVitals(ev actionEvent) {
	c.tellIDs = append(c.tellIDs, ev.Actor, ev.Target)
	for _, h := range ev.Hits {
		c.tellIDs = append(c.tellIDs, h.Target)
	}
	for _, h := range ev.Settled {
		c.tellIDs = append(c.tellIDs, h.Target)
	}
	slices.Sort(c.tellIDs)
	c.tellIDs = slices.Compact(c.tellIDs)
}

// completeAnswers tells the answers that released a held action what it became: an
// attack roll's outcome, total and armor class (the master's log reads "17 → 13 contra CA
// 15"), and a damage's final amount.
func (s *Service) completeAnswers(c *combatTx, windows []playdb.ReactionWindow, st *replayState, done actionEvent) {
	for _, ref := range c.answered {
		if !slices.ContainsFunc(windows, func(w playdb.ReactionWindow) bool { return w.ID == ref.window }) {
			continue
		}
		switch st.hold.Kind {
		case "attack":
			ref.ev.Outcome, ref.ev.AC = done.Outcome, done.TargetAC
			ref.ev.After = done.Total
			ref.ev.Before = done.Total + st.reduction
		case "damage":
			ref.ev.Before = st.rolledTotal()
			ref.ev.After = done.Amount
		}
	}
}

// rolledTotal is the damage the hold rolled.
func (st *replayState) rolledTotal() int32 {
	if st.rolled == nil {
		return 0
	}
	return st.rolled.Total
}
