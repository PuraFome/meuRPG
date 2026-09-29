package campaigns

import (
	"crypto/sha256"
	"encoding/base64"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
)

// Acceptance tests: one per acceptance criterion in
// docs/produto/historias.md, named after the story. A story's backend part
// is done when its tests pass.

// MR-001: Dado que estou logado, quando crio a campanha "Mirathel", então
// viro mestre dela e só os membros a veem na lista.
func TestMR001_CreatorBecomesMasterAndOnlyMembersSeeTheCampaign(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre := h.newUser("Mestre")
	jogador := h.newUser("Jogador")
	outsider := h.newUser("Outra pessoa")

	campaign := mestre.createCampaign(t, "  Mirathel ")
	if campaign.GetName() != "Mirathel" || campaign.GetMyRole() != campaignsv1.Role_ROLE_MASTER ||
		campaign.GetXpMode() != campaignsv1.XpMode_XP_MODE_MILESTONES || campaign.GetId() == "" {
		t.Fatalf("CreateCampaign() = %v, want Mirathel (trimmed), with the creator as master", campaign)
	}
	if got := h.memberRoles(campaign.GetId()); len(got) != 1 || got[mestre.id] != "master" {
		t.Errorf("campaign_members = %v, want only the creator, as master", got)
	}

	// Members see it in their list: the master, and a player who joins.
	_, token := mestre.createInvite(t, campaign.GetId(), 0, 0)
	jogador.join(t, token)
	for who, u := range map[campaignsv1.Role]*user{campaignsv1.Role_ROLE_MASTER: mestre, campaignsv1.Role_ROLE_PLAYER: jogador} {
		res, err := u.api.ListMyCampaigns(t.Context(), connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{}))
		if err != nil {
			t.Fatalf("ListMyCampaigns() error = %v", err)
		}
		if got := res.Msg.GetCampaigns(); len(got) != 1 || got[0].GetId() != campaign.GetId() || got[0].GetMyRole() != who {
			t.Errorf("ListMyCampaigns() for the %v = %v, want Mirathel with that role", who, got)
		}
	}

	// Anyone else does not see it, and cannot even tell that it exists.
	res, err := outsider.api.ListMyCampaigns(t.Context(), connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{}))
	if err != nil || len(res.Msg.GetCampaigns()) != 0 {
		t.Errorf("ListMyCampaigns() for a non-member = %v, %v; want an empty list", res, err)
	}
	_, err = outsider.api.GetCampaign(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: campaign.GetId()}))
	wantCode(t, "GetCampaign() by a non-member", err, connect.CodeNotFound)
	_, err = h.anonymous().ListMyCampaigns(t.Context(), connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{}))
	wantCode(t, "ListMyCampaigns() signed out", err, connect.CodeUnauthenticated)
}

// MR-002 (proposed criterion): Dado que sou mestre de "Mirathel", quando
// gero um convite, então recebo um link que vale para uma pessoa por 7 dias
// e o servidor guarda só o hash do token.
func TestMR002_MasterGetsASingleUseSevenDayInviteStoredAsAHash(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre := h.newUser("Mestre")
	campaign := mestre.createCampaign(t, "Mirathel")

	invite, token := mestre.createInvite(t, campaign.GetId(), 0, 0)
	if invite.GetMaxUses() != 1 || invite.GetUseCount() != 0 || invite.GetState() != campaignsv1.InviteState_INVITE_STATE_ACTIVE {
		t.Errorf("CreateInvite() = %v, want an active invite for one person", invite)
	}
	if got, want := invite.GetExpiresAt().AsTime(), h.clock.Now().Add(7*24*time.Hour); !got.Equal(want) {
		t.Errorf("invite expires at %v, want %v (7 days)", got, want)
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token %q is not 32 random bytes in base64url", token)
	}

	// The database has the SHA-256 of the token, and the token nowhere.
	var storedHash []byte
	var row string
	err = h.pool.QueryRow(t.Context(), `
		SELECT token_hash, (i.*)::TEXT FROM campaign_invites AS i WHERE id = $1`, invite.GetId(),
	).Scan(&storedHash, &row)
	if err != nil {
		t.Fatalf("read invite: %v", err)
	}
	if want := sha256.Sum256(raw); !slices.Equal(storedHash, want[:]) {
		t.Errorf("token_hash = %x, want the SHA-256 of the token", storedHash)
	}
	if strings.Contains(row, token) {
		t.Errorf("the invite row contains the token itself: %s", row)
	}

	// The master sees the invite in the list, without its token.
	list, err := mestre.api.ListInvites(t.Context(), connect.NewRequest(&campaignsv1.ListInvitesRequest{CampaignId: campaign.GetId()}))
	if err != nil {
		t.Fatalf("ListInvites() error = %v", err)
	}
	if got := list.Msg.GetInvites(); len(got) != 1 || got[0].GetId() != invite.GetId() || strings.Contains(got[0].String(), token) {
		t.Errorf("ListInvites() = %v, want the invite, without its token", got)
	}
}

