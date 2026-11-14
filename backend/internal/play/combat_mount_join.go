package play

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The initiative of a pair (SRD 5.1, "Mounted Combat"). A controlled mount acts as its rider directs, so while it carries one it takes the
// rider's initiative total, d20 and bonus and sits right after the rider in the order: groupRuns then makes the two one joint group, and
// the turn passes when both parts end. What the mount had of its own waits in mount_prior_init until the rider gets off (or anything
// else takes the rider off it). An independent mount keeps its own place and its own turn.
//
// The pair is worked out from the rows: syncMounts makes the initiative columns of every mount follow who rides it, and orderCombatants
// seats a controlled mount right after its rider whatever the totals say, so a reorder (a new roll, a reinforcement) never splits them.

// priorInit is the place a controlled mount keeps while it rides in its rider's (combatants.mount_prior_init): its own total and d20
// (null while it had none), whether the master had ordered its tie, and, for the order to come back as it was, its bonus and who
// stood right before it in the order.
type priorInit struct {
	Initiative *int32 `json:"initiative"`
	Face       *int32 `json:"face"`
	Tie        bool   `json:"tie"`
	Bonus      int32  `json:"bonus"`
	After      string `json:"after,omitempty"`
}

// joinedPair says whether the two are a rider and the controlled mount that takes its initiative.
func joinedPair(a, b playdb.Combatant) bool {
	return (a.MountedOnID != nil && *a.MountedOnID == b.ID && b.MountKind == mountKindControlled) ||
		(b.MountedOnID != nil && *b.MountedOnID == a.ID && a.MountKind == mountKindControlled)
}

// mountsAfterRiders puts every controlled mount that carries a rider right after its rider, in the order cs is in: a mount follows its
// rider wherever the totals would put it.
func mountsAfterRiders(cs []playdb.Combatant) []playdb.Combatant {
	behind := map[string]playdb.Combatant{} // rider ID to its controlled mount
	follows := map[string]bool{}            // the mounts that are not placed on their own
	for _, rider := range cs {
		if rider.MountedOnID == nil {
			continue
		}
		for _, m := range cs {
			if m.ID == *rider.MountedOnID && m.MountKind == mountKindControlled {
				behind[rider.ID], follows[m.ID] = m, true
			}
		}
	}
	if len(behind) == 0 {
		return cs
	}
	out := make([]playdb.Combatant, 0, len(cs))
	for _, c := range cs {
		if follows[c.ID] {
			continue
		}
		out = append(out, c)
		if m, ok := behind[c.ID]; ok {
			out = append(out, m)
		}
	}
	return out
}

// riderJoined is the rider a controlled mount takes the initiative of: the one that sits on it and has rolled; nil for any other mount.
func riderJoined(cs []playdb.Combatant, mount playdb.Combatant) *playdb.Combatant {
	if mount.MountKind != mountKindControlled {
		return nil
	}
	if rider := riderOf(cs, mount.ID); rider != nil && rider.Initiative != nil {
		return rider
	}
	return nil
}

// sameInitiative says the mount has the rider's total, d20, bonus and tie.
func sameInitiative(m, rider playdb.Combatant) bool {
	return optInt(m.Initiative) == optInt(rider.Initiative) && optInt(m.InitiativeFace) == optInt(rider.InitiativeFace) &&
		m.InitiativeBonus == rider.InitiativeBonus && m.TieOrdered == rider.TieOrdered
}

// optInt is the value of an optional number, -1 for none (no total or d20 is negative).
func optInt(n *int32) int32 {
	if n == nil {
		return -1
	}
	return *n
}

// writeJoin writes the initiative columns of the mount and the prior initiative it keeps (nil: none), and answers the mount as it stands.
func writeJoin(ctx context.Context, q *playdb.Queries, m playdb.Combatant, total, face *int32, bonus int32, tie bool, prior []byte) (playdb.Combatant, error) {
	if err := q.SetCombatantJoin(ctx, playdb.SetCombatantJoinParams{
		ID: m.ID, Initiative: total, InitiativeFace: face, InitiativeBonus: bonus, TieOrdered: tie, MountPriorInit: prior,
	}); err != nil {
		return m, fmt.Errorf("write the initiative of the mount: %w", err)
	}
	m.Initiative, m.InitiativeFace, m.InitiativeBonus, m.TieOrdered, m.MountPriorInit = total, face, bonus, tie, prior
	return m, nil
}

