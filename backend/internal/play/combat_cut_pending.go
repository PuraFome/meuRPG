package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// The line of a Cutting Words whose held roll asked its roller. A reaction window holds an attack or a spell attack, the windows are
// answered, and the replay rolls the d20; when the roller is asked about it (a halfling's Lucky, a Bardic Inspiration die) the roll
// is kept and written only when the answer comes, in a transaction of its own. The bard's answer is written first, so it cannot say
// what the roll became. Its event is marked (Asked, with the hold's group and what the group took off), the event the answer writes
// names the group (CutGroup), and the log joins them when it reads: until then the line says nothing of the roll, and once the roll
// is written it says what a roll that asked nothing says (completeAnswers). No event is ever changed.

// markAsked, answersOf and completeAnswers read c.answered as the answers of this change alone: the window answered last, whose answer
// released the replay. An answer written in an earlier change of the same group is not in it (#1061).

// markAsked marks the answers of the windows of a held attack or spell attack whose replay asked its roller, so the log completes them
// when the roll is written. A held check asks the same way and is not marked: its line is not completed this way, and keeps what the answer wrote when the
// replay asked, before the roll had a total.
func markAsked(c *combatTx, h playdb.ReactionHold, windows []playdb.ReactionWindow, st *replayState) {
	if h.Kind != "attack" && h.Kind != "cast" {
		return
	}
	for _, ref := range answersOf(c, windows) {
		if ref.ev.Kind == string(reaction.CuttingWords) && !ref.ev.Ineffect { // an answer that did nothing says so whatever the roll becomes
			ref.ev.Asked, ref.ev.Group, ref.ev.Reduction = true, h.GroupID, st.reduction
		}
	}
}

// answersOf are the answers of this change that belong to the windows of a held action.
func answersOf(c *combatTx, windows []playdb.ReactionWindow) []answeredRef {
	return slices.DeleteFunc(slices.Clone(c.answered), func(ref answeredRef) bool {
		return !slices.ContainsFunc(windows, func(w playdb.ReactionWindow) bool { return w.ID == ref.window })
	})
}

// cutGroupOfAnswer is the group of the reaction hold behind the roll that the answer being run settles, "" when the roll was no
// replay of a held action. The hold of the roll is marked as a window's and keeps the key of the request the window held; the
// event that held that request, written under the key, names the group.
func (s *Service) cutGroupOfAnswer(ctx context.Context, c *combatTx) (string, error) {
	reply, answering := heldReplyOf(ctx)
	if !answering {
		return "", nil
	}
	h, err := newHeldStores(c.q).byIDOf(ctx, holdRef{Campaign: c.session.CampaignID, ID: reply.holdID})
	if err != nil {
		return "", fmt.Errorf("find the held roll: %w", err)
	}
	if !h.Window {
		return "", nil
	}
	row, err := c.q.GetSessionEventByIdempotencyKey(ctx, playdb.GetSessionEventByIdempotencyKeyParams{GameSessionID: c.session.ID, IdempotencyKey: &h.Key})
	if errors.Is(err, pgx.ErrNoRows) { // no event holds the roll: nothing to complete, and the answer goes on
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find the event that held the roll: %w", err)
	}
	held, err := readEvent(row.Payload)
	if err != nil {
		return "", fmt.Errorf("read the event that held the roll: %w", err)
	}
	if held.Reaction == nil || !held.Reaction.Hold {
		return "", nil
	}
	return held.Reaction.Group, nil
}

// cutJoin is the roll that settled the question of a Cutting Words line, and the answer as it reads once the roll is written.
type cutJoin struct {
	by   actionEvent
	done *reactionEvent
}

// registerCut remembers an answer that waits for the roll of its group.
func (b *logBuilder) registerCut(entry *logEntry) {
	re := entry.ev.Reaction
	if re.Asked && re.Group != "" && re.Kind == string(reaction.CuttingWords) {
		b.byCut[re.Group] = append(b.byCut[re.Group], entry)
	}
}

