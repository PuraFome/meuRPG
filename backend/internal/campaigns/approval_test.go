package campaigns

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

// Invites with approval (RN-15, MR-024), the campaigns half: the invite's
// flag and the pending membership. Creating, approving and rejecting the
// character are package characters' tests.

// MR-024: Dado que sou mestre de "Mirathel", quando gero um convite, então
// escolho se ele exige a minha aprovação; sem escolher, não exige.
func TestMR024_MasterChoosesWhetherAnInviteRequiresApproval(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre := h.newUser("Mestre")
	campaign := mestre.createCampaign(t, "Mirathel")

	plain, _ := mestre.createInvite(t, campaign.GetId(), 0, 0)
	withApproval, _ := mestre.createApprovalInvite(t, campaign.GetId())
	if plain.GetRequiresApproval() || !withApproval.GetRequiresApproval() {
		t.Errorf("requires_approval = %v (default) and %v (asked for), want false and true", plain.GetRequiresApproval(), withApproval.GetRequiresApproval())
	}

	res, err := mestre.api.ListInvites(t.Context(), connect.NewRequest(&campaignsv1.ListInvitesRequest{CampaignId: campaign.GetId()}))
	if err != nil {
		t.Fatalf("ListInvites() error = %v", err)
	}
	listed := map[string]bool{}
	for _, inv := range res.Msg.GetInvites() {
		listed[inv.GetId()] = inv.GetRequiresApproval()
	}
	if len(listed) != 2 || listed[plain.GetId()] || !listed[withApproval.GetId()] {
		t.Errorf("ListInvites() requires_approval = %v, want the plain invite false and the other true", listed)
	}
}

// RN-15: Dado um convite que exige aprovação, quando o jogador o aceita,
// então ele fica pendente: vê só o nome da campanha, e para todo o resto
// não é membro.
func TestRN15_AcceptingAnInviteWithApprovalMakesAPendingMember(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, jogadora := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := mestre.createCampaign(t, "Mirathel")
	id := campaign.GetId()
	invite, token := mestre.createApprovalInvite(t, id)

	accepted, err := jogadora.accept(t, token)
	if err != nil {
		t.Fatalf("AcceptInvite() error = %v", err)
	}
	wantPendingView(t, "AcceptInvite()", accepted.GetCampaign(), id)
	if accepted.GetAlreadyMember() {
		t.Error("AcceptInvite() already_member = true, want false the first time")
	}
	if got := h.memberStatus(id, jogadora.id); got != "pending" {
		t.Errorf("status = %q, want pending", got)
	}
	if n := h.useCount(invite.GetId()); n != 1 {
		t.Errorf("use_count = %d, want 1", n)
	}

	// She sees the campaign's name, and it is in her list, waiting.
	got, err := jogadora.api.GetCampaign(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: id}))
	if err != nil {
		t.Fatalf("pending member's GetCampaign() error = %v", err)
	}
	wantPendingView(t, "GetCampaign()", got.Msg.GetCampaign(), id)
	mine, err := jogadora.api.ListMyCampaigns(t.Context(), connect.NewRequest(&campaignsv1.ListMyCampaignsRequest{}))
	if err != nil || len(mine.Msg.GetCampaigns()) != 1 {
		t.Fatalf("pending member's ListMyCampaigns() = %v, %v; want Mirathel", mine, err)
	}
	wantPendingView(t, "ListMyCampaigns()", mine.Msg.GetCampaigns()[0], id)

	// Everything else: not a member.
	_, err = jogadora.api.ListMembers(t.Context(), connect.NewRequest(&campaignsv1.ListMembersRequest{CampaignId: id}))
	wantCode(t, "pending member's ListMembers()", err, connect.CodeNotFound)
	_, err = jogadora.api.ListInvites(t.Context(), connect.NewRequest(&campaignsv1.ListInvitesRequest{CampaignId: id}))
	wantCode(t, "pending member's ListInvites()", err, connect.CodeNotFound)

	// The master does not list her as a member yet.
	members, err := mestre.api.ListMembers(t.Context(), connect.NewRequest(&campaignsv1.ListMembersRequest{CampaignId: id}))
	if err != nil {
		t.Fatalf("ListMembers() error = %v", err)
	}
	if n := len(members.Msg.GetMembers()); n != 1 {
		t.Errorf("master's ListMembers() = %d members, want only the master", n)
	}

	// Accepting the same invite again changes nothing: she stays pending,
	// and no use is spent. (An ordinary invite promotes her instead, Q25:
	// TestQ25_PlainInvitePromotesPendingMember.)
	again, err := jogadora.accept(t, token)
	if err != nil {
		t.Fatalf("AcceptInvite() again error = %v", err)
	}
	if !again.GetAlreadyMember() {
		t.Error("AcceptInvite() again already_member = false, want true")
	}
	wantPendingView(t, "AcceptInvite() again", again.GetCampaign(), id)
	if got := h.memberStatus(id, jogadora.id); got != "pending" {
		t.Errorf("status after accepting again = %q, want pending", got)
	}
	if n := h.useCount(invite.GetId()); n != 1 {
		t.Errorf("use_count after accepting again = %d, want 1", n)
	}
}

