package progression

import (
	"context"
	"errors"
	"sync"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

func (u *user) setXPMode(campaignID string, mode campaignsv1.XpMode, confirm bool) error {
	_, err := u.campaigns.SetCampaignXpMode(context.Background(), connect.NewRequest(&campaignsv1.SetCampaignXpModeRequest{CampaignId: campaignID, XpMode: mode, Confirm: confirm}))
	return err
}

// blockedXP returns how much XP the refusal of a change of the XP mode says was
// awarded.
func blockedXP(t *testing.T, err error) *campaignsv1.XpModeChangeBlocked {
	t.Helper()
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("SetCampaignXpMode() error = %v, want failed_precondition", err)
	}
	ce, _ := errors.AsType[*connect.Error](err)
	for _, d := range ce.Details() {
		if v, derr := d.Value(); derr == nil {
			if b, ok := v.(*campaignsv1.XpModeChangeBlocked); ok {
				return b
			}
		}
	}
	t.Fatalf("SetCampaignXpMode() error %v has no XpModeChangeBlocked detail", err)
	return nil
}

// TestRN09_ChangingTheXPModeAsksOnceXPWasAwarded runs the change of the XP mode
// against the real awards: free while nothing stands, a confirmation that says
// how much was awarded afterwards, an undone award not counting, a milestone
// mark counting with no XP, and nothing already awarded being converted.
func TestRN09_ChangingTheXPModeAsksOnceXPWasAwarded(t *testing.T) {
	t.Parallel()
	tb := newTable(t, enemies, 2)
	master := tb.master

	// Nothing awarded: the mode changes at once.
	if err := master.setXPMode(tb.campaign, campaignsv1.XpMode_XP_MODE_GOLD, false); err != nil {
		t.Fatalf("SetCampaignXpMode(gold) with no awards error = %v", err)
	}
	if err := master.setXPMode(tb.campaign, campaignsv1.XpMode_XP_MODE_ENEMIES, false); err != nil {
		t.Fatalf("SetCampaignXpMode(enemies) error = %v", err)
	}

	// 60 XP awarded: a change needs `confirm`, and the refusal says how much.
	tb.master.manual(t, tb.campaign, 40, tb.ids(2)...)
	tb.master.manual(t, tb.campaign, 20, tb.ids(1)...)
	b := blockedXP(t, master.setXPMode(tb.campaign, campaignsv1.XpMode_XP_MODE_MILESTONES, false))
	if b.GetAwards() != 2 || b.GetTotalXp() != 60 {
		t.Errorf("detail = %v, want 2 awards and 60 XP", b)
	}
	// With confirm it changes, and the XP already given stays on the sheets.
	if err := master.setXPMode(tb.campaign, campaignsv1.XpMode_XP_MODE_MILESTONES, true); err != nil {
		t.Fatalf("SetCampaignXpMode(milestones, confirm) error = %v", err)
	}
	if got := master.xpOf(t, tb.pcs[0]); got != 40 {
		t.Errorf("the first character's XP after the change = %d, want 40 (20 + 20, nothing converted)", got)
	}
	if h := master.history(t, tb.campaign); len(h) != 2 {
		t.Errorf("the history has %d awards after the change, want the same 2", len(h))
	}
	// A milestone mark is an award with no XP: it counts as one.
	tb.master.milestone(t, tb.campaign, tb.ids(1)...)
	b = blockedXP(t, master.setXPMode(tb.campaign, campaignsv1.XpMode_XP_MODE_GOLD, false))
	if b.GetAwards() != 3 || b.GetTotalXp() != 60 {
		t.Errorf("detail after a milestone = %v, want 3 awards and still 60 XP", b)
	}
}

// TestRN09_AnUndoneAwardDoesNotCount: undoing the last award leaves nothing that
// stands, so the mode changes without a confirmation.
func TestRN09_AnUndoneAwardDoesNotCount(t *testing.T) {
	t.Parallel()
	tb := newTable(t, enemies, 1)
	tb.master.manual(t, tb.campaign, 30, tb.ids(1)...)
	blockedXP(t, tb.master.setXPMode(tb.campaign, campaignsv1.XpMode_XP_MODE_GOLD, false))
	if _, err := tb.master.undo(tb.campaign, newKey()); err != nil {
		t.Fatalf("UndoLastXPAward() error = %v", err)
	}
	if err := tb.master.setXPMode(tb.campaign, campaignsv1.XpMode_XP_MODE_GOLD, false); err != nil {
		t.Errorf("SetCampaignXpMode() after the undo error = %v, want none: no award stands", err)
	}
}

// TestRN09_AnAwardAndAChangeOfTheXPModeTakeTurns: an award racing the change of the
// XP mode never slips between the confirmation's check and the change. For each
// round, either the change passed (and then the award was refused by the new mode,
// so nothing was awarded without a confirmation) or the award landed (and then the
// change asked for `confirm`).
func TestRN09_AnAwardAndAChangeOfTheXPModeTakeTurns(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 4) // the racers must overlap: one connection would run them one by one
	h := newHarness(t)
	master, ana := h.newUser("Samuel"), h.newUser("Ana")
	changed, awarded := 0, 0
	for round := range 8 {
		campaign := h.newCampaign(master, "Mirathel", enemies, ana)
		pc := ana.pc(t, campaign, "Pensantus")
		var wg sync.WaitGroup
		var changeErr, awardErr error
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			changeErr = master.setXPMode(campaign, campaignsv1.XpMode_XP_MODE_MILESTONES, false)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, awardErr = master.award(campaign, func(r *progressionv1.AwardXPRequest) {
				r.Mode, r.Amount, r.CharacterIds = progressionv1.XPAwardMode_XP_AWARD_MODE_MANUAL, 50, []string{pc.GetId()}
			})
		}()
		close(start)
		wg.Wait()
		live := len(master.history(t, campaign))
		switch {
		case changeErr == nil && awardErr == nil:
			t.Fatalf("round %d: the award (50 XP) committed and the mode changed with no confirmation", round)
		case changeErr == nil:
			changed++
			if live != 0 || connect.CodeOf(awardErr) != connect.CodeFailedPrecondition {
				t.Errorf("round %d: the mode changed, the award error = %v, history %d; want it refused and nothing awarded", round, awardErr, live)
			}
		case awardErr == nil:
			awarded++
			if live != 1 || connect.CodeOf(changeErr) != connect.CodeFailedPrecondition {
				t.Errorf("round %d: the award landed, the change error = %v, history %d; want a confirmation asked", round, changeErr, live)
			}
		default:
			t.Errorf("round %d: both failed: change %v, award %v", round, changeErr, awardErr)
		}
	}
	t.Logf("the change won %d rounds, the award %d", changed, awarded)
}