// joinCut lands the event of a roll an answer settled on the lines of the Cutting Words of its group: they host it, so that the line's
// number moves with it, and read what the roll became. An undone roll never gets here, and its lines wait again.
func (b *logBuilder) joinCut(id, eventKind string, final actionEvent) {
	kind := "attack"
	if eventKind == eventSpellCast {
		kind = "cast"
	}
	for _, host := range b.byCut[final.CutGroup] {
		if done, ok := host.ev.Reaction.completedBy(kind, final); ok {
			host.hosts = append(host.hosts, id)
			host.cut = &cutJoin{by: final, done: &done}
		}
	}
}

// cutNumbers are the numbers a Cutting Words answer reads once the roll it cut is written.
type cutNumbers struct {
	outcome       string
	ac            int32
	before, after int32
}

// cutNumbersOf is the one rule of those numbers, for the hold kind of the roll ("attack" or "cast"): a weapon attack's outcome, armor
// class and total, or the one roll of a spell attack's; after is the total, before the total with the die back on it (reduction is what
// the group took off). ok is false when the rule says nothing: a cast with no single hit (a spell that rolled no attack, or several
// rays) completes nothing, and any other kind is no attack roll.
func cutNumbersOf(kind string, final actionEvent, reduction int32) (cutNumbers, bool) {
	switch kind {
	case "attack":
		return cutNumbers{final.Outcome, final.TargetAC, final.Total + reduction, final.Total}, true
	case "cast":
		if len(final.Hits) != 1 {
			return cutNumbers{}, false
		}
		h := final.Hits[0]
		return cutNumbers{h.Outcome, h.TargetAC, h.Total + reduction, h.Total}, true
	}
	return cutNumbers{}, false
}

// fill writes the numbers of a roll on the answer.
func (r *reactionEvent) fill(n cutNumbers) {
	r.Outcome, r.AC, r.Before, r.After = n.outcome, n.ac, n.before, n.after
}

// dropWaiting reads an event that forgets held rolls: the removal of a combatant (forgetHoldsOf drops its holds) and the end of the
// combat. The lines of that roller (every roller at the end) never get a roll, and a completed one is not read as waiting (reactionFor).
func (b *logBuilder) dropWaiting(kind string, ev actionEvent) {
	if kind != eventCombatantRemoved && kind != eventEncounterEnded {
		return
	}
	for _, hosts := range b.byCut {
		for _, host := range hosts {
			if kind == eventEncounterEnded || host.ev.Target == ev.Actor {
				host.neverRolled = true
			}
		}
	}
}

// completedBy is the answer as it reads once the roll that settled its question is known.
// The numbers are cutNumbersOf's, for the kind the roll's own event says (a spell_cast is a cast).
// Only a replay with one ray asks, because holdSpellAttack puts rayCombatants (one per ray) through spellMayAsk, which wants exactly one.
// So an Asked answer of a cast never has more than one hit.
func (r reactionEvent) completedBy(kind string, final actionEvent) (reactionEvent, bool) {
	n, ok := cutNumbersOf(kind, final, r.Reduction)
	if !ok {
		return r, false
	}
	r.fill(n)
	r.Asked = false
	return r, true
}

// reactionFor is the answer as the viewer reads it: the event's own while the roll it waits for is not written or the viewer does not hear
// of that event, else the completed one. The viewer who hears nothing of the roll (a hidden roller's, a fogged one's) reads
// no more than the others did before it.
func (e *logEntry) reactionFor(v combatViewer) *reactionEvent {
	if e.cut != nil && e.cut.by.heardBy(v) {
		return e.cut.done
	}
	if e.cut == nil && e.neverRolled {
		gone := *e.ev.Reaction
		gone.NeverRolled = true
		return &gone
	}
	return e.ev.Reaction
}
