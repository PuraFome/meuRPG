package characters

import (
	"sync"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// Acceptance tests for MR-024 and RN-15 (invites with approval), one per
// criterion in docs/product/stories.md. The invite's flag and the
// pending membership themselves are tested in package campaigns
// (approval_test.go there).

// MR-024: Dado um convite que exige aprovação, quando o jogador o aceita,
// então cria o personagem, que nasce pendente de aprovação e, enquanto
// espera, ele só vê o nome da campanha e o próprio personagem, que pode
// editar.
func TestMR024_PendingPlayerCreatesTheirCharacterAndWaits(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, jogadora := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(mestre, "Mirathel")
	h.joinPending(mestre, campaign, jogadora)

	// The editor's catalog, then the character, which starts pending.
	if _, err := jogadora.content.ListContent(t.Context(), connect.NewRequest(&rulesv1.ListContentRequest{CampaignId: campaign})); err != nil {
		t.Fatalf("pending member's ListContent() error = %v", err)
	}
	created := jogadora.createPensantus(t, campaign)
	if created.GetState() != charactersv1.CharacterState_CHARACTER_STATE_PENDING || !created.GetCanEdit() || !created.GetCanEditStory() ||
		created.GetCanApprove() || created.GetCanMarkDead() || created.GetPlayerUserId() != jogadora.id {
		t.Fatalf("CreateCharacter() = %v, want her pending character, editable, with no master buttons", created)
	}

	// She edits the sheet and the story while she waits.
	edited, err := jogadora.update(t, created, "Pensantus, o Paciente", pensantusSheet())
	if err != nil {
		t.Fatalf("pending member's UpdateCharacter() error = %v", err)
	}
	if _, err := jogadora.updateStory(t, edited, &charactersv1.CharacterStory{Backstory: "Espera à porta da guilda."}); err != nil {
		t.Fatalf("pending member's UpdateCharacterStory() error = %v", err)
	}
	if got := jogadora.get(t, campaign, created.GetId()); got.GetName() != "Pensantus, o Paciente" || got.GetStory().GetBackstory() != "Espera à porta da guilda." {
		t.Errorf("pending member's GetCharacter() = %v, want her edits", got)
	}
	if list := jogadora.list(t, campaign); len(list) != 1 || list[0].GetId() != created.GetId() {
		t.Errorf("pending member's ListCharacters() = %v, want only her pending character", list)
	}

	// One character only (RN-03 counts a pending one as living).
	_, err = jogadora.api.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Outro", Sheet: pensantusSheet(),
	}))
	if d := blocked(t, "a second pending character", err); d.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_LIVING_CHARACTER_EXISTS {
		t.Errorf("CharacterBlocked = %v, want LIVING_CHARACTER_EXISTS", d)
	}

	// The master sees it waiting, with the approve buttons.
	list := mestre.list(t, campaign)
	if len(list) != 1 || list[0].GetState() != charactersv1.CharacterState_CHARACTER_STATE_PENDING || list[0].GetPlayerDisplayName() != "Jogadora" {
		t.Errorf("master's ListCharacters() = %v, want Jogadora's pending character", list)
	}
	if got := mestre.get(t, campaign, created.GetId()); !got.GetCanApprove() || got.GetCanMarkDead() {
		t.Errorf("master's GetCharacter() can_approve %v, can_mark_dead %v; want true, false", got.GetCanApprove(), got.GetCanMarkDead())
	}
}

// MR-024: Dado um personagem pendente, quando o mestre o aprova, então o
// personagem vira rascunho e o jogador passa a ser jogador da campanha.
func TestMR024_MasterApprovesAndThePlayerJoins(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, jogadora := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(mestre, "Mirathel")
	h.joinPending(mestre, campaign, jogadora)
	pending := jogadora.createPensantus(t, campaign)

	approved := mestre.approve(t, pending)
	if approved.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT || approved.GetCanApprove() || !approved.GetCanMarkDead() ||
		approved.GetRevision() != pending.GetRevision() {
		t.Fatalf("ApproveCharacter() = %v, want a draft, same revision, no longer approvable", approved)
	}
	if got := h.memberStatus(campaign, jogadora.id); got != "active" {
		t.Errorf("membership after approval = %q, want active", got)
	}

	// She is a player now: she sees the whole campaign and its members.
	res, err := jogadora.campaigns.GetCampaign(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: campaign}))
	if err != nil || res.Msg.GetCampaign().GetAwaitingApproval() || res.Msg.GetCampaign().GetXpMode() != campaignsv1.XpMode_XP_MODE_MILESTONES {
		t.Errorf("approved player's GetCampaign() = %v, %v; want the full view", res, err)
	}
	members, err := mestre.campaigns.ListMembers(t.Context(), connect.NewRequest(&campaignsv1.ListMembersRequest{CampaignId: campaign}))
	if err != nil || len(members.Msg.GetMembers()) != 2 {
		t.Errorf("ListMembers() = %v, %v; want the master and Jogadora", members, err)
	}

	// Her character is an ordinary draft: it locks at the next session.
	if n := h.lockSheets(campaign); n != 1 {
		t.Errorf("LockSheets() locked %d sheets, want 1", n)
	}
	if got := jogadora.get(t, campaign, pending.GetId()); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_LOCKED {
		t.Errorf("state after the session started = %v, want LOCKED", got.GetState())
	}

	// Approving again changes nothing (idempotent); so does approving a
	// character that never waited.
	if again := mestre.approve(t, pending); again.GetState() != charactersv1.CharacterState_CHARACTER_STATE_LOCKED {
		t.Errorf("ApproveCharacter() again = %v, want the character as it is", again.GetState())
	}
	jogador := h.newUser("Jogador")
	h.join(mestre, campaign, jogador)
	draft := jogador.createPensantus(t, campaign)
	if got := mestre.approve(t, draft); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT {
		t.Errorf("ApproveCharacter(an ordinary draft) = %v, want it unchanged", got.GetState())
	}
}