// RN-15: an invite without approval works as it always did.
func TestRN15_InviteWithoutApprovalMakesAPlayerAtOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, jogador := h.newUser("Mestre"), h.newUser("Jogador")
	campaign := mestre.createCampaign(t, "Mirathel")
	_, token := mestre.createInvite(t, campaign.GetId(), 0, 0)

	res, err := jogador.accept(t, token)
	if err != nil {
		t.Fatalf("AcceptInvite() error = %v", err)
	}
	if c := res.GetCampaign(); c.GetAwaitingApproval() || c.GetXpMode() != campaignsv1.XpMode_XP_MODE_MILESTONES || c.GetMyRole() != campaignsv1.Role_ROLE_PLAYER {
		t.Errorf("AcceptInvite() campaign = %v, want the full view, as a player", c)
	}
	if got := h.memberStatus(campaign.GetId(), jogador.id); got != "active" {
		t.Errorf("status = %q, want active", got)
	}
}

// TestPendingMemberSettlement: the two methods package characters calls
// inside its approval transaction touch only a pending membership.
func TestPendingMemberSettlement(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	svc, err := New(Config{Pool: h.pool, Profiles: h.users})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	mestre, ativo, pendente, recusado := h.newUser("Mestre"), h.newUser("Ativo"), h.newUser("Pendente"), h.newUser("Recusado")
	id := mestre.createCampaign(t, "Mirathel").GetId()
	_, token := mestre.createInvite(t, id, MaxInviteUses, 0)
	ativo.join(t, token)
	approval, err := mestre.api.CreateInvite(t.Context(), connect.NewRequest(&campaignsv1.CreateInviteRequest{
		CampaignId: id, MaxUses: MaxInviteUses, RequiresApproval: true,
	}))
	if err != nil {
		t.Fatalf("CreateInvite() error = %v", err)
	}
	pendente.join(t, approval.Msg.GetToken())
	recusado.join(t, approval.Msg.GetToken())

	inTx := func(f func(tx pgx.Tx) error) {
		t.Helper()
		if err := db.InTx(t.Context(), h.pool, f); err != nil {
			t.Fatalf("transaction error = %v", err)
		}
	}
	inTx(func(tx pgx.Tx) error {
		for _, u := range []*user{mestre, ativo, pendente} {
			if err := svc.ActivatePendingMember(t.Context(), tx, id, u.id); err != nil {
				return err
			}
		}
		for _, u := range []*user{mestre, ativo, recusado} {
			if err := svc.DeletePendingMember(t.Context(), tx, id, u.id); err != nil {
				return err
			}
		}
		return nil
	})

	want := map[string]string{mestre.id: "active", ativo.id: "active", pendente.id: "active", recusado.id: ""}
	for userID, status := range want {
		if got := h.memberStatus(id, userID); got != status {
			t.Errorf("status of %s = %q, want %q", userID, got, status)
		}
	}
	// Approved, she is a member: GetCampaign shows her everything.
	got, err := pendente.api.GetCampaign(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: id}))
	if err != nil || got.Msg.GetCampaign().GetAwaitingApproval() || got.Msg.GetCampaign().GetXpMode() == campaignsv1.XpMode_XP_MODE_UNSPECIFIED {
		t.Errorf("approved member's GetCampaign() = %v, %v; want the full view", got, err)
	}
	// Rejected, he is out: the same answer as for a stranger.
	_, err = recusado.api.GetCampaign(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: id}))
	wantCode(t, "rejected member's GetCampaign()", err, connect.CodeNotFound)
}

