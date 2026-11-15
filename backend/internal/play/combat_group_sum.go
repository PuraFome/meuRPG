package play

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The master takes an extra out of a group of parts that share a damage type (a typed damage-type sum covered them: the
// roll kept one number for the group). The group's number cannot say what the part was worth, so the damage is taken
// back as the master's undo takes it back, the part is marked out, and the damage waits for the sum of that damage type
// alone (pendingAwaitingGroupSum). RollDamage takes that sum, keeps what the other damage types rolled, and the damage
// lands as for a first roll.
//
// What depends on the damage follows the undo's rule (combat_undo.go), the same code runs it (takeBackAction):
//   - a damage applied to an NPC or a creature is taken back only while its roll is the last action the master could
//     undo in the session (else NOTHING_TO_UNDO), and while its target's hit points, temporary hit points, Wild Shape
//     pool and defeat are as the roll left them (else UNDO_TARGET_CHANGED);
//   - a damage that is only rolled (a player's character waits for the master) has no hit points to put back: its roll
//     is found among the latest events wherever it stands, what the roll spent and opened is put back, but an offer of
//     the damage that was answered already leaves nothing to undo (NOTHING_TO_UNDO).
//
// The take-out is no undoable action: it ends the undo chain, as a correction of the hit points does.

// takeOutOfGroup takes the part out of the group the damage was rolled with, inside the removal's transaction.
func (s *Service) takeOutOfGroup(ctx context.Context, c *combatTx, run *undoRun, r partRemoval, p playdb.PendingDamage, attacker, target playdb.Combatant, parts []partRecord, rolls []partRoll) (actionEvent, error) {
	pi := slices.IndexFunc(parts, func(x partRecord) bool { return x.Key == r.partKey })
	ri := slices.IndexFunc(rolls, func(x partRoll) bool { return x.Key == r.partKey })
	if p.CastID != nil || !mayComeOut(parts, rolls, r.partKey) {
		return actionEvent{}, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_PART_NOT_REMOVABLE, "that part cannot be taken out")
	}
	awaited := rolls[ri].Group
	recent, err := c.q.ListRecentSessionEvents(ctx, playdb.ListRecentSessionEventsParams{GameSessionID: c.session.ID, Limit: recentEvents})
	if err != nil {
		return actionEvent{}, fmt.Errorf("read the latest events: %w", err)
	}
	row, rolled, err := s.rollToTakeBack(ctx, c, recent, p)
	if err != nil {
		return actionEvent{}, err
	}
	*run = undoRun{}
	if err := s.takeBackAction(ctx, c, run, recent, row, onlyWhatComesOut(rolled, parts[pi])); err != nil {
		return actionEvent{}, err
	}
	label := parts[pi].Label
	parts[pi].Removed = true
	if _, err := c.q.SetPendingDamageGroupSum(ctx, playdb.SetPendingDamageGroupSumParams{
		ID: p.ID, AwaitedDamageTypeKey: awaited, Parts: mustJSON(parts), PartRolls: mustJSON(keptRolls(rolls, awaited)),
	}); err != nil {
		return actionEvent{}, fmt.Errorf("keep the damage that awaits the sum: %w", err)
	}
	return actionEvent{
		Round: c.enc.Round, Secret: secretOf(attacker, target), Fogged: rolled.Fogged, SeenBy: rolled.SeenBy, Actor: attacker.ID, Target: target.ID,
		Pending: p.ID, Key: r.partKey, PartLabel: label, PartHidden: parts[pi].Hide || rolls[ri].Hide, AwaitsSum: awaited, DamageType: p.DamageType,
		Undone: row.ID, UndoneKind: row.Kind, UndoneAlso: run.alsoUndone,
	}, nil
}

// onlyWhatComesOut is the roll's event as the take-out takes it back: what the roll spent stays spent, and the extra that
// comes out gives back its once-per-turn use alone (Sneak Attack, Colossus Slayer, a table's once-per-turn extra), as the removal of
// an extra from an ordinary damage does. The slot of a Divine Smite and the uses of a maneuver were paid once, when the first roll
// declared them, and the roll of the new sum spends nothing (settleParts), so the part that stays never pays twice. The use of a
// reroll rule stays the undo's: it is the roll's, and a new roll spends its own.
func onlyWhatComesOut(ev actionEvent, removed partRecord) actionEvent {
	ev.Slot, ev.ManeuverSpent = nil, nil
	if ev.FeatureBefore == nil {
		return ev
	}
	kept := map[string]*featureEntry{}
	if feature := onceFeatureOf(removed); feature != "" {
		if entry, ok := ev.FeatureBefore[feature]; ok {
			kept[feature] = entry
		}
	}
	ev.FeatureBefore = kept
	return ev
}