// MR-024: Dado um personagem pendente, quando o mestre o recusa, então o
// personagem é apagado, o jogador não entra na campanha e precisa de um
// convite novo para tentar de novo.
func TestMR024_MasterRejectsAndThePlayerStaysOut(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, jogadora := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(mestre, "Mirathel")
	inv, err := mestre.campaigns.CreateInvite(t.Context(), connect.NewRequest(&campaignsv1.CreateInviteRequest{CampaignId: campaign, RequiresApproval: true}))
	if err != nil {
		t.Fatalf("CreateInvite() error = %v", err)
	}
	accept := func(token string) error {
		_, err := jogadora.campaigns.AcceptInvite(t.Context(), connect.NewRequest(&campaignsv1.AcceptInviteRequest{Token: token}))
		return err
	}
	if err := accept(inv.Msg.GetToken()); err != nil {
		t.Fatalf("AcceptInvite() error = %v", err)
	}
	pending := jogadora.createPensantus(t, campaign)
	if _, err := mestre.api.UpdateMasterNotes(t.Context(), connect.NewRequest(&charactersv1.UpdateMasterNotesRequest{
		CampaignId: campaign, CharacterId: pending.GetId(), Notes: "Não combina com a mesa.",
	})); err != nil {
		t.Fatalf("UpdateMasterNotes() error = %v", err)
	}

	if err := mestre.reject(t, pending); err != nil {
		t.Fatalf("RejectCharacter() error = %v", err)
	}

	// The character, its story and the master's notes are gone, and so is
	// her pending membership.
	for query, what := range map[string]string{
		"SELECT count(*) FROM characters WHERE id = $1":                       "the character",
		"SELECT count(*) FROM character_master_notes WHERE character_id = $1": "the master's notes",
	} {
		var n int
		if err := h.pool.QueryRow(t.Context(), query, pending.GetId()).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s after the rejection: %d rows, %v; want none", what, n, err)
		}
	}
	if got := h.memberStatus(campaign, jogadora.id); got != "" {
		t.Errorf("membership after the rejection = %q, want none", got)
	}
	_, err = jogadora.campaigns.GetCampaign(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: campaign}))
	wantCode(t, "rejected player's GetCampaign()", err, connect.CodeNotFound)
	_, err = jogadora.api.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: campaign, CharacterId: pending.GetId()}))
	wantCode(t, "rejected player's GetCharacter()", err, connect.CodeNotFound)
	_, err = mestre.api.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: campaign, CharacterId: pending.GetId()}))
	wantCode(t, "master's GetCharacter(rejected)", err, connect.CodeNotFound)
	wantCode(t, "RejectCharacter() again", mestre.reject(t, pending), connect.CodeNotFound)

	// The old link was used up: she needs a new invite.
	wantCode(t, "AcceptInvite(the used link)", accept(inv.Msg.GetToken()), connect.CodeFailedPrecondition)
	h.joinPending(mestre, campaign, jogadora)
	if got := h.memberStatus(campaign, jogadora.id); got != "pending" {
		t.Errorf("membership after a new invite = %q, want pending", got)
	}
}

// MR-024: Dado que sou jogador, ou jogador pendente, quando tento aprovar
// ou recusar um personagem, então o servidor recusa.
func TestMR024_OnlyTheMasterApprovesOrRejects(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, jogador, jogadora := h.newUser("Mestre"), h.newUser("Jogador"), h.newUser("Jogadora")
	campaign := h.newCampaign(mestre, "Mirathel", jogador)
	h.joinPending(mestre, campaign, jogadora)
	pending := jogadora.createPensantus(t, campaign)

	for _, tt := range []struct {
		who  string
		u    *user
		want connect.Code
	}{
		{"a player", jogador, connect.CodePermissionDenied},
		{"the pending member herself", jogadora, connect.CodeNotFound},
	} {
		_, err := tt.u.api.ApproveCharacter(t.Context(), connect.NewRequest(&charactersv1.ApproveCharacterRequest{CampaignId: campaign, CharacterId: pending.GetId()}))
		wantCode(t, tt.who+"'s ApproveCharacter()", err, tt.want)
		wantCode(t, tt.who+"'s RejectCharacter()", tt.u.reject(t, pending), tt.want)
	}
	if got := mestre.get(t, campaign, pending.GetId()); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_PENDING {
		t.Errorf("state = %v, want still PENDING", got.GetState())
	}
}

