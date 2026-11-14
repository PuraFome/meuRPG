package play

import (
	"context"
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// The dismount window (SRD 5.1, "Mounted Combat"): if the mount is knocked prone, the rider can use its reaction to land on its feet
// in a space within 5 feet of the mount, or else it falls prone; the same goes for a mount that is killed under its rider. The window
// is the rider's reaction, so it has the rules of one: only the rider's player answers it (the master for an NPC or a creature), a
// rider that has no reaction, or cannot use it, falls at once with no window, and it holds the turn like any other reaction (reaction
// pending). Its answer closes the undo chain. A mount a caster dismisses (a creature) lets the rider land on its feet with no window:
// syncCreatures takes the rider off. What falls to the window is the mount that stays: knocked prone, or an NPC at 0 hit points.

// The causes a dismount window keeps in its trigger.
const (
	mountCauseMountProne    = "mount_prone"
	mountCauseMountDefeated = "mount_defeated"
)

// riderCanReact says the rider has a reaction it may spend now: it is not spent, not incapacitated, not down.
func (s *Service) riderCanReact(ctx context.Context, c *combatTx, rider playdb.Combatant) (bool, error) {
	if rider.ReactionUsed || rider.EffectNoReaction || slices.ContainsFunc(rider.Conditions, func(k string) bool { return slices.Contains(incapacitating, k) }) {
		return false, nil
	}
	if rider.Kind == kindPlayer {
		down, err := s.isDown(ctx, c.tx, c.session.CampaignID, rider)
		return !down, err
	}
	return true, nil
}

// openDismount asks the rider whose mount was knocked prone, or defeated, whether it lands on its feet; a rider that cannot react
// falls at once. A rider that is defeated, or already has the window, is left alone.
func (s *Service) openDismount(ctx context.Context, c *combatTx, _ []playdb.Combatant, rider, mount playdb.Combatant, cause string) error {
	if rider.Defeated || rider.MountedOnID == nil || *rider.MountedOnID != mount.ID {
		return nil
	}
	open, err := openWindowsOf(ctx, c)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(open, func(w playdb.ReactionWindow) bool {
		return w.Kind == string(reaction.Dismount) && conv.OrZero(w.ReactorID) == rider.ID
	}) {
		return nil
	}
	can, err := s.riderCanReact(ctx, c, rider)
	if err != nil {
		return err
	}
	if !can {
		return s.dropRider(ctx, c, rider, mount, cause, true)
	}
	_, err = s.openWindows(ctx, c, newGroup(), nil, []windowSpec{{kind: reaction.Dismount, reactor: &rider, trigger: windowTrigger{Target: mount.ID, Cause: cause}}})
	return err
}

// dropRider takes the rider off the mount: prone when it fell, on its feet when it landed. The fall has the line of a thrown rider.
func (s *Service) dropRider(ctx context.Context, c *combatTx, rider, mount playdb.Combatant, cause string, prone bool) error {
	ev := actionEvent{Round: c.enc.Round, Secret: rider.Hidden || mount.Hidden, Actor: rider.ID, Target: mount.ID}
	ev.Reaction = &reactionEvent{Kind: string(reaction.MountSave), Cause: cause, AutoFail: true}
	return s.dropRiderOf(ctx, c, rider, mount, &ev, &mountSaveRun{}, prone)
}

// openDefeatedMounts opens the dismount window of the rider of every NPC mount that is at 0 hit points: it runs after each change. A
// creature that is defeated is dismissed instead, and its rider lands on its feet with no window.
func (s *Service) openDefeatedMounts(ctx context.Context, c *combatTx) error {
	if c.enc.ID == "" || c.enc.Status != statusActive {
		return nil
	}
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return err
	}
	for _, mount := range cs {
		if !mount.Defeated || isCreature(mount) {
			continue
		}
		if rider := riderOf(cs, mount.ID); rider != nil {
			if err := s.openDismount(ctx, c, cs, *rider, mount, mountCauseMountDefeated); err != nil {
				return err
			}
		}
	}
	return nil
}

// mountKnockedProne opens the dismount window of the rider of a mount that was knocked prone.
func (s *Service) mountKnockedProne(ctx context.Context, c *combatTx, cs []playdb.Combatant, mount playdb.Combatant) error {
	if mount.Defeated {
		return nil
	}
	if rider := riderOf(cs, mount.ID); rider != nil {
		return s.openDismount(ctx, c, cs, *rider, mount, mountCauseMountProne)
	}
	return nil
}