// MR-002 (proposed criterion): Dado um convite ainda não usado, quando o
// mestre o revoga, então o link para de funcionar e quem já entrou continua
// na campanha.
func TestMR002_RevokedInviteStopsWorking(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre := h.newUser("Mestre")
	jogador := h.newUser("Jogador")
	latecomer := h.newUser("Atrasado")
	campaign := mestre.createCampaign(t, "Mirathel")
	invite, token := mestre.createInvite(t, campaign.GetId(), 2, 0)
	jogador.join(t, token)

	revoke := func() *campaignsv1.Invite {
		t.Helper()
		res, err := mestre.api.RevokeInvite(t.Context(), connect.NewRequest(&campaignsv1.RevokeInviteRequest{
			CampaignId: campaign.GetId(),
			InviteId:   invite.GetId(),
		}))
		if err != nil {
			t.Fatalf("RevokeInvite() error = %v", err)
		}
		return res.Msg.GetInvite()
	}
	revoked := revoke()
	if revoked.GetState() != campaignsv1.InviteState_INVITE_STATE_REVOKED || !revoked.GetRevokedAt().AsTime().Equal(h.clock.Now()) {
		t.Errorf("RevokeInvite() = %v, want it revoked now", revoked)
	}

	_, err := latecomer.accept(t, token)
	if got := inviteUnusableState(t, err); got != campaignsv1.InviteState_INVITE_STATE_REVOKED {
		t.Errorf("AcceptInvite() after revoking: state %v, want REVOKED", got)
	}
	roles := h.memberRoles(campaign.GetId())
	if _, ok := roles[latecomer.id]; ok || roles[jogador.id] != "player" {
		t.Errorf("members = %v, want the player who joined before, and not the latecomer", roles)
	}

	// Revoking again is fine, and keeps the first revocation time.
	h.clock.Advance(time.Hour)
	if again := revoke(); !again.GetRevokedAt().AsTime().Equal(revoked.GetRevokedAt().AsTime()) {
		t.Errorf("second RevokeInvite() revoked_at = %v, want %v", again.GetRevokedAt().AsTime(), revoked.GetRevokedAt().AsTime())
	}
}