// RN-15: a pending character is not part of the campaign yet: a game
// session does not lock it, it cannot die, and only a pending one can be
// rejected. NPCs never wait for approval.
func TestRN15_PendingCharacterIsNotPartOfTheCampaignYet(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, jogador, jogadora := h.newUser("Mestre"), h.newUser("Jogador"), h.newUser("Jogadora")
	campaign := h.newCampaign(mestre, "Mirathel", jogador)
	h.joinPending(mestre, campaign, jogadora)
	draft := jogador.createPensantus(t, campaign)
	pending := jogadora.createPensantus(t, campaign)
	npc := mestre.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_STORY, "Taverneiro", basicSheet())

	if n := h.lockSheets(campaign); n != 1 {
		t.Errorf("LockSheets() locked %d sheets, want only the draft", n)
	}
	if got := jogadora.get(t, campaign, pending.GetId()); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_PENDING || !got.GetCanEdit() {
		t.Errorf("pending character after a session started = %v, want still pending and editable", got)
	}

	_, err := mestre.api.MarkCharacterDead(t.Context(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: campaign, CharacterId: pending.GetId()}))
	if d := blocked(t, "MarkCharacterDead(pending)", err); d.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_AWAITING_APPROVAL {
		t.Errorf("CharacterBlocked = %v, want AWAITING_APPROVAL", d)
	}
	if d := blocked(t, "RejectCharacter(draft)", mestre.reject(t, draft)); d.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_NOT_PENDING {
		t.Errorf("CharacterBlocked = %v, want NOT_PENDING", d)
	}
	wantCode(t, "RejectCharacter(NPC)", mestre.reject(t, npc), connect.CodeInvalidArgument)
	_, err = mestre.api.ApproveCharacter(t.Context(), connect.NewRequest(&charactersv1.ApproveCharacterRequest{CampaignId: campaign, CharacterId: npc.GetId()}))
	wantCode(t, "ApproveCharacter(NPC)", err, connect.CodeInvalidArgument)

	// A pending member sees only their pending character: not the other
	// player's, not the NPC.
	for _, id := range []string{draft.GetId(), npc.GetId()} {
		_, err := jogadora.api.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: campaign, CharacterId: id}))
		wantCode(t, "pending member's GetCharacter(someone else's)", err, connect.CodeNotFound)
	}
}

// RN-15: an approval and a rejection that race settle the character once:
// either it is approved and its player is a member, or it is gone and so is
// the pending membership. Never half of each.
func TestRN15_ApproveAndRejectRace(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 4) // the racers must overlap: one connection would run them one by one
	h := newHarness(t)
	mestre := h.newUser("Mestre")
	campaign := h.newCampaign(mestre, "Mirathel")

	for round := range 5 {
		jogadora := h.newUser("Jogadora")
		h.joinPending(mestre, campaign, jogadora)
		pending := jogadora.createPensantus(t, campaign)

		var wg sync.WaitGroup
		var approveErr, rejectErr error
		start := dbtest.NewBarrier(2)
		wg.Go(func() {
			start.Wait()
			_, approveErr = mestre.api.ApproveCharacter(t.Context(), connect.NewRequest(&charactersv1.ApproveCharacterRequest{CampaignId: campaign, CharacterId: pending.GetId()}))
		})
		wg.Go(func() {
			start.Wait()
			rejectErr = mestre.reject(t, pending)
		})
		wg.Wait()

		var exists int
		if err := h.pool.QueryRow(t.Context(), "SELECT count(*) FROM characters WHERE id = $1", pending.GetId()).Scan(&exists); err != nil {
			t.Fatalf("count characters: %v", err)
		}
		status := h.memberStatus(campaign, jogadora.id)
		switch exists {
		case 1:
			// Approved first: the rejection found nothing pending.
			if approveErr != nil || connect.CodeOf(rejectErr) != connect.CodeFailedPrecondition || status != "active" {
				t.Errorf("round %d, approved: approve %v, reject %v, membership %q; want ok, failed_precondition, active", round, approveErr, rejectErr, status)
			}
		default:
			// Rejected first: the approval found nothing.
			if rejectErr != nil || connect.CodeOf(approveErr) != connect.CodeNotFound || status != "" {
				t.Errorf("round %d, rejected: reject %v, approve %v, membership %q; want ok, not_found, none", round, rejectErr, approveErr, status)
			}
		}
	}
}
