package campaigns

import (
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
)

// The audit of 07/10/2026 (F7): a retried CreateCampaign, after a timeout or a second tap, must
// not make a second campaign.

func createWithKey(t *testing.T, u *user, name, key string) (*campaignsv1.Campaign, error) {
	t.Helper()
	res, err := u.api.CreateCampaign(t.Context(), connect.NewRequest(&campaignsv1.CreateCampaignRequest{
		Name: name, XpMode: campaignsv1.XpMode_XP_MODE_MILESTONES, IdempotencyKey: key,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCampaign(), nil
}

func TestCreateCampaignIsIdempotent(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")

	first, err := createWithKey(t, master, "Mirathel", "key-1")
	if err != nil {
		t.Fatalf("CreateCampaign() error = %v", err)
	}
	again, err := createWithKey(t, master, "Mirathel", "key-1")
	if err != nil {
		t.Fatalf("CreateCampaign() retry error = %v", err)
	}
	if !proto.Equal(again, first) {
		t.Errorf("retry = %v, want the first campaign %v", again, first)
	}
	// The same key with another request is refused, and makes nothing.
	_, err = createWithKey(t, master, "Outra", "key-1")
	wantCode(t, "CreateCampaign(same key, other name)", err, connect.CodeInvalidArgument)

	list, err := master.api.ListMyCampaigns(t.Context(), connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{}))
	if err != nil {
		t.Fatalf("ListMyCampaigns() error = %v", err)
	}
	if n := len(list.Msg.GetCampaigns()); n != 1 {
		t.Errorf("campaigns = %d, want 1", n)
	}

	// A new key is a new campaign; no key is not deduplicated, as before.
	second, err := createWithKey(t, master, "Mirathel", "key-2")
	if err != nil || second.GetId() == first.GetId() {
		t.Errorf("new key = %v, %v; want another campaign", second, err)
	}
	a, _ := createWithKey(t, master, "Sem chave", "")
	b, _ := createWithKey(t, master, "Sem chave", "")
	if a == nil || b == nil || a.GetId() == b.GetId() {
		t.Errorf("no key made %v and %v, want two campaigns", a, b)
	}

	// The key belongs to its user: another person's same key is their own campaign.
	other := h.newUser("Outro")
	theirs, err := createWithKey(t, other, "Mirathel", "key-1")
	if err != nil || theirs.GetId() == first.GetId() {
		t.Errorf("another user's key = %v, %v; want their own campaign", theirs, err)
	}

	_, err = createWithKey(t, master, "Mirathel", string(make([]byte, 65)))
	wantCode(t, "CreateCampaign(a key too long)", err, connect.CodeInvalidArgument)
}

func TestCreateCampaignWithTheSameKeyAtOnceMakesOne(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")

	const calls = 6
	ids := make([]string, calls)
	var wg sync.WaitGroup
	for i := range calls {
		wg.Go(func() {
			if c, err := createWithKey(t, master, "Mirathel", "racing"); err == nil {
				ids[i] = c.GetId()
			} else {
				t.Errorf("CreateCampaign() error = %v", err)
			}
		})
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("ids = %v, want one campaign", ids)
		}
	}
	list, err := master.api.ListMyCampaigns(t.Context(), connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{}))
	if err != nil {
		t.Fatalf("ListMyCampaigns() error = %v", err)
	}
	if n := len(list.Msg.GetCampaigns()); n != 1 {
		t.Errorf("campaigns = %d, want 1", n)
	}
}