// dismountGone says what a window asked is not so any more: the rider or the mount left, the rider is not on that mount or is
// defeated, or the mount is not prone (or not defeated) any more.
func dismountGone(w playdb.ReactionWindow, t windowTrigger, byID map[string]playdb.Combatant) bool {
	rider, ok := byID[conv.OrZero(w.ReactorID)]
	mount, there := byID[t.Target]
	if !ok || !there || rider.Defeated || rider.MountedOnID == nil || *rider.MountedOnID != mount.ID {
		return true
	}
	if t.Cause == mountCauseMountDefeated {
		return !mount.Defeated
	}
	return mount.Defeated || !slices.Contains(mount.Conditions, condProne)
}

// answerDismount: "Usar" spends the reaction and lands the rider on its feet within 5 feet of the mount; "Deixar passar" throws it
// prone, as a fall from a mount does.
func (s *Service) answerDismount(ctx context.Context, c *combatTx, w playdb.ReactionWindow, reactor playdb.Combatant, use bool, got *answered) error {
	t := windowTriggerOf(w)
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return err
	}
	mount, ok := combatantByID(cs, t.Target)
	if !ok || reactor.MountedOnID == nil || *reactor.MountedOnID != mount.ID {
		return errNotYourTurnToAnswer()
	}
	secret := reactor.Hidden || mount.Hidden
	got.result = &playv1.ReactionResult{Kind: playv1.ReactionKind_REACTION_KIND_DISMOUNT, ByMaster: c.master, Used: use}
	ev := actionEvent{Round: c.enc.Round, Secret: secret, Actor: reactor.ID, Target: mount.ID, ReactionBefore: reactor.ReactionUsed}
	if !use {
		ev.Reaction = &reactionEvent{Kind: w.Kind, Window: w.ID, ByMaster: c.master}
		got.ev = ev
		if _, err := s.closeWindow(ctx, c, w, windowAnswered, reaction.ReasonNone, windowOutcome{ByMaster: c.master}, ""); err != nil {
			return err
		}
		return s.dropRider(ctx, c, reactor, mount, t.Cause, true)
	}
	if err := s.spendReaction(ctx, c, reactor, reaction.Dismount); err != nil {
		return err
	}
	if _, err := s.closeWindow(ctx, c, w, windowAnswered, reaction.ReasonNone, windowOutcome{ByMaster: c.master, Used: true}, ""); err != nil {
		return err
	}
	after := reactor
	after.ReactionUsed = true
	run := &mountSaveRun{}
	landing := actionEvent{Round: c.enc.Round, Secret: secret, Actor: reactor.ID, Target: mount.ID, Reaction: &reactionEvent{Kind: string(reaction.MountSave), Cause: t.Cause}}
	if err := s.dropRiderOf(ctx, c, after, mount, &landing, run, false); err != nil {
		return err
	}
	ev.Reaction = &reactionEvent{Kind: w.Kind, Window: w.ID, Used: true, ByMaster: c.master, LandCol: landing.Reaction.LandCol, LandRow: landing.Reaction.LandRow, NoRoom: landing.Reaction.NoRoom}
	got.ev = ev
	c.lastAnswered = reaction.Dismount
	c.characterID = &reactor.CharacterID
	return nil
}

// dismountWait adds a rider's dismount to the line the caller reads: the master reads every one, a player only the character they
// see sit on a mount they see. A hidden mount or rider makes the wait the master's (RN-10), and so does a creature's.
func (wv *windowView) dismountWait(w playdb.ReactionWindow, reactor *playdb.Combatant, answers bool, wait *reaction.Wait) {
	player := reactor != nil && reactor.Kind == kindPlayer
	if wv.v.master {
		if player {
			wait.Reactors = append(wait.Reactors, reactor.Label)
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
		wait.Reactors = append(wait.Reactors, reactor.Label)
		return
	}
	wait.Master = true
}

// dismount is the line of a rider that used its reaction to land on its feet: it names nobody the viewer does not see, and a mount
// or a rider the players do not see is the master's alone.
func (f *reactionFacts) dismount() (string, bool) {
	if !f.re.Used || !f.seesReactor || !f.seesOther {
		return "", false
	}
	return f.who + " usou a reação para desmontar de pé." + f.by, true
}

// fallWithWindow throws the rider of a dismount window that closed by itself because its reactor could not react (it was
// incapacitated, or its reaction was spent): with no answer to wait for, the rider falls. A window whose trigger went away throws nobody.
func (s *Service) fallWithWindow(ctx context.Context, c *combatTx, w playdb.ReactionWindow, f reaction.Facts, byID map[string]playdb.Combatant) error {
	if w.Kind != string(reaction.Dismount) || f.TriggerGone {
		return nil
	}
	t := windowTriggerOf(w)
	rider, mount := byID[conv.OrZero(w.ReactorID)], byID[t.Target]
	if rider.ID == "" || mount.ID == "" || rider.Defeated {
		return nil
	}
	return s.dropRider(ctx, c, rider, mount, t.Cause, true)
}
