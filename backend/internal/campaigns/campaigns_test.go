package campaigns

import (
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
)

func TestCreateCampaignValidation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")

	tests := []struct {
		name   string
		cname  string
		xpMode campaignsv1.XpMode
		ok     bool
	}{
		{"longest name", strings.Repeat("á", MaxNameLength), campaignsv1.XpMode_XP_MODE_ENEMIES, true},
		{"every XP mode: gold", "Mirathel", campaignsv1.XpMode_XP_MODE_GOLD, true},
		{"every XP mode: milestones", "Mirathel", campaignsv1.XpMode_XP_MODE_MILESTONES, true},
		{"empty name", "", campaignsv1.XpMode_XP_MODE_ENEMIES, false},
		{"only spaces", "   ", campaignsv1.XpMode_XP_MODE_ENEMIES, false},
		{"name too long", strings.Repeat("á", MaxNameLength+1), campaignsv1.XpMode_XP_MODE_ENEMIES, false},
		{"line break", "Mira\nthel", campaignsv1.XpMode_XP_MODE_ENEMIES, false},
		{"no XP mode", "Mirathel", campaignsv1.XpMode_XP_MODE_UNSPECIFIED, false},
		{"unknown XP mode", "Mirathel", campaignsv1.XpMode(99), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res, err := master.api.CreateCampaign(t.Context(), connect.NewRequest(&campaignsv1.CreateCampaignRequest{
				Name:   tt.cname,
				XpMode: tt.xpMode,
			}))
			switch {
			case tt.ok && err != nil:
				t.Errorf("CreateCampaign() error = %v, want it to work", err)
			case tt.ok && res.Msg.GetCampaign().GetXpMode() != tt.xpMode:
				t.Errorf("CreateCampaign() xp_mode = %v, want %v", res.Msg.GetCampaign().GetXpMode(), tt.xpMode)
			case !tt.ok:
				wantCode(t, "CreateCampaign()", err, connect.CodeInvalidArgument)
			}
		})
	}
}

func TestListsAreOrdered(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	first := master.createCampaign(t, "Primeira")
	second := master.createCampaign(t, "Segunda")

	// Campaigns: newest first.
	res, err := master.api.ListMyCampaigns(t.Context(), connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{}))
	if err != nil {
		t.Fatalf("ListMyCampaigns() error = %v", err)
	}
	if got := res.Msg.GetCampaigns(); len(got) != 2 || got[0].GetId() != second.GetId() || got[1].GetId() != first.GetId() {
		t.Errorf("ListMyCampaigns() = %v, want Segunda, then Primeira", got)
	}

	// Members: the master first, then players in the order they joined,
	// with the display names they chose (or none).
	_, token := master.createInvite(t, first.GetId(), 3, 0)
	ana := h.newUser("Ana")
	nameless := h.newUser("")
	ana.join(t, token)
	nameless.join(t, token)
	members, err := ana.api.ListMembers(t.Context(), connect.NewRequest(&campaignsv1.ListMembersRequest{CampaignId: first.GetId()}))
	if err != nil {
		t.Fatalf("ListMembers() error = %v", err)
	}
	var got []string
	for _, m := range members.Msg.GetMembers() {
		got = append(got, m.GetRole().String()+":"+m.GetDisplayName())
	}
	want := []string{"ROLE_MASTER:Mestre", "ROLE_PLAYER:Ana", "ROLE_PLAYER:"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("ListMembers() = %v, want %v", got, want)
	}

	// Invites: newest first.
	h.clock.Advance(time.Minute)
	newer, _ := master.createInvite(t, first.GetId(), 0, 0)
	invites, err := master.api.ListInvites(t.Context(), connect.NewRequest(&campaignsv1.ListInvitesRequest{CampaignId: first.GetId()}))
	if err != nil {
		t.Fatalf("ListInvites() error = %v", err)
	}
	if got := invites.Msg.GetInvites(); len(got) != 2 || got[0].GetId() != newer.GetId() {
		t.Errorf("ListInvites() = %v, want the newer invite first", got)
	}
}

// TestDeletingAnAccount checks the ON DELETE rules that docs/privacidade.md
// promises: a player's memberships go with their account; a master's
// campaigns go with theirs, together with the campaigns' members and
// invites.
func TestDeletingAnAccount(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	player := h.newUser("Jogador")
	campaign := master.createCampaign(t, "Mirathel")
	_, token := master.createInvite(t, campaign.GetId(), 2, 0)
	player.join(t, token)

	deleteUser := func(id string) {
		t.Helper()
		if _, err := h.pool.Exec(t.Context(), "DELETE FROM users WHERE id = $1", id); err != nil {
			t.Fatalf("delete user: %v", err)
		}
	}
	count := func(table string) int {
		t.Helper()
		var n int
		if err := h.pool.QueryRow(t.Context(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}

	deleteUser(player.id)
	if roles := h.memberRoles(campaign.GetId()); len(roles) != 1 || roles[master.id] != "master" {
		t.Errorf("members after the player deleted their account = %v, want only the master", roles)
	}

	deleteUser(master.id)
	for _, table := range []string{"campaigns", "campaign_members", "campaign_invites"} {
		if n := count(table); n != 0 {
			t.Errorf("%s after the master deleted their account = %d rows, want 0", table, n)
		}
	}
}

// TestInvitesAreDeletedByTheDatabase: CockroachDB's row-level TTL deletes
// invites 30 days after they expire (docs/privacidade.md).
func TestInvitesAreDeletedByTheDatabase(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	var name, create string
	if err := h.pool.QueryRow(t.Context(), "SHOW CREATE TABLE campaign_invites").Scan(&name, &create); err != nil {
		t.Fatalf("SHOW CREATE TABLE: %v", err)
	}
	if !strings.Contains(create, "ttl = 'on'") || !strings.Contains(create, `expires_at + INTERVAL \'30 days\'`) {
		t.Errorf("campaign_invites has no row-level TTL of expires_at + 30 days: %s", create)
	}
}
