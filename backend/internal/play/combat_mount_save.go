package play

import (
	"context"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// The saving throw of a rider (SRD 5.1, "Mounted Combat"): if an effect moves the mount against its will while you ride it, or you are
// knocked prone while mounted, you make a DC 10 Dexterity saving throw or fall off, landing prone in a space within 5 feet of the
// mount. The question is a reaction window of its own kind (`mount_save`) that the rider's player or the master answers with RollMountSave
// (combat_mount_save_roll.go): it uses nobody's reaction and holds the turn like a concentration save. Encounter.mount_save is how it
// reads, and no other list carries it.
//
// What asks it, by the path that writes the square or the condition (the map of the writers is `carryRider`'s call sites):
//   - pusher.push: the push of a spell or a feature (Thunderwave, the Open Hand technique) when the mount really moves;
//   - shovePushes: the shove of a mount that pushes it; a shove of a rider is never a push (PUSH_BLOCKED);
//   - dragTo: a grappler's drag of the mount it holds;
//   - shoveKnocksProne, riderConsequence, giveCondition and giveTrapConditions: the rider knocked prone (a shove, a hit's rider, a spell
//     or a zone, a trap);
//
// and what does not: the mount's own move (its rider's player, or the master), the master's token, a teleport (the spell moves its
// caster, so the mount's own or the rider's, which is refused), the return of a mover to the square it left the reach at, the master's
// manual edit of the conditions and every undo. A pull has no path yet: it would call mountPushed from where it writes the square.

// mountSaveDC is the DC of the saving throw, the SRD's 10.
const mountSaveDC = 10

// The causes a window keeps in its trigger.
const (
	mountCausePushed = "pushed"
	mountCauseProne  = "prone"
)

// mountCauseKey is the word a window keeps for its cause.
func mountCauseKey(cause playv1.MountSaveCause) string {
	if cause == playv1.MountSaveCause_MOUNT_SAVE_CAUSE_PRONE {
		return mountCauseProne
	}
	return mountCausePushed
}

// mountCauseOf is the cause a stored word says.
func mountCauseOf(key string) playv1.MountSaveCause {
	if key == mountCauseProne {
		return playv1.MountSaveCause_MOUNT_SAVE_CAUSE_PRONE
	}
	return playv1.MountSaveCause_MOUNT_SAVE_CAUSE_PUSHED
}

// openMountSave opens the save of a rider that sits on the mount, unless it owes one already (one pending save for each rider, whatever
// asks) or it is defeated (the fall of a rider at 0 hit points belongs to another rule). The window keeps the square the mount was
// pushed to, so that the undo of the push, which puts the mount back, closes it.
func (s *Service) openMountSave(ctx context.Context, c *combatTx, _ []playdb.Combatant, rider, mount playdb.Combatant, cause playv1.MountSaveCause) error {
	if rider.Defeated {
		return nil
	}
	open, err := openWindowsOf(ctx, c)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(open, func(w playdb.ReactionWindow) bool {
		return w.Kind == string(reaction.MountSave) && conv.OrZero(w.ReactorID) == rider.ID
	}) {
		return nil
	}
	trigger := windowTrigger{Target: mount.ID, Cause: mountCauseKey(cause)}
	if placed(mount) {
		trigger.AtCol, trigger.AtRow = conv.OrZero(mount.GridCol), conv.OrZero(mount.GridRow)
	}
	_, err = s.openWindows(ctx, c, newGroup(), nil, []windowSpec{{kind: reaction.MountSave, reactor: &rider, trigger: trigger}})
	return err
}

// mountPushed asks the rider's save when the mount an effect moved against its will carries one: it runs after the square of the mount
// and of its rider is written.
func (s *Service) mountPushed(ctx context.Context, c *combatTx, mountID string) error {
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return err
	}
	mount, ok := combatantByID(cs, mountID)
	rider := riderOf(cs, mountID)
	if !ok || rider == nil {
		return nil
	}
	return s.openMountSave(ctx, c, cs, *rider, mount, playv1.MountSaveCause_MOUNT_SAVE_CAUSE_PUSHED)
}

// riderProne asks the rider's save when the combatant an effect knocked prone sits on a mount: it runs after the condition is written.
func (s *Service) riderProne(ctx context.Context, c *combatTx, riderID string) error {
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return err
	}
	rider, ok := combatantByID(cs, riderID)
	if !ok || rider.MountedOnID == nil {
		return nil
	}
	mount := ridingOn(cs, rider)
	if mount == nil {
		return nil
	}
	return s.openMountSave(ctx, c, cs, rider, *mount, playv1.MountSaveCause_MOUNT_SAVE_CAUSE_PRONE)
}

// mountSaveGone says what a window asked is not so any more: the rider or the mount left the combat, the rider is not on that mount, the
// mount is not on the square it was pushed to (the push was undone), or the rider is not prone (the prone was undone or taken off).
func mountSaveGone(w playdb.ReactionWindow, t windowTrigger, byID map[string]playdb.Combatant) bool {
	rider, ok := byID[conv.OrZero(w.ReactorID)]
	mount, there := byID[t.Target]
	if !ok || !there || rider.Defeated || rider.MountedOnID == nil || *rider.MountedOnID != mount.ID {
		return true
	}
	if t.Cause == mountCauseProne {
		return !slices.Contains(rider.Conditions, condProne)
	}
	return !placed(mount) || conv.OrZero(mount.GridCol) != t.AtCol || conv.OrZero(mount.GridRow) != t.AtRow
}