// joinRider makes the mount take the rider's initiative and keep its own.
func joinRider(ctx context.Context, q *playdb.Queries, m, rider playdb.Combatant, after string) (playdb.Combatant, error) {
	prior, err := json.Marshal(priorInit{Initiative: m.Initiative, Face: m.InitiativeFace, Tie: m.TieOrdered, Bonus: m.InitiativeBonus, After: after})
	if err != nil {
		return m, fmt.Errorf("keep the prior initiative of the mount: %w", err)
	}
	return writeJoin(ctx, q, m, rider.Initiative, rider.InitiativeFace, rider.InitiativeBonus, rider.TieOrdered, prior)
}

// releaseJoin gives the mount its own initiative back, and answers who it stood after in the order ("" for the first place).
func releaseJoin(ctx context.Context, q *playdb.Queries, m playdb.Combatant) (playdb.Combatant, string, error) {
	var p priorInit
	if err := json.Unmarshal(m.MountPriorInit, &p); err != nil {
		return m, "", fmt.Errorf("read the prior initiative of the mount: %w", err)
	}
	m, err := writeJoin(ctx, q, m, p.Initiative, p.Face, p.Bonus, p.Tie, nil)
	return m, p.After, err
}

// syncMounts makes the initiative columns of every mount follow who rides it: a controlled mount that carries a rider that has rolled
// takes its initiative, and a mount that carries none (or is not controlled any more) has its own back. It saves the order when
// anything changed and answers the combatants as they stand, in order. cs are the combatants as stored.
func syncMounts(ctx context.Context, c *combatTx, cs []playdb.Combatant) ([]playdb.Combatant, error) {
	work := slices.Clone(cs)
	after := map[int]string{} // the mounts that got their own initiative back, by place in cs, and who they stood after
	changed := false
	for i, m := range work {
		rider, held := riderJoined(work, m), len(m.MountPriorInit) > 0
		var err error
		switch {
		case rider != nil && !held:
			work[i], err = joinRider(ctx, c.q, m, *rider, predecessorOf(cs, i))
		case rider != nil && !sameInitiative(m, *rider):
			work[i], err = writeJoin(ctx, c.q, m, rider.Initiative, rider.InitiativeFace, rider.InitiativeBonus, rider.TieOrdered, m.MountPriorInit)
		case rider == nil && held:
			work[i], after[i], err = releaseJoin(ctx, c.q, m)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		changed = true
	}
	if !changed {
		return cs, nil
	}
	return saveOrder(ctx, c.q, cs, orderCombatants(placedAfter(work, after)))
}

// predecessorOf is the ID of the combatant right before the one at place i of cs, "" for the first.
func predecessorOf(cs []playdb.Combatant, i int) string {
	if i == 0 {
		return ""
	}
	return cs[i-1].ID
}

// placedAfter is cs with the order keys the sort reads: every place doubled, and a mount that got its own initiative back right after
// the combatant it stood after (or first), so that among those it ties with it comes back to where it was.
func placedAfter(cs []playdb.Combatant, after map[int]string) []playdb.Combatant {
	out := slices.Clone(cs)
	for i := range out {
		out[i].OrderIndex *= 2
	}
	for i, id := range after {
		out[i].OrderIndex = -1
		if at := slices.IndexFunc(cs, func(c playdb.Combatant) bool { return c.ID == id }); id != "" && at >= 0 {
			out[i].OrderIndex = cs[at].OrderIndex*2 + 1
		}
	}
	return out
}

// settleMounts is syncMounts for the combatants the change left in the combat.
func settleMounts(ctx context.Context, c *combatTx) error {
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return err
	}
	_, err = syncMounts(ctx, c, cs)
	return err
}

// turnSnap is what the start of a turn resets on a combatant (ResetCombatantTurn) and its turn state, as the undo of a mount that took
// a part of its rider's turn puts it back.
type turnSnap struct {
	CreatureSpent         json.RawMessage `json:"creature_spent,omitempty"`
	MovementUsedFt        int32           `json:"movement_used_ft,omitempty"`
	MovementUsedDft       int32           `json:"movement_used_dft,omitempty"`
	LastMoveDft           int32           `json:"last_move_dft,omitempty"`
	Dashed                bool            `json:"dashed,omitempty"`
	Disengaged            bool            `json:"disengaged,omitempty"`
	JumpDoubled           bool            `json:"jump_doubled,omitempty"`
	MountMoveUsed         bool            `json:"mount_move_used,omitempty"`
	ActionSurged          bool            `json:"action_surged,omitempty"`
	SpellCast             bool            `json:"spell_cast,omitempty"`
	BonusSpellCast        bool            `json:"bonus_spell_cast,omitempty"`
	ActionUsed            bool            `json:"action_used,omitempty"`
	BonusActionUsed       bool            `json:"bonus_action_used,omitempty"`
	ObjectInteractionUsed bool            `json:"object_interaction_used,omitempty"`
	ReactionUsed          bool            `json:"reaction_used,omitempty"`
	ReactionKind          *string         `json:"reaction_kind,omitempty"`
	ReactionRound         *int32          `json:"reaction_round,omitempty"`
	AttacksMade           int32           `json:"attacks_made,omitempty"`
	ActionAttackKey       *string         `json:"action_attack_key,omitempty"`
	BonusAttacksLeft      int32           `json:"bonus_attacks_left,omitempty"`
	AcBonus               int32           `json:"ac_bonus,omitempty"`
	ExtraActionUsed       bool            `json:"extra_action_used,omitempty"`
	DeathSaveRolled       bool            `json:"death_save_rolled,omitempty"`
	TurnState             string          `json:"turn_state"`
}

