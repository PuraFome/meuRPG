package play

import (
	"context"
	"fmt"
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The end of a readied spell before its release, and the undo of a readied spell. A held spell and its concentration go
// together: settleReadiedSpells, in the settle step of every change, breaks the held spell whose concentration ended
// (whatever ended it: a failed concentration save, 0 hit points, an incapacitating condition, a new concentration
// spell, the master or the player ending it), and ends the concentration of a readier whose spell stopped waiting
// (lost at its next turn, defeated or dismissed, given up, let pass). The slot stays spent: nothing comes back.

// settleReadied is the readied actions' part of settleChange. It reads the waiting rows and the combatants once: a readier
// that is defeated or dismissed loses its row (loseReadiedOfDefeated), then each held spell and its concentration are kept
// together (settleReadiedSpellsOf). With no row waiting and nobody holding a spell there is nothing to do. It says
// whether a row stopped waiting: its READIED window, if open, must then close (readiedGone).
func settleReadied(ctx context.Context, c *combatTx) (bool, error) {
	rows, all, err := readiedState(ctx, c)
	if err != nil || (len(rows) == 0 && !slices.ContainsFunc(all, holdsReadiedSpell)) {
		return false, err
	}
	before := len(rows)
	if rows, err = loseReadiedOfDefeated(ctx, c, rows, all); err != nil {
		return false, err
	}
	broke, err := settleReadiedSpellsOf(ctx, c, rows, all)
	return broke || len(rows) < before, err
}

// settleReadiedSpells keeps each held spell and its concentration together, outside settleChange (a correction), and
// says whether a held spell broke.
func settleReadiedSpells(ctx context.Context, c *combatTx) (bool, error) {
	rows, all, err := readiedState(ctx, c)
	if err != nil {
		return false, err
	}
	return settleReadiedSpellsOf(ctx, c, rows, all)
}

// readiedState is the combat's waiting rows and all of its combatants, the dismissed too; nothing for no combat.
func readiedState(ctx context.Context, c *combatTx) ([]playdb.ReadiedAction, []playdb.Combatant, error) {
	if c.enc.ID == "" {
		return nil, nil, nil
	}
	rows, err := c.q.ListWaitingReadiedActions(ctx, c.enc.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("list the readied actions: %w", err)
	}
	all, err := c.q.ListCombatantsWithDismissed(ctx, c.enc.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("list the combatants: %w", err)
	}
	return rows, all, nil
}

// holdsReadiedSpell says the combatant concentrates on a held spell.
func holdsReadiedSpell(c playdb.Combatant) bool {
	return conv.OrZero(c.ConcentrationSpell) == concentrationReadied
}

// settleReadiedSpellsOf keeps each held spell and its concentration together, for the combatants in the order. A waiting
// spell whose readier no longer concentrates on it is broken, with its line; a readier that concentrates on a held spell
// with none waiting stops. It says whether a held spell broke.
func settleReadiedSpellsOf(ctx context.Context, c *combatTx, rows []playdb.ReadiedAction, all []playdb.Combatant) (bool, error) {
	cs := slices.DeleteFunc(slices.Clone(all), func(x playdb.Combatant) bool { return x.Dismissed })
	holds, broke, err := breakUnheldSpells(ctx, c, rows, cs)
	if err != nil {
		return false, err
	}
	for _, who := range cs {
		if !holdsReadiedSpell(who) || holds[who.ID] {
			continue
		}
		if err := c.q.SetCombatantConcentration(ctx, playdb.SetCombatantConcentrationParams{ID: who.ID}); err != nil {
			return false, fmt.Errorf("end the readied spell's concentration: %w", err)
		}
	}
	return broke, nil
}

// breakUnheldSpells breaks the waiting spells whose readier no longer concentrates on them, and says which readiers
// still hold one and whether one broke.
func breakUnheldSpells(ctx context.Context, c *combatTx, rows []playdb.ReadiedAction, cs []playdb.Combatant) (map[string]bool, bool, error) {
	holds, broke := map[string]bool{}, false
	for _, row := range rows {
		who, ok := combatantByID(cs, row.CombatantID)
		if row.Kind != readiedSpell || !ok {
			continue
		}
		if holdsReadiedSpell(who) {
			holds[who.ID] = true
			continue
		}
		if err := breakReadied(ctx, c, row, who); err != nil {
			return nil, false, err
		}
		broke = true
	}
	return holds, broke, nil
}

// breakReadied ends a held spell whose concentration ended: the row is broken and its readied_lost line written. The
// line is written before the change's own event; the undo of that change makes the row wait again when it gives the
// concentration back (brokenReadiedBack), and otherwise goes past the line (chainReadiedDefeat).
func breakReadied(ctx context.Context, c *combatTx, row playdb.ReadiedAction, who playdb.Combatant) error {
	if err := c.q.SetReadiedActionStatus(ctx, playdb.SetReadiedActionStatusParams{ID: row.ID, Status: readiedBroken}); err != nil {
		return fmt.Errorf("break the readied spell: %w", err)
	}
	ev := readiedRowEvent(row, who, c.enc.Round)
	ev.Readied.Ended = readiedBroken
	return insertEvent(ctx, c, eventReadiedLost, &c.actorUserID, nil, ev)
}

// released says the event is the release of a readied action: its use wrote a readied_fired line before it.
func (r *readiedEvent) released() bool {
	return r != nil && r.Line != ""
}

// holdsSpell says the event's readied action is a spell.
func (r *readiedEvent) holdsSpell() bool {
	return r != nil && r.Kind == readiedSpell
}

// readiedSpellBack is the undo of the Ready of a spell, besides the action: the slot comes back, the concentration is
// the one before (the one Ready ended comes back with its creatures, actionTaken), and the spell cast this turn is
// forgotten.
func (u *undoer) readiedSpellBack(ctx context.Context, ev actionEvent) error {
	who, ok := u.find(ev.Actor)
	if !ok || !ev.Readied.holdsSpell() {
		return nil
	}
	if err := u.castConcentrationBack(ctx, who, ev); err != nil {
		return err
	}
	if err := u.castSpentBack(ctx, who, ev); err != nil {
		return err
	}
	if err := u.c.q.SetCombatantSpellsCast(ctx, playdb.SetCombatantSpellsCastParams{ID: who.ID, SpellCast: ev.SpellCastBefore, BonusSpellCast: ev.BonusSpellBefore}); err != nil {
		return fmt.Errorf("put back the spells cast: %w", err)
	}
	return nil
}

// holdAgain gives the readier of a spell that waits again (the undo of its release or of its pass) the concentration
// that holds it.
func (u *undoer) holdAgain(ctx context.Context, r *readiedEvent, actor string) error {
	if !r.holdsSpell() {
		return nil
	}
	key := concentrationReadied
	if err := u.c.q.SetCombatantConcentration(ctx, playdb.SetCombatantConcentrationParams{ID: actor, ConcentrationSpell: &key}); err != nil {
		return fmt.Errorf("hold the readied spell again: %w", err)
	}
	return nil
}

// brokenReadiedBack: the undo of a change that broke a held spell (its lines are written just before the change's own
// event) makes the row wait again when the undo gave the readier its concentration back, and takes the line with it.
// A row whose readier did not get it back stays broken, with its line.
func (s *Service) brokenReadiedBack(ctx context.Context, c *combatTx, run *undoRun, recent []playdb.ListRecentSessionEventsRow, last playdb.ListRecentSessionEventsRow) error {
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return err
	}
	for _, older := range olderThan(recent, last) {
		r, more, err := brokenLineOf(older)
		if err != nil || !more {
			return err
		}
		if r == nil {
			continue
		}
		who, ok := combatantByID(cs, r.actor)
		if !ok || conv.OrZero(who.ConcentrationSpell) != concentrationReadied {
			continue
		}
		if err := c.q.ReopenReadiedAction(ctx, r.id); err != nil {
			return fmt.Errorf("make the readied spell wait again: %w", err)
		}
		run.alsoUndone = append(run.alsoUndone, older.ID)
	}
	return nil
}

