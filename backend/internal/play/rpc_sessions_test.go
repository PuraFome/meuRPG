package play

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Characterization of StartGameSession and EndGameSession (rpc.go): the
// idempotency key, the locked sheets, the replay, and what ending a session
// does with the image the master left with the players.

func (u *user) startKeyed(t *testing.T, campaignID, key string) (*playv1.StartGameSessionResponse, error) {
	t.Helper()
	res, err := u.play.StartGameSession(t.Context(), connect.NewRequest(&playv1.StartGameSessionRequest{CampaignId: campaignID, IdempotencyKey: key}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func TestStartGameSession_TheIdempotencyKey(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	other := h.newCampaign(master, "Outra")
	player.createCharacter(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")

	_, err := master.startKeyed(t, campaign, strings.Repeat("k", 65))
	wantRefusal(t, "a key of 65 characters", err, connect.CodeInvalidArgument, "idempotency_key must be 1 to 64 characters")
	_, err = master.startKeyed(t, campaign, "a\nb")
	wantRefusal(t, "a key with a control character", err, connect.CodeInvalidArgument, "idempotency_key must be 1 to 64 characters")
	_, err = player.startKeyed(t, campaign, "k1")
	wantRefusal(t, "a player", err, connect.CodePermissionDenied, "")

	first, err := master.startKeyed(t, campaign, "  k1  ")
	if err != nil || first.GetLockedSheetCount() != 1 || first.GetGameSession().GetSessionNumber() != 1 {
		t.Fatalf("first start = %v, %v; want session 1 with 1 locked sheet", first, err)
	}
	// A retry while open: the same session, no count of its own.
	again, err := master.startKeyed(t, campaign, "k1")
	if err != nil || again.GetGameSession().GetId() != first.GetGameSession().GetId() || again.GetLockedSheetCount() != 0 {
		t.Fatalf("retry = %v, %v; want the first session and no locked count", again, err)
	}
	// A retry after the session ended still returns the first session, and starts no other.
	master.end(t, first.GetGameSession())
	late, err := master.startKeyed(t, campaign, "k1")
	if err != nil || late.GetGameSession().GetId() != first.GetGameSession().GetId() || late.GetGameSession().GetEndedAt() == nil {
		t.Fatalf("late retry = %v, %v; want the first session, ended", late, err)
	}
	// Another key starts the next one; the same key in another campaign is its own.
	second, err := master.startKeyed(t, campaign, "k2")
	if err != nil || second.GetGameSession().GetSessionNumber() != 2 {
		t.Fatalf("second start = %v, %v; want session 2", second, err)
	}
	if res, err := master.startKeyed(t, other, "k1"); err != nil || res.GetGameSession().GetCampaignId() != other {
		t.Fatalf("start in another campaign with the same key = %v, %v", res, err)
	}
	// An open session refuses a new key, with the typed reason.
	_, err = master.startKeyed(t, campaign, "k3")
	wantBlocked(t, "start with a session open", err, playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_SESSION_ALREADY_OPEN)
}

func TestEndGameSession_TheKeptImageLeavesWithThePlayers(t *testing.T) {
	t.Parallel()
	st := newShownTable(t)
	a, b := st.image(t, st.campaign), st.image(t, st.campaign)
	session := st.master.start(t, st.campaign).GetGameSession()
	if _, err := st.show(t, a, true); err != nil {
		t.Fatalf("show A error = %v", err)
	}

	// The maps module fails to leave the image: the session stays open.
	boom := errors.New("boom")
	st.maps.leave = func(context.Context, pgx.Tx, string, string, time.Time) error { return boom }
	_, err := st.master.play.EndGameSession(t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: st.campaign, GameSessionId: session.GetId()}))
	wantRefusal(t, "leave fails", err, connect.CodeInternal, "")
	if id, _ := st.dbShown(t); id != a {
		t.Errorf("the open session shows %q after the failed end, want A", id)
	}
	st.maps.leave = nil

	ended := st.master.end(t, session)
	if ended.GetEndedAt() == nil {
		t.Fatal("the session did not end")
	}
	if got := st.left(t); len(got) != 1 || got[0] != a {
		t.Errorf("left images = %v, want [A]: the kept image outlives the session", got)
	}

	// An image that is shown but not kept does not leave.
	st.master.start(t, st.campaign)
	if _, err := st.show(t, b, false); err != nil {
		t.Fatalf("show B error = %v", err)
	}
	next, err := st.master.play.ListGameSessions(t.Context(), connect.NewRequest(&playv1.ListGameSessionsRequest{CampaignId: st.campaign}))
	if err != nil {
		t.Fatal(err)
	}
	st.master.end(t, next.Msg.GetGameSessions()[0])
	if got := st.left(t); len(got) != 1 {
		t.Errorf("left images = %v, want only A", got)
	}
}
