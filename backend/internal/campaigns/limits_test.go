package campaigns

import (
	"crypto/rand"
	"errors"
	"sync"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/identity"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// create tries to create a campaign as u and returns the error.
func (u *user) create(t *testing.T) error {
	t.Helper()
	_, err := u.api.CreateCampaign(t.Context(), connect.NewRequest(&campaignsv1.CreateCampaignRequest{
		Name:   "Mesa",
		XpMode: campaignsv1.XpMode_XP_MODE_MILESTONES,
	}))
	return err
}

// refusal returns the code and the CampaignCreationRefused detail of err.
func refusal(t *testing.T, err error) (connect.Code, *campaignsv1.CampaignCreationRefused) {
	t.Helper()
	connectErr, ok := errors.AsType[*connect.Error](err)
	if !ok {
		t.Fatalf("error = %v, want a Connect error", err)
	}
	for _, d := range connectErr.Details() {
		if msg, err := d.Value(); err == nil {
			if refused, ok := msg.(*campaignsv1.CampaignCreationRefused); ok {
				return connectErr.Code(), refused
			}
		}
	}
	t.Fatalf("error %v has no CampaignCreationRefused detail", err)
	return 0, nil
}

// RN-30: one account is master of a limited number of campaigns.
func TestRN30_TheCampaignCapPerAccount(t *testing.T) {
	t.Parallel()
	h := newHarnessWith(t, func(c *Config) { c.MaxCampaignsPerUser = 2 })
	ana, bia := h.newUser("Ana"), h.newUser("Bia")

	for range 2 {
		if err := ana.create(t); err != nil {
			t.Fatalf("within the cap: %v", err)
		}
	}
	code, detail := refusal(t, ana.create(t))
	if code != connect.CodeResourceExhausted ||
		detail.GetReason() != campaignsv1.CampaignCreationRefusedReason_CAMPAIGN_CREATION_REFUSED_REASON_LIMIT_REACHED ||
		detail.GetMaxCampaigns() != 2 {
		t.Fatalf("over the cap: code %v, detail %v", code, detail)
	}
	// Another account has its own count.
	if err := bia.create(t); err != nil {
		t.Fatalf("another account: %v", err)
	}
	// Being a player somewhere does not count: only campaigns she is master of.
	campaign := bia.createCampaign(t, "De Bia")
	_, token := bia.createInvite(t, campaign.GetId(), 0, 0)
	if _, err := ana.accept(t, token); err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	if code, _ := refusal(t, ana.create(t)); code != connect.CodeResourceExhausted {
		t.Fatalf("a campaign she plays in is not a free slot: code %v", code)
	}
}

// Two creations at the same time cannot both slip under the cap.
func TestRN30_TheCapHoldsUnderConcurrentCreations(t *testing.T) {
	dbtest.PoolSize(t, 8) // the racers must overlap: one connection would run them one by one
	h := newHarnessWith(t, func(c *Config) { c.MaxCampaignsPerUser = 3 })
	ana := h.newUser("Ana")

	const racers = 8
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		done int
	)
	start := dbtest.NewBarrier(racers)
	for range racers {
		wg.Go(func() {
			start.Wait()
			if err := ana.create(t); err == nil {
				mu.Lock()
				done++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if done != 3 {
		t.Fatalf("%d of %d concurrent creations succeeded, want exactly 3", done, racers)
	}
}

// RN-30: with an allow-list, only a verified e-mail on it may create.
func TestRN30_TheCreatorsAllowList(t *testing.T) {
	t.Parallel()
	h := newHarnessWith(t, func(c *Config) { c.Creators = []string{"mestre@example.com"} })
	signUp := func(email string) *user {
		t.Helper()
		id, err := h.users.UpsertUser(t.Context(), identity.ExternalIdentity{Issuer: "https://idp.test", Subject: rand.Text(), Email: email})
		if err != nil {
			t.Fatalf("UpsertUser: %v", err)
		}
		return &user{id: id, api: h.client(id), doc: h.documentClient(id)}
	}

	// The e-mail is matched without regard to case.
	if err := signUp("Mestre@Example.com").create(t); err != nil {
		t.Fatalf("a listed e-mail: %v", err)
	}
	for name, u := range map[string]*user{
		"another e-mail":     signUp("outro@example.com"),
		"no verified e-mail": signUp(""), // an unverified e-mail is never stored
		"no e-mail at all":   h.newUser("Sem e-mail"),
	} {
		code, detail := refusal(t, u.create(t))
		if code != connect.CodePermissionDenied ||
			detail.GetReason() != campaignsv1.CampaignCreationRefusedReason_CAMPAIGN_CREATION_REFUSED_REASON_NOT_ALLOWED {
			t.Errorf("%s: code %v, detail %v", name, code, detail)
		}
	}

	// No list, no refusal: anyone creates.
	open := newHarness(t)
	if err := open.newUser("Qualquer").create(t); err != nil {
		t.Fatalf("with no allow-list: %v", err)
	}
}