func turnSnapOf(c playdb.Combatant) *turnSnap {
	return &turnSnap{
		CreatureSpent: c.CreatureSpent, MovementUsedFt: c.MovementUsedFt, MovementUsedDft: c.MovementUsedDft, LastMoveDft: c.LastMoveDft,
		Dashed: c.Dashed, Disengaged: c.Disengaged, JumpDoubled: c.JumpDoubled, MountMoveUsed: c.MountMoveUsed, ActionSurged: c.ActionSurged,
		SpellCast: c.SpellCast, BonusSpellCast: c.BonusSpellCast, ActionUsed: c.ActionUsed, BonusActionUsed: c.BonusActionUsed,
		ObjectInteractionUsed: c.ObjectInteractionUsed, ReactionUsed: c.ReactionUsed, ReactionKind: c.ReactionKind, ReactionRound: c.ReactionRound,
		AttacksMade: c.AttacksMade, ActionAttackKey: c.ActionAttackKey, BonusAttacksLeft: c.BonusAttacksLeft, AcBonus: c.AcBonus,
		ExtraActionUsed: c.ExtraActionUsed, DeathSaveRolled: c.DeathSaveRolled, TurnState: c.TurnState,
	}
}

// orderSlot is the place of a combatant in the order, as the undo of a ride puts it back.
type orderSlot struct {
	ID    string `json:"id"`
	Index int32  `json:"index"`
	Tie   bool   `json:"tie,omitempty"`
}

// joinUndo is what a mount or a dismount that changed the initiative of the mount keeps for its undo: the mount as it was (its initiative,
// its prior initiative and the turn it had) and the place of everyone in the order.
type joinUndo struct {
	Mount      string          `json:"mount"`
	Initiative *int32          `json:"initiative,omitempty"`
	Face       *int32          `json:"face,omitempty"`
	Bonus      int32           `json:"bonus,omitempty"`
	Tie        bool            `json:"tie,omitempty"`
	Prior      json.RawMessage `json:"prior,omitempty"`
	Turn       *turnSnap       `json:"turn,omitempty"`
	Order      []orderSlot     `json:"order,omitempty"`
}

func joinUndoOf(cs []playdb.Combatant, mount playdb.Combatant) *joinUndo {
	j := &joinUndo{
		Mount: mount.ID, Initiative: mount.Initiative, Face: mount.InitiativeFace, Bonus: mount.InitiativeBonus, Tie: mount.TieOrdered,
		Prior: mount.MountPriorInit, Turn: turnSnapOf(mount),
	}
	for _, c := range cs {
		j.Order = append(j.Order, orderSlot{ID: c.ID, Index: c.OrderIndex, Tie: c.TieOrdered})
	}
	return j
}

// mountPassed says the mount's own turn came before the rider's group in this round: the rider's turn is running, and the mount sits in an
// earlier group of the order. cs are the combatants before the ride.
func mountPassed(cs []playdb.Combatant, rider, mount playdb.Combatant) bool {
	runs := groupRuns(cs)
	at := func(id string) int {
		return slices.IndexFunc(runs, func(g []playdb.Combatant) bool {
			return slices.ContainsFunc(g, func(c playdb.Combatant) bool { return c.ID == id })
		})
	}
	return at(mount.ID) < at(rider.ID)
}

// rideInitiative settles the initiative and the turn of the mount a ride changed: the rider has just mounted it (or got off). It answers
// what the undo needs, or nil when the mount's initiative did not change (an independent mount, a rider that has not rolled). cs are the
// combatants before the ride, in order.
func rideInitiative(ctx context.Context, c *combatTx, cs []playdb.Combatant, rider, mount playdb.Combatant) (*joinUndo, error) {
	if mount.MountKind != mountKindControlled {
		return nil, nil
	}
	now, err := listCombatants(ctx, c)
	if err != nil {
		return nil, err
	}
	if now, err = syncMounts(ctx, c, now); err != nil {
		return nil, err
	}
	after, _ := combatantByID(now, mount.ID)
	was, is := len(mount.MountPriorInit) > 0, len(after.MountPriorInit) > 0
	switch {
	case is && !was:
		return joinUndoOf(cs, mount), joinTurn(ctx, c, rider, after, mountPassed(cs, rider, mount))
	case was && !is:
		return joinUndoOf(cs, mount), leaveGroup(ctx, c, now, rider, after)
	}
	return nil, nil
}