// MR-002 (proposed criterion): Dado que sou jogador de "Mirathel", quando
// tento gerar, listar ou revogar convites, então o servidor recusa.
func TestMR002_OnlyTheMasterManagesInvites(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre := h.newUser("Mestre")
	jogador := h.newUser("Jogador")
	campaign := mestre.createCampaign(t, "Mirathel")
	invite, token := mestre.createInvite(t, campaign.GetId(), 0, 0)
	jogador.join(t, token)
	ctx, id := t.Context(), campaign.GetId()

	_, err := jogador.api.CreateInvite(ctx, connect.NewRequest(&campaignsv1.CreateInviteRequest{CampaignId: id}))
	wantCode(t, "CreateInvite() by a player", err, connect.CodePermissionDenied)
	_, err = jogador.api.ListInvites(ctx, connect.NewRequest(&campaignsv1.ListInvitesRequest{CampaignId: id}))
	wantCode(t, "ListInvites() by a player", err, connect.CodePermissionDenied)
	_, err = jogador.api.RevokeInvite(ctx, connect.NewRequest(&campaignsv1.RevokeInviteRequest{CampaignId: id, InviteId: invite.GetId()}))
	wantCode(t, "RevokeInvite() by a player", err, connect.CodePermissionDenied)

	var invites int
	if err := h.pool.QueryRow(ctx, "SELECT count(*) FROM campaign_invites WHERE revoked_at IS NULL").Scan(&invites); err != nil || invites != 1 {
		t.Errorf("active invites = %d, %v; want only the master's one", invites, err)
	}
}

// MR-003, first criterion, the part this module owns: Dado um convite válido
// para "Mirathel", quando o jogador abre o link e faz login, então vira
// jogador da campanha, que o mestre já vê na campanha. (Creating the player's
// character comes with the characters module, in Etapa 4.)
func TestMR003_ValidInviteMakesTheUserAPlayerTheMasterSees(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre := h.newUser("Mestre")
	jogador := h.newUser("Pensantus")
	campaign := mestre.createCampaign(t, "Mirathel")
	invite, token := mestre.createInvite(t, campaign.GetId(), 0, 0)

	res, err := jogador.accept(t, token)
	if err != nil {
		t.Fatalf("AcceptInvite() error = %v", err)
	}
	if got := res.GetCampaign(); got.GetId() != campaign.GetId() || got.GetName() != "Mirathel" ||
		got.GetMyRole() != campaignsv1.Role_ROLE_PLAYER || res.GetAlreadyMember() {
		t.Errorf("AcceptInvite() = %v, want Mirathel, as a new player", res)
	}
	if n := h.useCount(invite.GetId()); n != 1 {
		t.Errorf("use_count = %d, want 1", n)
	}

	members, err := mestre.api.ListMembers(t.Context(), connect.NewRequest(&campaignsv1.ListMembersRequest{CampaignId: campaign.GetId()}))
	if err != nil {
		t.Fatalf("ListMembers() error = %v", err)
	}
	got := members.Msg.GetMembers()
	if len(got) != 2 || got[1].GetUserId() != jogador.id || got[1].GetRole() != campaignsv1.Role_ROLE_PLAYER ||
		got[1].GetDisplayName() != "Pensantus" {
		t.Errorf("ListMembers() = %v, want the master and then Pensantus as a player", got)
	}
}

// MR-003, second criterion: Dado um convite expirado, quando alguém abre o
// link, então vê uma mensagem clara e nada é criado.
func TestMR003_ExpiredInviteGivesAClearErrorAndCreatesNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre := h.newUser("Mestre")
	jogador := h.newUser("Jogador")
	campaign := mestre.createCampaign(t, "Mirathel")
	invite, token := mestre.createInvite(t, campaign.GetId(), 0, 0)

	h.clock.Advance(7 * 24 * time.Hour)
	_, err := jogador.accept(t, token)
	if got := inviteUnusableState(t, err); got != campaignsv1.InviteState_INVITE_STATE_EXPIRED {
		t.Errorf("AcceptInvite() state = %v, want EXPIRED", got)
	}
	if msg := connectMessage(err); !strings.Contains(msg, "expired") || !strings.Contains(msg, "ask the master") {
		t.Errorf("message %q does not say the invite expired and what to do", msg)
	}
	if roles := h.memberRoles(campaign.GetId()); len(roles) != 1 {
		t.Errorf("members = %v, want only the master", roles)
	}
	if n := h.useCount(invite.GetId()); n != 0 {
		t.Errorf("use_count = %d, want 0", n)
	}
}
