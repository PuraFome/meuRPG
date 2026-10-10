package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master reveals a hider (SRD 5.1, "Hiding": the DM decides when circumstances reveal a
// hidden creature). The fixture is the hide tests': Toren the rogue hides with a d20 of 19.

func (c *cx) revealHider(t *testing.T, u *user, label string) (*playv1.RevealHiderResponse, error) {
	t.Helper()
	res, err := u.contests.RevealHider(t.Context(), connect.NewRequest(&playv1.RevealHiderRequest{
		CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: c.id(t, label),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func TestRevealHiderEndsTheHidingForEveryoneAndTheMasterCanUndoIt(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	toren := c.id(t, "Toren")

	_, err := c.revealHider(t, a.master, "Toren")
	wantContestBlocked(t, "RevealHider on a combatant that is not hidden", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_HIDDEN)

	tried := c.mustHide(t, a.caio, "Toren", cunningHide, 19)
	c.mustResolveHide(t, tried.GetId(), nil)
	if got := c.state(t, a.caio).GetHiddenIds(); !slices.Equal(got, []string{toren}) {
		t.Fatalf("Toren reads %v hidden, want himself", got)
	}

	// Only the master reveals, and only inside the campaign.
	_, err = c.revealHider(t, a.caio, "Toren")
	wantCode(t, "RevealHider as the hider's player", err, connect.CodePermissionDenied)
	outs := a.outsiders(t)
	outs.refused(t, "RevealHider", func(u *user, campaignID string) error {
		_, err := u.contests.RevealHider(t.Context(), connect.NewRequest(&playv1.RevealHiderRequest{
			CampaignId: campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: toren,
		}))
		return err
	}, a.campaignID)

	if _, err := c.revealHider(t, a.master, "Toren"); err != nil {
		t.Fatalf("RevealHider() error = %v", err)
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if got := c.state(t, u).GetHiddenIds(); len(got) != 0 {
			t.Errorf("%s reads %v hidden after the reveal, want nobody", who, got)
		}
	}
	// The unseen-attacker advantage is gone.
	if tg := targetOf(a.mustOptions(t, a.caio, c.refresh(t), "Toren"), rapier, c.hob); tg == nil || len(tg.GetSources()) != 0 {
		t.Errorf("the Hobgoblin as a target = %v, want no source after the reveal", tg)
	}
	// The log: the master and the hider's player read it; the other players do not (RN-10).
	if en := a.contestLine(t, a.master, c.e, contestHideRevealed); en == nil || en.GetActorLabel() != "Toren" || !en.GetUndoable() {
		t.Errorf("the master's line = %v, want Toren's reveal, undoable", en)
	}
	if en := a.contestLine(t, a.caio, c.e, contestHideRevealed); en == nil || en.GetActorLabel() != "Toren" {
		t.Errorf("Toren's player reads %v, want the reveal", en)
	}
	for who, u := range map[string]*user{"Pensantus's player": a.ana, "Brisa's player": a.bia} {
		if en := a.contestLine(t, u, c.e, contestHideRevealed); en != nil {
			t.Errorf("%s reads the reveal: %v", who, en)
		}
	}

	// The undo gives the hiding back as it was.
	a.undoLast(t, c.e)
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if got := c.state(t, u).GetHiddenIds(); !slices.Equal(got, []string{toren}) {
			t.Errorf("%s reads %v hidden after the undo, want Toren", who, got)
		}
	}
}

// TestUndoingAHideDecisionIsStillRefused: the reveal is undoable, the master's decision on an
// attempt is not (the undo chain closes on it).
func TestUndoingAHideDecisionIsStillRefused(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	tried := c.mustHide(t, a.caio, "Toren", cunningHide, 19)
	c.mustResolveHide(t, tried.GetId(), nil)
	if id := a.log(t, a.master, c.refresh(t)).GetUndoableEventId(); id != "" {
		t.Errorf("the undoable event after a ResolveHide is %q, want none", id)
	}
}