// onceFeatureOf is the feature whose once-per-turn use the part marked when it counted: Sneak Attack's, Colossus Slayer's, or
// the table's own for a part that is once per turn (settleParts); none for a part that marks nothing.
func onceFeatureOf(part partRecord) string {
	switch {
	case part.Key == combat.ExtraSneakAttack:
		return featureSneakAttack
	case part.Key == combat.ExtraColossusSlayer:
		return featureColossusSlayer
	case part.Once:
		return part.Key
	}
	return ""
}

// mayComeOut says the part is an extra that counts and is none the app keeps (the weapon, an automatic line, a smite whose slot is
// spent): the rules of takeOut, apart from the group.
func mayComeOut(parts []partRecord, rolls []partRoll, key string) bool {
	pi := slices.IndexFunc(parts, func(x partRecord) bool { return x.Key == key })
	ri := slices.IndexFunc(rolls, func(x partRoll) bool { return x.Key == key })
	return pi >= 0 && ri >= 0 && parts[pi].Kind == partExtra && key != combat.ExtraDivineSmite && key != combat.ExtraDivineSmiteExtra && rolls[ri].Counted
}

// keptRolls are the rolls of the damage types other than the awaited one, as they were rolled.
func keptRolls(rolls []partRoll, awaited string) []partRoll {
	kept := []partRoll{}
	for _, r := range rolls {
		if r.DamageType != awaited {
			kept = append(kept, r)
		}
	}
	return kept
}

// rollToTakeBack finds the event of the damage's roll among the session's latest events and applies the limit of the undo to
// it: the refusals are the undo's, and nothing is written.
func (s *Service) rollToTakeBack(ctx context.Context, c *combatTx, recent []playdb.ListRecentSessionEventsRow, p playdb.PendingDamage) (playdb.ListRecentSessionEventsRow, actionEvent, error) {
	nothing := errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOTHING_TO_UNDO, "the damage can no longer be taken back: something else was done after it")
	row, ev, ok := rollEventOf(recent, p.ID)
	if !ok {
		return row, ev, nothing
	}
	if p.Status == pendingApplied {
		if !isLastAction(recent, c.enc.ID, row) {
			return row, ev, nothing
		}
		return row, ev, nil
	}
	riders, err := c.q.ListHitRiders(ctx, c.enc.ID)
	if err != nil {
		return row, ev, fmt.Errorf("list the riders: %w", err)
	}
	if slices.ContainsFunc(riders, func(h playdb.HitRider) bool { return h.Used && conv.OrZero(h.PendingID) == p.ID }) {
		return row, ev, nothing
	}
	return row, ev, nil
}

// isLastAction says the event is the last action the master could undo in the session: the limit of the take-out of a damage applied
// to an NPC or a creature (rollToTakeBack), and the condition of the listing that offers it (takeOutListing).
func isLastAction(recent []playdb.ListRecentSessionEventsRow, encounterID string, row playdb.ListRecentSessionEventsRow) bool {
	last, ok := lastAction(recent, encounterID)
	return ok && last.ID == row.ID
}

