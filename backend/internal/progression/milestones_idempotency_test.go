package progression

import (
	"context"
	"sync"
	"testing"

	"connectrpc.com/connect"

	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// The audit of 07/10/2026 (F7): a retried AddMilestone must not add the milestone twice.

func (u *user) planWithKey(campaignID, text, key string) (*progressionv1.AddMilestoneResponse, error) {
	res, err := u.xp.AddMilestone(context.Background(), connect.NewRequest(&progressionv1.AddMilestoneRequest{CampaignId: campaignID, Text: text, IdempotencyKey: key}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func TestAddMilestoneIsIdempotent(t *testing.T) {
	t.Parallel()
	tb := newTable(t, milestones, 1)

	first, err := tb.master.planWithKey(tb.campaign, "Chegar ao Vale Seco", "key-1")
	if err != nil {
		t.Fatalf("AddMilestone() error = %v", err)
	}
	again, err := tb.master.planWithKey(tb.campaign, "Chegar ao Vale Seco", "key-1")
	if err != nil {
		t.Fatalf("AddMilestone() retry error = %v", err)
	}
	if again.GetMilestone().GetId() != first.GetMilestone().GetId() {
		t.Errorf("retry made milestone %q, want the first, %q", again.GetMilestone().GetId(), first.GetMilestone().GetId())
	}
	if n := len(again.GetMilestones()); n != 1 {
		t.Errorf("milestones after the retry = %d, want 1", n)
	}
	// The same key with another text is refused, and adds nothing.
	_, err = tb.master.planWithKey(tb.campaign, "Outro marco", "key-1")
	wantCode(t, "AddMilestone(same key, other text)", err, connect.CodeInvalidArgument)
	// A new key is a new milestone; no key is not deduplicated, as before.
	if _, err := tb.master.planWithKey(tb.campaign, "Chegar ao Vale Seco", "key-2"); err != nil {
		t.Fatalf("AddMilestone(new key) error = %v", err)
	}
	for range 2 {
		if _, err := tb.master.planWithKey(tb.campaign, "Sem chave", ""); err != nil {
			t.Fatalf("AddMilestone(no key) error = %v", err)
		}
	}
	if got := len(tb.master.milestones(t, tb.campaign)); got != 4 {
		t.Errorf("milestones = %d, want 4", got)
	}
	// A retry is answered even when the list is full.
	for len(tb.master.milestones(t, tb.campaign)) < maxMilestones {
		tb.master.plan(t, tb.campaign, "Marco")
	}
	if _, err := tb.master.planWithKey(tb.campaign, "Chegar ao Vale Seco", "key-1"); err != nil {
		t.Errorf("AddMilestone(retry at the limit) error = %v, want it answered", err)
	}
	_, err = tb.master.planWithKey(tb.campaign, "Mais um", "key-3")
	wantCode(t, "AddMilestone(new key at the limit)", err, connect.CodeResourceExhausted)
}

func TestAddMilestoneWithTheSameKeyAtOnceAddsOne(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 4) // the racers must overlap: one connection would run them one by one
	tb := newTable(t, milestones, 1)
	ids := make([]string, 4)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := tb.master.planWithKey(tb.campaign, "Chegar ao Vale Seco", "racing")
			if err != nil {
				t.Errorf("AddMilestone() error = %v", err)
				return
			}
			ids[i] = res.GetMilestone().GetId()
		}()
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("ids = %v, want one milestone", ids)
		}
	}
	if got := len(tb.master.milestones(t, tb.campaign)); got != 1 {
		t.Errorf("milestones = %d, want 1", got)
	}
}
