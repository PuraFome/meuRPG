package characters

import (
	"testing"
	"time"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// Q24 and Q25 (RN-15, MR-024), the characters half: a pending member's
// character and the invite, and the 30-day deadline. The rest is in package
// campaigns' pending_test.go.

// pendingExpiresAt reads campaign_members.pending_expires_at directly; nil
// when it is NULL.
func (h *harness) pendingExpiresAt(campaignID, userID string) *time.Time {
	h.t.Helper()
	var at *time.Time
	if err := h.pool.QueryRow(h.t.Context(), "SELECT pending_expires_at FROM campaign_members WHERE campaign_id = $1 AND user_id = $2", campaignID, userID).Scan(&at); err != nil {
		h.t.Fatalf("read pending_expires_at: %v", err)
	}
	return at
}

// Q25: Dado um membro pendente com personagem esperando aprovação, quando
// ele aceita um convite sem aprovação, então ele vira jogador e o
// personagem é aprovado, com o mesmo efeito de o mestre aprovar.
func TestQ25_PlainInvitePromotesPendingMember(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, porConvite, porMestre := h.newUser("Mestre"), h.newUser("Por convite"), h.newUser("Pelo mestre")
	campaign := h.newCampaign(mestre, "Mirathel")
	h.joinPending(mestre, campaign, porConvite, porMestre)
	waiting := porConvite.createPensantus(t, campaign)
	compared := porMestre.createPensantus(t, campaign)

	// The same state, reached the two ways.
	h.join(mestre, campaign, porConvite)                 // an ordinary invite
	promoted := mestre.get(t, campaign, waiting.GetId()) // the master's view, like approve's
	approved := mestre.approve(t, compared)

	if promoted.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT || promoted.GetRevision() != waiting.GetRevision() {
		t.Errorf("character after the promotion = %v, want a draft with the same revision", promoted)
	}
	if promoted.GetState() != approved.GetState() || promoted.GetRevision() != approved.GetRevision() || promoted.GetCanMarkDead() != approved.GetCanMarkDead() {
		t.Errorf("promoted character %v differs from the one the master approved %v", promoted, approved)
	}
	if got := h.memberStatus(campaign, porConvite.id); got != "active" {
		t.Errorf("membership after the invite = %q, want active", got)
	}
	if got := h.pendingExpiresAt(campaign, porConvite.id); got != nil {
		t.Errorf("pending_expires_at = %v, want NULL", got)
	}
	// Her character is an ordinary draft: it locks at the next session.
	if n := h.lockSheets(campaign); n != 2 {
		t.Errorf("LockSheets() locked %d sheets, want 2", n)
	}
	// And she is a player: the full campaign and a member list.
	res, err := porConvite.campaigns.GetCampaign(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: campaign}))
	if err != nil || res.Msg.GetCampaign().GetAwaitingApproval() {
		t.Errorf("promoted player's GetCampaign() = %v, %v; want the full view", res, err)
	}
	// A pending member who has no character yet is promoted too, with
	// nothing to approve.
	semPersonagem := h.newUser("Sem personagem")
	h.joinPending(mestre, campaign, semPersonagem)
	h.join(mestre, campaign, semPersonagem)
	if got := h.memberStatus(campaign, semPersonagem.id); got != "active" {
		t.Errorf("membership of someone without a character = %q, want active", got)
	}
}

// Q24: Dado um membro pendente, quando ele cria o personagem, então o prazo
// de 30 dias para quem não tem personagem some, ele deixa a lista do mestre
// de "pendentes sem personagem", e o mestre não o remove por ali: recusa o
// personagem.
func TestQ24_CreatingTheCharacterClearsTheDeadline(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, jogadora := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(mestre, "Mirathel")
	h.joinPending(mestre, campaign, jogadora)

	if got := h.pendingExpiresAt(campaign, jogadora.id); got == nil {
		t.Fatal("a new pending member has no deadline")
	}
	list, err := mestre.campaigns.ListPendingMembers(t.Context(), connect.NewRequest(&campaignsv1.ListPendingMembersRequest{CampaignId: campaign}))
	if err != nil || len(list.Msg.GetMembers()) != 1 {
		t.Fatalf("ListPendingMembers() = %v, %v; want Jogadora", list, err)
	}

	pending := jogadora.createPensantus(t, campaign)
	if got := h.pendingExpiresAt(campaign, jogadora.id); got != nil {
		t.Errorf("pending_expires_at after creating the character = %v, want NULL", got)
	}
	list, err = mestre.campaigns.ListPendingMembers(t.Context(), connect.NewRequest(&campaignsv1.ListPendingMembersRequest{CampaignId: campaign}))
	if err != nil || len(list.Msg.GetMembers()) != 0 {
		t.Errorf("ListPendingMembers() after the character = %v, %v; want nobody", list, err)
	}

	// Her character waits for the master: he rejects it, not removes her.
	_, err = mestre.campaigns.RemovePendingMember(t.Context(), connect.NewRequest(&campaignsv1.RemovePendingMemberRequest{CampaignId: campaign, UserId: jogadora.id}))
	wantCode(t, "RemovePendingMember(a pending member with a character)", err, connect.CodeFailedPrecondition)
	if got := h.memberStatus(campaign, jogadora.id); got != "pending" {
		t.Errorf("membership after the refused removal = %q, want pending", got)
	}
	if err := mestre.reject(t, pending); err != nil {
		t.Fatalf("RejectCharacter() error = %v", err)
	}
	if got := h.memberStatus(campaign, jogadora.id); got != "" {
		t.Errorf("membership after the rejection = %q, want none", got)
	}
}