// brokenLine is a readied_lost line of a held spell that broke: its row and its readier.
type brokenLine struct{ id, actor string }

// brokenLineOf reads an event older than the undone one: the broken line it is (nil for another line to go past), and
// whether the walk goes on (the 0 hit points' end of a concentration comes before its lines; anything else ends it).
func brokenLineOf(older playdb.ListRecentSessionEventsRow) (*brokenLine, bool, error) {
	if older.Kind != eventReadiedLost && older.Kind != eventConditionsSet {
		return nil, false, nil
	}
	ev, err := readEvent(older.Payload)
	if err != nil {
		return nil, false, err
	}
	if older.Kind == eventConditionsSet {
		return nil, ev.Reaction != nil && ev.Reaction.Kind == concentrationEnd, nil
	}
	if ev.Readied == nil || ev.Readied.Ended != readiedBroken {
		return nil, true, nil
	}
	return &brokenLine{id: ev.Readied.ID, actor: ev.Actor}, true, nil
}

// olderThan are the events older than last, newest first.
func olderThan(recent []playdb.ListRecentSessionEventsRow, last playdb.ListRecentSessionEventsRow) []playdb.ListRecentSessionEventsRow {
	for i, e := range recent {
		if e.ID == last.ID {
			return recent[i+1:]
		}
	}
	return nil
}