// MR-024 through sign-in: a signed-out player opens an invite that requires
// approval, signs in, and lands straight on creating their character, as a
// pending member. Opening the link again takes them to the campaign's page.
func TestSignInWithAnApprovalInviteGoesToCreateTheCharacter(t *testing.T) {
	t.Parallel()
	st := newSignInStack(t)
	master, campaign, _, _ := st.masterWithInvite(0)
	res, err := st.campaigns(master).CreateInvite(t.Context(), connect.NewRequest(&campaignsv1.CreateInviteRequest{
		CampaignId: campaign.GetId(), RequiresApproval: true,
	}))
	if err != nil {
		t.Fatalf("CreateInvite() error = %v", err)
	}
	token := res.Msg.GetToken()

	location, session := st.signInWithInvite(token)
	if want := "/campaigns/" + campaign.GetId() + "/characters/new"; location != want {
		t.Errorf("callback went to %q, want %q; logs: %s", location, want, st.logs)
	}
	player := st.userID(session)
	var status string
	if err := st.pool.QueryRow(t.Context(), "SELECT status FROM campaign_members WHERE campaign_id = $1 AND user_id = $2", campaign.GetId(), player).Scan(&status); err != nil || status != "pending" {
		t.Errorf("status = %q, %v; want pending", status, err)
	}

	if location, _ := st.continueWithInvite(token); location != "/campaigns/"+campaign.GetId() {
		t.Errorf("second sign-in went to %q, want the campaign's page", location)
	}
	st.assertNoSecretsInLogs(token)
}

// wantPendingView fails the test unless c is what a pending member sees:
// the id, the name, the XP mode and ROLE_PLAYER, with awaiting_approval, and
// nothing else (not the dice mode, not when the campaign was made).
func wantPendingView(t *testing.T, call string, c *campaignsv1.Campaign, id string) {
	t.Helper()
	if c.GetId() != id || c.GetName() != "Mirathel" || c.GetMyRole() != campaignsv1.Role_ROLE_PLAYER || !c.GetAwaitingApproval() ||
		c.GetXpMode() == campaignsv1.XpMode_XP_MODE_UNSPECIFIED || c.GetDiceMode() != campaignsv1.DiceMode_DICE_MODE_UNSPECIFIED || c.GetCreatedAt() != nil {
		t.Errorf("%s campaign = %v, want only the id, the name, the XP mode and ROLE_PLAYER, awaiting approval", call, c)
	}
}

// A pending membership past its 30-day deadline is no membership, even while
// the TTL job has not deleted the row: approving a character cannot bring it
// back. Before the deadline, the same call activates it.
func TestActivatePendingMemberLeavesAnExpiredMembershipAlone(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, atrasada, emDia := h.newUser("Mestre"), h.newUser("Atrasada"), h.newUser("EmDia")
	id := mestre.createCampaign(t, "Mirathel").GetId()
	invite := mestre.createManyApprovalInvite(t, id)
	atrasada.join(t, invite)
	emDia.join(t, invite)
	past := h.clock.Now().Add(-time.Hour)
	if _, err := h.pool.Exec(t.Context(), "UPDATE campaign_members SET pending_expires_at = $3 WHERE campaign_id = $1 AND user_id = $2", id, atrasada.id, past); err != nil {
		t.Fatal(err)
	}

	if err := db.InTx(t.Context(), h.pool, func(tx pgx.Tx) error {
		for _, u := range []*user{atrasada, emDia} {
			if err := h.service.ActivatePendingMember(t.Context(), tx, id, u.id); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("transaction error = %v", err)
	}
	if got := h.memberStatus(id, atrasada.id); got != "pending" {
		t.Errorf("status of the expired member = %q, want pending", got)
	}
	if got := h.memberStatus(id, emDia.id); got != "active" {
		t.Errorf("status of the member within the deadline = %q, want active (positive control)", got)
	}
}