// joinTurn gives the mount that joined its rider's pair the rider's part of the turn that runs. Its own turn came before this one (passed):
// the economy stays as it was, spent, and it takes the part with what it has left (no second turn). Its own turn is still to come: it
// starts with the economy a turn starts with, and it does not act again this round at its old place.
func joinTurn(ctx context.Context, c *combatTx, rider, mount playdb.Combatant, passed bool) error {
	if c.enc.Status != statusActive || rider.TurnState == turnIdle || mount.TurnState != turnIdle || mount.Defeated {
		return nil
	}
	if passed {
		if err := c.q.SetCombatantTurnState(ctx, playdb.SetCombatantTurnStateParams{ID: mount.ID, TurnState: turnActing}); err != nil {
			return fmt.Errorf("give the mount its part of the turn: %w", err)
		}
		return nil
	}
	if err := c.q.ResetCombatantTurn(ctx, mount.ID); err != nil {
		return fmt.Errorf("start the mount's part of the turn: %w", err)
	}
	return nil
}

// leaveGroup takes the mount that got off out of the rider's turn, unless it is in the rider's group by its own total.
func leaveGroup(ctx context.Context, c *combatTx, now []playdb.Combatant, rider, mount playdb.Combatant) error {
	if mount.TurnState == turnIdle {
		return nil
	}
	for _, g := range groupRuns(now) {
		in := func(id string) bool {
			return slices.ContainsFunc(g, func(o playdb.Combatant) bool { return o.ID == id })
		}
		if in(mount.ID) && in(rider.ID) {
			return nil
		}
	}
	if err := c.q.SetCombatantTurnState(ctx, playdb.SetCombatantTurnStateParams{ID: mount.ID, TurnState: turnIdle}); err != nil {
		return fmt.Errorf("take the mount out of the turn: %w", err)
	}
	return nil
}

// joinBack puts the mount and the order back as the ride found them (the undo of a mount or a dismount).
func (u *undoer) joinBack(ctx context.Context, j *joinUndo) error {
	if j == nil {
		return nil
	}
	if _, here := u.find(j.Mount); !here {
		return nil
	}
	q := u.c.q
	if err := q.SetCombatantJoin(ctx, playdb.SetCombatantJoinParams{
		ID: j.Mount, Initiative: j.Initiative, InitiativeFace: j.Face, InitiativeBonus: j.Bonus, TieOrdered: j.Tie, MountPriorInit: j.Prior,
	}); err != nil {
		return fmt.Errorf("put back the initiative of the mount: %w", err)
	}
	if t := j.Turn; t != nil {
		if err := q.RestoreCombatantTurn(ctx, playdb.RestoreCombatantTurnParams{
			ID: j.Mount, CreatureSpent: t.CreatureSpent, MovementUsedFt: t.MovementUsedFt, MovementUsedDft: t.MovementUsedDft, LastMoveDft: t.LastMoveDft,
			Dashed: t.Dashed, Disengaged: t.Disengaged, JumpDoubled: t.JumpDoubled, MountMoveUsed: t.MountMoveUsed, ActionSurged: t.ActionSurged,
			SpellCast: t.SpellCast, BonusSpellCast: t.BonusSpellCast, ActionUsed: t.ActionUsed, BonusActionUsed: t.BonusActionUsed,
			ObjectInteractionUsed: t.ObjectInteractionUsed, ReactionUsed: t.ReactionUsed, ReactionKind: t.ReactionKind, ReactionRound: t.ReactionRound,
			AttacksMade: t.AttacksMade, ActionAttackKey: t.ActionAttackKey, BonusAttacksLeft: t.BonusAttacksLeft, AcBonus: t.AcBonus,
			ExtraActionUsed: t.ExtraActionUsed, DeathSaveRolled: t.DeathSaveRolled, TurnState: t.TurnState,
		}); err != nil {
			return fmt.Errorf("put back the mount's turn: %w", err)
		}
	}
	for _, s := range j.Order {
		if err := q.SetCombatantOrder(ctx, playdb.SetCombatantOrderParams{ID: s.ID, OrderIndex: s.Index, TieOrdered: s.Tie}); err != nil {
			return fmt.Errorf("put back the turn order: %w", err)
		}
	}
	return nil
}