// mountSaveWait adds a rider's save to the line the caller reads. The master reads every one and a player only the character they see
// sit on a mount they see: a hidden mount or rider makes the wait the master's (RN-10).
func (wv *windowView) mountSaveWait(w playdb.ReactionWindow, reactor *playdb.Combatant, answers bool, wait *reaction.Wait) {
	player := reactor != nil && reactor.Kind == kindPlayer
	if wv.v.master {
		if player {
			wait.Savers = append(wait.Savers, reactor.Label)
		} else {
			wait.Self = true
		}
		return
	}
	if answers {
		return
	}
	mount, known := wv.byID[windowTriggerOf(w).Target]
	if player && wv.v.sees(*reactor) && known && wv.v.sees(mount) {
		wait.Savers = append(wait.Savers, reactor.Label)
		return
	}
	wait.Master = true
}

// mountSaveView builds Encounter.mount_save for the caller: the rider's player and the master read it, nobody else.
func (s *Service) mountSaveView(ctx context.Context, m authz.Membership, d *encounterData, v combatViewer) (*playv1.MountSavePrompt, error) {
	if d.enc.Status != statusActive || !slices.ContainsFunc(d.cs, func(c playdb.Combatant) bool { return c.MountedOnID != nil }) {
		return nil, nil
	}
	open, err := s.queries.ListOpenReactionWindows(ctx, d.enc.ID)
	if err != nil {
		return nil, s.dbError(ctx, "list the reaction windows", err)
	}
	byID := combatantsByID(d.cs)
	for _, w := range open {
		if w.Kind != string(reaction.MountSave) {
			continue
		}
		rider, ok := byID[conv.OrZero(w.ReactorID)]
		if !ok || !(v.master || v.owns(rider)) {
			continue
		}
		return s.mountSavePrompt(ctx, m, d, v, w, rider)
	}
	return nil, nil
}

// mountSavePrompt is the prompt of an open window: the rider, the mount when the caller sees it, the DC and the circumstances of the d20.
func (s *Service) mountSavePrompt(ctx context.Context, m authz.Membership, d *encounterData, v combatViewer, w playdb.ReactionWindow, rider playdb.Combatant) (*playv1.MountSavePrompt, error) {
	t := windowTriggerOf(w)
	out := &playv1.MountSavePrompt{RiderId: rider.ID, Dc: mountSaveDC, Cause: mountCauseOf(t.Cause)}
	if mount, ok := combatantByID(d.cs, t.Target); ok && v.sees(mount) {
		out.MountId = mount.ID
	}
	facts, err := s.mountSaveFacts(ctx, nil, s.queries, m.CampaignID, d.enc, rider, statesOf(d.states))
	if err != nil {
		return nil, err
	}
	out.Mode, out.AutoFail = modeToProto[facts.mode], facts.autoFail
	return out, nil
}

// mountSaveFacts are the circumstances of the save a rider makes.
type mountSaveFacts struct {
	mode     combat.RollMode
	autoFail bool
	creature combat.Creature
}

// mountSaveFacts reads the circumstances of the rider's Dexterity saving throw: the mode with the sources the sheet shows for it and the
// automatic failure of a paralyzed, stunned, unconscious or petrified creature. tx is nil for a read through the pool.
func (s *Service) mountSaveFacts(ctx context.Context, tx pgx.Tx, q *playdb.Queries, campaignID string, enc playdb.Encounter, rider playdb.Combatant, states map[string][]playdb.CombatantState) (mountSaveFacts, error) {
	sheet, err := s.sheetOf(ctx, tx, campaignID, rider)
	if err != nil {
		return mountSaveFacts{}, fmt.Errorf("read the rider's sheet: %w", err)
	}
	sun, err := s.sunlightOf(ctx, q, enc)
	if err != nil {
		return mountSaveFacts{}, err
	}
	creature := sun.facts(creatureFacts(rider, states, sheet.Traits), rider)
	sources := combat.SaveMode(combat.SaveScene{Creature: creature, Ability: "dex", EffectVisible: true})
	return mountSaveFacts{mode: combat.Resolve(sources), autoFail: combat.AutoFailsSave(creature, "dex"), creature: creature}, nil
}

// mountSaveHidden says a player who asks is not to be told whose save holds the turn: the rider or its mount is hidden from the
// players. The master is told whatever the window.
func mountSaveHidden(ctx context.Context, c *combatTx, w playdb.ReactionWindow) (bool, error) {
	if c.master {
		return false, nil
	}
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return false, err
	}
	rider, ok := combatantByID(cs, conv.OrZero(w.ReactorID))
	mount, there := combatantByID(cs, windowTriggerOf(w).Target)
	return !ok || !there || rider.Hidden || mount.Hidden, nil
}