// takeOutListing is the damage the attacker's last action applied to an NPC or a creature that a take-out could still undo: the
// master's turn options list it beside the damage that waits, with the status APPLIED, so his screen offers "Tirar" on it. It
// holds what takeOutOfGroup and removePartWrite check before anything is written, from the same functions: the damage is the
// attacker's, applied, not a cast's, on a target that holds hit points, its roll is the last action (rollToTakeBack) and a part
// of it can come out (mayComeOut). The checks that need the transaction (the target's hit points as the roll left them) stay
// with the take-out, which refuses with UNDO_TARGET_CHANGED.
func (s *Service) takeOutListing(ctx context.Context, d *encounterData, attacker playdb.Combatant) (playdb.PendingDamage, bool, error) {
	recent, err := s.queries.ListRecentSessionEvents(ctx, playdb.ListRecentSessionEventsParams{GameSessionID: d.enc.GameSessionID, Limit: recentEvents})
	if err != nil {
		return playdb.PendingDamage{}, false, fmt.Errorf("read the latest events: %w", err)
	}
	row, ok := lastAction(recent, d.enc.ID)
	pending, byAttacker := damageRolledBy(row, attacker)
	if !ok || !byAttacker {
		return playdb.PendingDamage{}, false, nil
	}
	p, err := s.queries.GetPendingDamage(ctx, playdb.GetPendingDamageParams{EncounterID: d.enc.ID, ID: pending})
	if errors.Is(err, pgx.ErrNoRows) {
		return p, false, nil
	}
	if err != nil {
		return p, false, fmt.Errorf("find the pending damage: %w", err)
	}
	rolled, _, found := rollEventOf(recent, p.ID)
	return p, found && rolled.ID == row.ID && takeOutMayApply(d, p, attacker), nil
}

// damageRolledBy is the pending damage an event rolled, when the event is a damage roll of the attacker's.
func damageRolledBy(row playdb.ListRecentSessionEventsRow, attacker playdb.Combatant) (string, bool) {
	ev, err := readEvent(row.Payload)
	if err != nil || row.Kind != eventDamageRolled || ev.Pending == "" || ev.Actor != attacker.ID {
		return "", false
	}
	return ev.Pending, true
}

// takeOutMayApply says the pending damage is one a take-out could take back: the attacker's applied damage, not a cast's, on a
// target that holds hit points, with a part that can come out.
func takeOutMayApply(d *encounterData, p playdb.PendingDamage, attacker playdb.Combatant) bool {
	target, err := findCombatant(d.cs, p.TargetID, combatViewer{master: true})
	if err != nil || p.Status != pendingApplied || p.CastID != nil || conv.OrZero(p.AttackerID) != attacker.ID || !holdsHP(target) {
		return false
	}
	parts, rolls := readParts(p.Parts), readPartRolls(p.PartRolls)
	return slices.ContainsFunc(parts, func(x partRecord) bool { return mayComeOut(parts, rolls, x.Key) })
}

// rollEventOf finds the event of a pending damage's roll that no undo took back, newest first.
func rollEventOf(recent []playdb.ListRecentSessionEventsRow, pending string) (playdb.ListRecentSessionEventsRow, actionEvent, bool) {
	undone := map[string]bool{}
	for _, e := range recent {
		if e.Kind == eventActionUndone {
			markUndone(undone, e.Payload)
			continue
		}
		if e.Kind != eventDamageRolled || undone[e.ID] {
			continue
		}
		ev, err := readEvent(e.Payload)
		if err != nil || ev.Pending != pending || (ev.Reaction != nil && ev.Reaction.Hold) {
			continue
		}
		return e, ev, true
	}
	return playdb.ListRecentSessionEventsRow{}, actionEvent{}, false
}

// groupSumTyped is the sums typed for the roll of a group's sum: typed_sum is the sum of the awaited damage type's dice, and
// typed_parts may name that damage type or its parts, and nothing else (what the other damage types rolled stays).
func groupSumTyped(parts []partRecord, awaited string, in rollInput, typed typedSums) (typedSums, error) {
	out := typedSums{byPart: maps.Clone(typed.byPart), byType: maps.Clone(typed.byType)}
	if out.byPart == nil {
		out.byPart = map[string]int{}
	}
	if out.byType == nil {
		out.byType = map[string]int{}
	}
	for typ := range out.byType {
		if typ != awaited {
			return typedSums{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("typed_parts names %s, but the sum awaited is of %s", typ, awaited))
		}
	}
	for key := range out.byPart {
		if i := slices.IndexFunc(parts, func(x partRecord) bool { return x.Key == key }); i >= 0 && parts[i].DamageType != awaited {
			return typedSums{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("typed_parts names %s, but the sum awaited is of %s", key, awaited))
		}
	}
	if _, both := out.byType[awaited]; !in.inApp && in.typed != 0 && !both {
		out.byType[awaited] = in.typed // typed_sum is the awaited type's sum
	}
	return out, nil
}
