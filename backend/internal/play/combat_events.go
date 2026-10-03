package play

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The payload of the events the actions of a turn write (docs/dados.md,
// session_events): IDs and numbers only, never a name or a free text
// (docs/privacidade.md), at most 4 KiB. The combat log (combat_log.go) is
// built from these events, and the undo (combat_undo.go) puts back what they
// say was before, so they carry both.

// The values of a pending damage's status (pending_damages_status_valid).
const (
	pendingAwaitingRoll = "awaiting_roll"
	pendingRolled       = "rolled"
	pendingApplied      = "applied"
	pendingDiscarded    = "discarded"
)

// The outcomes of an attack roll, as stored in an event.
const (
	outcomeHit  = "hit"
	outcomeCrit = "critical"
	outcomeMiss = "miss"
)

// hpState is the hit points of a combatant at one moment: the numbers an
// undo puts back.
type hpState struct {
	HP       int32 `json:"hp"`
	Temp     int32 `json:"temp,omitempty"`
	Defeated bool  `json:"defeated,omitempty"`
}

// actionEvent is the payload of every event of this slice, and of the 6.3
// move: one shape, each kind fills what it needs. Reading an event back
// (the log, the undo) decodes the same struct, so a field absent from the
// payload is its zero value.
type actionEvent struct {
	// Round is the combat's round when it happened.
	Round int32 `json:"round,omitempty"`
	// Secret says a hidden combatant was in it when it happened: the players
	// never get its line of the log, even after the master reveals the
	// combatant (RN-10, RN-20).
	Secret bool `json:"secret,omitempty"`
	// Actor is who did it (attacker, mover, the one that took the action or
	// whose hit points changed), Target who it was done to.
	Actor  string `json:"combatant_id,omitempty"`
	Target string `json:"target_id,omitempty"`
	// NowHidden is the new state of the combatant a combatant_hidden_set event
	// is about.
	NowHidden bool `json:"hidden,omitempty"`
	// Pending is the pending damage the attack opened and its damage follows.
	Pending string `json:"pending_id,omitempty"`
	// Key is the attack or the action.
	Key string `json:"key,omitempty"`

	// The attack roll: the d20, the bonus, the total and the outcome.
	D20      int32  `json:"d20,omitempty"`
	Modifier int32  `json:"modifier,omitempty"`
	Total    int32  `json:"total,omitempty"`
	Physical bool   `json:"physical,omitempty"`
	Outcome  string `json:"outcome,omitempty"`
	// TargetAC is the armor class the total was compared with: only the
	// master's answer carries it (RN-20), and a retry reads it back from here.
	TargetAC int32 `json:"target_ac,omitempty"`

	// The damage roll: the dice, their faces, the total and the type. Applied
	// says an NPC took it at once.
	DiceCount  int32   `json:"dice_count,omitempty"`
	DiceSides  int32   `json:"dice_sides,omitempty"`
	Faces      []int32 `json:"faces,omitempty"`
	Amount     int32   `json:"amount,omitempty"`
	DamageType string  `json:"damage_type,omitempty"`
	Critical   bool    `json:"critical,omitempty"`
	Applied    bool    `json:"applied,omitempty"`

	// Before and After are the target's hit points around a damage or the
	// master's hand.
	Before *hpState `json:"before,omitempty"`
	After  *hpState `json:"after,omitempty"`

	// What the undo of an action puts back.
	ActionBefore   bool   `json:"action_before,omitempty"`
	BonusBefore    bool   `json:"bonus_before,omitempty"`
	ReactionBefore bool   `json:"reaction_before,omitempty"`
	DashedBefore   bool   `json:"dashed_before,omitempty"`
	PrevStatus     string `json:"prev_status,omitempty"`
	// Mode is how the master changed an NPC's hit points ("damage", "heal",
	// "set"), and Delta the change.
	Mode  string `json:"mode,omitempty"`
	Delta int32  `json:"delta,omitempty"`

	// A move: the square, what it cost the combatant and how far it went.
	Col        int32 `json:"col,omitempty"`
	Row        int32 `json:"row,omitempty"`
	CostFt     int32 `json:"cost_ft,omitempty"`
	DistanceFt int32 `json:"distance_ft,omitempty"`
	OnTurn     bool  `json:"on_turn,omitempty"`

	// An undo: the event it took back.
	Undone     string `json:"undone_id,omitempty"`
	UndoneKind string `json:"undone_kind,omitempty"`
}

// readEvent decodes an event's payload. A payload of this module never fails
// to decode; an event that does is skipped by its readers.
func readEvent(payload []byte) (actionEvent, error) {
	var ev actionEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return actionEvent{}, fmt.Errorf("read an event payload: %w", err)
	}
	return ev, nil
}

// secretOf says whether a hidden combatant is one of the combatants.
func secretOf(cs ...playdb.Combatant) bool {
	return slices.ContainsFunc(cs, func(c playdb.Combatant) bool { return c.Hidden })
}

// touchesHidden says whether the attacker or the target of any of the
// pending damages is hidden now.
func touchesHidden(cs []playdb.Combatant, pending []playdb.PendingDamage) bool {
	hidden := map[string]bool{}
	for _, c := range cs {
		hidden[c.ID] = c.Hidden
	}
	return slices.ContainsFunc(pending, func(p playdb.PendingDamage) bool { return hidden[p.AttackerID] || hidden[p.TargetID] })
}

// hpOf is a combatant's hit points as the undo stores them.
func hpOf(c playdb.Combatant) hpState {
	return hpState{HP: num(c.HpCurrent), Temp: num(c.HpTemp), Defeated: c.Defeated}
}

// num is the value of an optional number, 0 when unset.
func num(n *int32) int32 {
	if n == nil {
		return 0
	}
	return *n
}

// pendingOf returns the damage the combatant's attacks still hold open:
// still to roll, or rolled and waiting for the master.
func (c *combatTx) pendingOf(ctx context.Context, attackerID string) ([]playdb.PendingDamage, error) {
	open, err := c.q.ListOpenPendingDamages(ctx, c.enc.ID)
	if err != nil {
		return nil, fmt.Errorf("list the pending damage: %w", err)
	}
	return slices.DeleteFunc(open, func(p playdb.PendingDamage) bool { return p.AttackerID != attackerID }), nil
}

// publishLogChanged tells the streams to read the combat log again: the
// master's always, the players' only when the change touches a line they may
// see.
func (s *Service) publishLogChanged(campaignID, encounterID string, players bool) {
	s.hub.Publish(campaignID, live.Event{
		Audience: live.Audience{Master: true, Players: players},
		Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_CombatLogChanged_{
			CombatLogChanged: &playv1.WatchGameSessionResponse_CombatLogChanged{EncounterId: encounterID},
		}},
	})
}
