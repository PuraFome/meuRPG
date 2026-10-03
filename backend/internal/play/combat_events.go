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
	pendingAwaitingReaction = "awaiting_reaction"
	pendingAwaitingRoll     = "awaiting_roll"
	pendingRolled           = "rolled"
	pendingApplied          = "applied"
	pendingDiscarded        = "discarded"
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

// slotRef is the spell slot a spell spent: its level, and whether it was a
// pact magic slot. The undo gives it back.
type slotRef struct {
	Level int32 `json:"level"`
	Pact  bool  `json:"pact,omitempty"`
}

// deathState is a combatant's death save counts and whether its turn's save was
// rolled, and whether it was out of the fight (a confirmed death): what an undo
// puts back.
type deathState struct {
	Successes int32 `json:"successes,omitempty"`
	Failures  int32 `json:"failures,omitempty"`
	Rolled    bool  `json:"rolled,omitempty"`
	Dead      bool  `json:"dead,omitempty"`
}

func deathOf(c playdb.Combatant) *deathState {
	return &deathState{Successes: c.DeathSuccesses, Failures: c.DeathFailures, Rolled: c.DeathSaveRolled, Dead: c.Defeated}
}

// saveRoll is a target's saving throw against a spell, as the cast event keeps it.
type saveRoll struct {
	D20   int32 `json:"d20"`
	Bonus int32 `json:"bonus,omitempty"`
	Total int32 `json:"total"`
	DC    int32 `json:"dc"`
	Saved bool  `json:"saved,omitempty"`
	// Unknown says the target is a basic-sheet NPC with no saving throw bonus:
	// the roll is d20 + 0 and the master may overrule it.
	Unknown bool `json:"bonus_unknown,omitempty"`
}

// castHit is what a cast did to one target.
type castHit struct {
	Target string `json:"target_id"`
	Darts  int32  `json:"darts,omitempty"`
	// A spell attack: the d20, the bonus, the total and the outcome.
	Outcome  string    `json:"outcome,omitempty"`
	D20      int32     `json:"d20,omitempty"`
	Modifier int32     `json:"modifier,omitempty"`
	Total    int32     `json:"total,omitempty"`
	Physical bool      `json:"physical,omitempty"`
	Save     *saveRoll `json:"save,omitempty"`
	// Pending is the pending damage or heal the cast opened for the target.
	Pending string `json:"pending_id,omitempty"`
}

// damageHit is what a damage roll did to one pending damage of a cast: the
// amount that landed (half for a target that saved), and the numbers an undo
// puts back.
type damageHit struct {
	Pending     string      `json:"pending_id"`
	Target      string      `json:"target_id"`
	Amount      int32       `json:"amount"`
	Half        bool        `json:"half,omitempty"`
	Applied     bool        `json:"applied,omitempty"`
	Before      *hpState    `json:"before,omitempty"`
	After       *hpState    `json:"after,omitempty"`
	DeathBefore *deathState `json:"death_before,omitempty"`
	// ConcentrationDC is the save DC a damage that landed threatens a
	// concentration with (RN-22), 0 when it does not.
	ConcentrationDC int32 `json:"concentration_dc,omitempty"`
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

	// A spell cast (spell_cast) and a reaction: the cast, the slot spent, what it
	// did to each target and the concentration it set or ended; a feature action:
	// the resource a use of which was spent. For a damage roll of a cast, Settled
	// is every pending damage the one roll settled.
	CastID      string      `json:"cast_id,omitempty"`
	Slot        *slotRef    `json:"slot,omitempty"`
	Resource    string      `json:"resource,omitempty"`
	Hits        []castHit   `json:"hits,omitempty"`
	Settled     []damageHit `json:"settled,omitempty"`
	Heal        bool        `json:"heal,omitempty"`
	Concentrate bool        `json:"concentrate,omitempty"`
	// ConcBefore is the spell the caster concentrated on before (empty: none),
	// and ConcEnded the one the cast stopped (the same, when it replaced it).
	ConcBefore string `json:"conc_before,omitempty"`
	ConcEnded  string `json:"conc_ended,omitempty"`
	// Escudo: the +5 the target had before, the reaction and what it did.
	ACBonusBefore int32 `json:"ac_bonus_before,omitempty"`
	Stopped       bool  `json:"stopped,omitempty"`
	// Extra Attack and the opportunity attack.
	AttacksBefore int32 `json:"attacks_before,omitempty"`
	AsReaction    bool  `json:"as_reaction,omitempty"`

	// A death save, or damage at 0 hit points: the counts before and after,
	// the outcome and the failures the damage caused.
	Death         *deathState `json:"death,omitempty"`
	DeathBefore   *deathState `json:"death_before,omitempty"`
	DeathOutcome  string      `json:"death_outcome,omitempty"`
	FailuresAdded int32       `json:"failures_added,omitempty"`
	// The master applied another amount than the rolled one (Rolled), and the
	// damage threatened a concentration (ConcentrationDC).
	Overridden      bool  `json:"overridden,omitempty"`
	Rolled          int32 `json:"rolled,omitempty"`
	ConcentrationDC int32 `json:"concentration_dc,omitempty"`
	// The conditions the master set (Conditions, with CondSet true even when
	// empty) and the ones there were.
	CondSet    bool     `json:"cond_set,omitempty"`
	Conditions []string `json:"conditions,omitempty"`
	CondBefore []string `json:"cond_before,omitempty"`

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
