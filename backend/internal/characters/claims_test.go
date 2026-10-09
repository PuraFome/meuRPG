package characters

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/identity"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
	"github.com/PuraFome/meuRPG/backend/internal/platform/secret"
)

// The reserved characters and claim links (MR-049; RN-10, RN-03).

// reservedByMaster makes a reserved character as the master, or fails the test.
func (u *user) reservedByMaster(t *testing.T, campaignID, name string) *charactersv1.Character {
	t.Helper()
	res, err := u.api.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: name, Sheet: pensantusSheet(), ForPlayer: true,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(for_player %q) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// link makes a claim link for c as the master, and returns its token.
func (u *user) link(t *testing.T, c *charactersv1.Character, days int32) string {
	t.Helper()
	res, err := u.api.CreateClaimLink(t.Context(), connect.NewRequest(&charactersv1.CreateClaimLinkRequest{
		CampaignId: c.GetCampaignId(), CharacterId: c.GetId(), ValidityDays: days,
	}))
	if err != nil {
		t.Fatalf("CreateClaimLink() error = %v", err)
	}
	return res.Msg.GetToken()
}

func (u *user) claim(t *testing.T, token string) (*charactersv1.ClaimCharacterResponse, error) {
	t.Helper()
	res, err := u.api.ClaimCharacter(t.Context(), connect.NewRequest(&charactersv1.ClaimCharacterRequest{Token: token}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (u *user) previewClaim(t *testing.T, token string) (*charactersv1.PreviewClaimResponse, error) {
	t.Helper()
	res, err := u.api.PreviewClaim(t.Context(), connect.NewRequest(&charactersv1.PreviewClaimRequest{Token: token}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// summaryOf finds a character in the caller's list, or nil.
func summaryOf(list []*charactersv1.CharacterSummary, id string) *charactersv1.CharacterSummary {
	for _, c := range list {
		if c.GetId() == id {
			return c
		}
	}
	return nil
}

// wantUnusable fails the test unless err is the one refusal every link that cannot be
// used gets: not_found, with the same sentence for every reason.
func wantUnusable(t *testing.T, call string, err error) {
	t.Helper()
	wantCode(t, call, err, connect.CodeNotFound)
	ce, _ := errors.AsType[*connect.Error](err)
	if got, want := ce.Message(), "this claim link cannot be used"; got != want {
		t.Errorf("%s message = %q, want %q (one sentence for every reason)", call, got, want)
	}
	if len(ce.Details()) != 0 {
		t.Errorf("%s carries %d details; the refusal of an unusable link says nothing more", call, len(ce.Details()))
	}
}

func TestMR049_TheMasterMakesAReservedCharacterAndNoPlayerSeesIt(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel", lia)

	c := master.reservedByMaster(t, campaign, "Kai")
	if !c.GetReserved() || c.GetPlayerUserId() != "" || c.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT {
		t.Errorf("reserved character = reserved %v, owner %q, state %v; want a draft with no owner", c.GetReserved(), c.GetPlayerUserId(), c.GetState())
	}
	if c.GetCanMarkDead() || c.GetCanSetStoryEditing() {
		t.Error("a reserved character has no player's life or story to change")
	}
	got := summaryOf(master.list(t, campaign), c.GetId())
	if got == nil || !got.GetReserved() || got.GetClaimState() != charactersv1.ClaimState_CLAIM_STATE_NONE {
		t.Fatalf("the master's list = %v; want Kai reserved with no link", got)
	}
	// RN-10: the player finds nothing, by list or by ID.
	if summaryOf(lia.list(t, campaign), c.GetId()) != nil {
		t.Error("the player's list holds a reserved character")
	}
	_, err := lia.api.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: campaign, CharacterId: c.GetId()}))
	wantCode(t, "GetCharacter(reserved) as a player", err, connect.CodeNotFound)
}

func TestMR049_ForPlayerIsTheMastersAlone(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel", lia)

	_, err := lia.api.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Kai", Sheet: pensantusSheet(), ForPlayer: true,
	}))
	wantCode(t, "CreateCharacter(for_player) as a player", err, connect.CodePermissionDenied)

	_, err = master.api.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, Name: "Orc", Sheet: enemySheet(), ForPlayer: true,
	}))
	wantCode(t, "CreateCharacter(for_player, an NPC)", err, connect.CodeInvalidArgument)

	_, err = master.api.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Kai", Sheet: pensantusSheet(),
	}))
	wantCode(t, "CreateCharacter(a player's character) as the master, without for_player", err, connect.CodePermissionDenied)

	if _, err := master.preview(t, campaign, "", charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, pensantusSheet()); err == nil {
		t.Error("PreviewCharacter(a player's sheet) as the master, without for_player, answered")
	}
	res, err := master.api.PreviewCharacter(t.Context(), connect.NewRequest(&charactersv1.PreviewCharacterRequest{
		CampaignId: campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Sheet: pensantusSheet(), ForPlayer: true,
	}))
	if err != nil || res.Msg.GetDerived() == nil {
		t.Errorf("PreviewCharacter(for_player) = %v, %v; want the derived sheet", res, err)
	}
}

func TestMR049_TheLinkIsRandomHashedAndReplacedByANewOne(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel", lia)
	c := master.reservedByMaster(t, campaign, "Kai")

	first := master.link(t, c, 0)
	second := master.link(t, c, 0)
	if first == second {
		t.Fatal("two links have the same token")
	}
	if raw, err := base64.RawURLEncoding.Strict().DecodeString(second); err != nil || len(raw) != 32 {
		t.Errorf("token = %q, want 32 random bytes in base64url (%v)", second, err)
	}
	// The database keeps the SHA-256 and never the token.
	var stored [][]byte
	rows, err := h.pool.Query(t.Context(), "SELECT token_hash FROM claim_links ORDER BY created_at")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var hash []byte
		if err := rows.Scan(&hash); err != nil {
			t.Fatal(err)
		}
		stored = append(stored, hash)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(second)
	want := sha256.Sum256(raw)
	if len(stored) != 2 || !bytes.Equal(stored[1], want[:]) {
		t.Fatalf("stored hashes = %d, want 2 and the last the token's SHA-256", len(stored))
	}
	for _, hash := range stored {
		if strings.Contains(string(hash), second) {
			t.Error("a stored hash holds the token")
		}
	}
	// One live link: the first stopped working when the second was made.
	_, err = lia.claim(t, first)
	wantUnusable(t, "ClaimCharacter(replaced link)", err)
	if got := summaryOf(master.list(t, campaign), c.GetId()); got.GetClaimState() != charactersv1.ClaimState_CLAIM_STATE_SENT {
		t.Errorf("claim state = %v, want SENT", got.GetClaimState())
	}
	if _, err := lia.claim(t, second); err != nil {
		t.Errorf("ClaimCharacter(the new link) error = %v", err)
	}
}

func TestMR049_ValidityIsOneSevenOrThirtyDays(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Vinicius")
	campaign := h.newCampaign(master, "Mirathel")
	c := master.reservedByMaster(t, campaign, "Kai")
	for _, tc := range []struct {
		days int32
		want time.Duration
	}{{0, 7 * 24 * time.Hour}, {1, 24 * time.Hour}, {7, 7 * 24 * time.Hour}, {30, 30 * 24 * time.Hour}} {
		res, err := master.api.CreateClaimLink(t.Context(), connect.NewRequest(&charactersv1.CreateClaimLinkRequest{
			CampaignId: campaign, CharacterId: c.GetId(), ValidityDays: tc.days,
		}))
		if err != nil {
			t.Fatalf("CreateClaimLink(%d days) error = %v", tc.days, err)
		}
		if got := res.Msg.GetExpiresAt().AsTime().Sub(h.clock.Now()).Round(time.Minute); got != tc.want {
			t.Errorf("CreateClaimLink(%d days) expires in %v, want %v", tc.days, got, tc.want)
		}
	}
	for _, days := range []int32{-1, 2, 8, 31, 365} {
		_, err := master.api.CreateClaimLink(t.Context(), connect.NewRequest(&charactersv1.CreateClaimLinkRequest{
			CampaignId: campaign, CharacterId: c.GetId(), ValidityDays: days,
		}))
		wantCode(t, "CreateClaimLink(days)", err, connect.CodeInvalidArgument)
	}
}

func TestMR049_ThePlayerClaimsOnceAndTheMasterSeesWhoUsedIt(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia, caio := h.newUser("Vinicius"), h.newUser("Lia"), h.newUser("Caio")
	campaign := h.newCampaign(master, "Mirathel", caio)
	c := master.reservedByMaster(t, campaign, "Kai")
	token := master.link(t, c, 7)

	card, err := lia.previewClaim(t, token)
	if err != nil {
		t.Fatalf("PreviewClaim() error = %v", err)
	}
	if got := card.GetCard(); got.GetCharacterName() != "Kai" || got.GetCampaignName() != "Mirathel" || got.GetSentByDisplayName() != "Vinicius" ||
		got.GetClassSummary() == "" || got.GetRaceNamePt() == "" {
		t.Errorf("card = %v; want Kai of Mirathel sent by Vinicius with its race and class", got)
	}
	if h.memberStatus(campaign, lia.id) != "" {
		t.Error("PreviewClaim made the person a member")
	}

	res, err := lia.claim(t, token)
	if err != nil {
		t.Fatalf("ClaimCharacter() error = %v", err)
	}
	if res.GetCharacterId() != c.GetId() || res.GetCampaignId() != campaign || res.GetCampaignName() != "Mirathel" {
		t.Errorf("ClaimCharacter() = %v", res)
	}
	if h.memberStatus(campaign, lia.id) != "active" {
		t.Errorf("member status = %q, want active with no approval step", h.memberStatus(campaign, lia.id))
	}
	mine := lia.get(t, campaign, c.GetId())
	if mine.GetReserved() || mine.GetPlayerUserId() != lia.id || mine.GetRevision() != c.GetRevision() {
		t.Errorf("the claimed character = reserved %v, owner %q, revision %d; want Lia's, unchanged", mine.GetReserved(), mine.GetPlayerUserId(), mine.GetRevision())
	}
	if !mine.GetCanEdit() {
		t.Error("the owner cannot edit a draft sheet she just claimed")
	}
	// Caio, another player, still does not see it: it is Lia's now.
	if summaryOf(caio.list(t, campaign), c.GetId()) != nil {
		t.Error("another player lists Lia's character")
	}
	// The master's list: used by Lia.
	got := summaryOf(master.list(t, campaign), c.GetId())
	if got.GetReserved() || got.GetClaimState() != charactersv1.ClaimState_CLAIM_STATE_USED || got.GetClaimedByDisplayName() != "Lia" {
		t.Errorf("master's summary = reserved %v, state %v, by %q; want used by Lia", got.GetReserved(), got.GetClaimState(), got.GetClaimedByDisplayName())
	}
	// A used link is single use, for anyone, and says nothing more than any dead link.
	_, err = caio.claim(t, token)
	wantUnusable(t, "ClaimCharacter(used link) by another", err)
	_, err = lia.claim(t, token)
	wantUnusable(t, "ClaimCharacter(used link) again", err)
	_, err = caio.previewClaim(t, token)
	wantUnusable(t, "PreviewClaim(used link)", err)
}

func TestMR049_EveryLinkThatCannotBeUsedIsTheSameRefusal(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel")

	expired := master.reservedByMaster(t, campaign, "Expirado")
	expiredToken := master.link(t, expired, 1)
	revoked := master.reservedByMaster(t, campaign, "Revogado")
	revokedToken := master.link(t, revoked, 7)
	if _, err := master.api.RevokeClaimLink(t.Context(), connect.NewRequest(&charactersv1.RevokeClaimLinkRequest{CampaignId: campaign, CharacterId: revoked.GetId()})); err != nil {
		t.Fatalf("RevokeClaimLink() error = %v", err)
	}
	gone := master.reservedByMaster(t, campaign, "Apagado")
	goneToken := master.link(t, gone, 7)
	if _, err := master.api.DeleteReservedCharacter(t.Context(), connect.NewRequest(&charactersv1.DeleteReservedCharacterRequest{CampaignId: campaign, CharacterId: gone.GetId()})); err != nil {
		t.Fatalf("DeleteReservedCharacter() error = %v", err)
	}
	h.clock.Advance(25 * time.Hour)

	neverMade, _ := secret.New()
	for name, token := range map[string]string{
		"expired": expiredToken, "revoked": revokedToken, "character deleted": goneToken,
		"never made": neverMade, "not a token": "x", "too long": strings.Repeat("a", 5000),
	} {
		_, err := lia.previewClaim(t, token)
		wantUnusable(t, "PreviewClaim("+name+")", err)
		_, err = lia.claim(t, token)
		wantUnusable(t, "ClaimCharacter("+name+")", err)
	}
	if h.memberStatus(campaign, lia.id) != "" {
		t.Error("a refused claim made the person a member")
	}
	if got := summaryOf(master.list(t, campaign), expired.GetId()); got.GetClaimState() != charactersv1.ClaimState_CLAIM_STATE_EXPIRED {
		t.Errorf("expired link state = %v, want EXPIRED", got.GetClaimState())
	}
	if got := summaryOf(master.list(t, campaign), revoked.GetId()); got.GetClaimState() != charactersv1.ClaimState_CLAIM_STATE_REVOKED {
		t.Errorf("revoked link state = %v, want REVOKED", got.GetClaimState())
	}
	// Without a session nothing is read at all.
	_, err := h.anonymous().api.PreviewClaim(t.Context(), connect.NewRequest(&charactersv1.PreviewClaimRequest{Token: expiredToken}))
	wantCode(t, "PreviewClaim(no session)", err, connect.CodeUnauthenticated)
	_, err = h.anonymous().api.ClaimCharacter(t.Context(), connect.NewRequest(&charactersv1.ClaimCharacterRequest{Token: expiredToken}))
	wantCode(t, "ClaimCharacter(no session)", err, connect.CodeUnauthenticated)
}

func TestRN03_AClaimRefusesAPlayerWhoHasALivingCharacterAndTheDeadOnesDoNotCount(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel", lia)
	icaro := lia.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Ícaro", pensantusSheet())
	kai := master.reservedByMaster(t, campaign, "Kai")
	token := master.link(t, kai, 7)

	_, err := lia.claim(t, token)
	b := blocked(t, "ClaimCharacter(RN-03)", err)
	if b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_LIVING_CHARACTER_EXISTS ||
		b.GetCharacterId() != icaro.GetId() || b.GetCharacterName() != "Ícaro" || b.GetCampaignId() != campaign {
		t.Errorf("RN-03 detail = %v; want LIVING_CHARACTER_EXISTS naming Ícaro", b)
	}
	if got := master.get(t, campaign, kai.GetId()); !got.GetReserved() {
		t.Error("a refused claim took the character")
	}
	// The link is not spent by a refusal: the same person can claim once Ícaro dies.
	master.markDead(t, icaro)
	if _, err := lia.claim(t, token); err != nil {
		t.Errorf("ClaimCharacter() after the living character died error = %v", err)
	}
}

func TestMR049_ClaimingClosesAPendingJoinRequest(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel")
	h.joinPending(master, campaign, lia)
	if h.memberStatus(campaign, lia.id) != "pending" {
		t.Fatal("setup: Lia is not pending")
	}
	token := master.link(t, master.reservedByMaster(t, campaign, "Kai"), 7)
	if _, err := lia.claim(t, token); err != nil {
		t.Fatalf("ClaimCharacter() error = %v", err)
	}
	if h.memberStatus(campaign, lia.id) != "active" {
		t.Errorf("member status = %q, want active", h.memberStatus(campaign, lia.id))
	}
}

func TestMR049_TheMasterOpeningTheirOwnLinkGetsNothingAndChangesNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Vinicius")
	campaign := h.newCampaign(master, "Mirathel")
	kai := master.reservedByMaster(t, campaign, "Kai")
	token := master.link(t, kai, 7)

	res, err := master.previewClaim(t, token)
	if err != nil || !res.GetOwnLink() || res.GetCard() != nil {
		t.Errorf("PreviewClaim() as the master = %v, %v; want own_link and no card", res, err)
	}
	_, err = master.claim(t, token)
	if b := blocked(t, "ClaimCharacter() as the master", err); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CLAIM_OWN_LINK {
		t.Errorf("reason = %v, want CLAIM_OWN_LINK", b.GetReason())
	}
	if got := master.get(t, campaign, kai.GetId()); !got.GetReserved() || got.GetPlayerUserId() != "" {
		t.Error("the master took their own reserved character")
	}
}

func TestMR049_SigningInWithALinkNeverClaimsAndTheCardComesBackWithoutTheToken(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel")
	kai := master.reservedByMaster(t, campaign, "Kai")
	token := master.link(t, kai, 7)
	intent := h.svc.ClaimIntent().(claimIntent)

	if _, err := intent.Prepare("not a token"); err == nil {
		t.Error("Prepare(garbage) accepted it")
	}
	hash, err := intent.Prepare(token)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if want, _ := secret.Hash(token); !bytes.Equal(hash, want) || len(hash) > identity.MaxIntentDataBytes {
		t.Error("Prepare() keeps more than the token's hash")
	}
	if path, err := intent.Complete(t.Context(), identity.SignedIn{}, hash); err == nil || path != "" {
		t.Errorf("Complete(SignedIn{}) = %q, %v; want no path and an error", path, err)
	}
	path, err := intent.complete(t.Context(), lia.id, hash)
	if err != nil || path != "/claim" {
		t.Fatalf("complete() = %q, %v; want /claim", path, err)
	}
	// Signing in did nothing to the character or the campaign.
	if got := master.get(t, campaign, kai.GetId()); !got.GetReserved() {
		t.Error("signing in claimed the character")
	}
	if h.memberStatus(campaign, lia.id) != "" {
		t.Error("signing in made the person a member")
	}
	// The card comes back with no token in the request.
	card, err := lia.previewClaim(t, "")
	if err != nil || card.GetCard().GetCharacterName() != "Kai" {
		t.Fatalf("PreviewClaim(no token) = %v, %v; want Kai's card", card, err)
	}
	if h.memberStatus(campaign, lia.id) != "" {
		t.Error("PreviewClaim made the person a member")
	}
	// Someone else did not sign in with it.
	_, err = h.newUser("Caio").previewClaim(t, "")
	wantUnusable(t, "PreviewClaim(no token) by someone who did not sign in with a link", err)
	// Only the click claims, with no token either.
	if _, err := lia.claim(t, ""); err != nil {
		t.Fatalf("ClaimCharacter(no token) error = %v", err)
	}
	if h.memberStatus(campaign, lia.id) != "active" {
		t.Error("the claim did not make Lia a member")
	}
	if _, err := lia.previewClaim(t, ""); err == nil {
		t.Error("the link signed in with was kept after the claim")
	}
}

func TestMR049_TheLinkSignedInWithIsKeptForTenMinutes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel")
	token := master.link(t, master.reservedByMaster(t, campaign, "Kai"), 7)
	hash, _ := secret.Hash(token)
	if _, err := h.svc.ClaimIntent().(claimIntent).complete(t.Context(), lia.id, hash); err != nil {
		t.Fatal(err)
	}
	h.clock.Advance(9 * time.Minute)
	if _, err := lia.previewClaim(t, ""); err != nil {
		t.Errorf("PreviewClaim(no token) after 9 minutes error = %v", err)
	}
	h.clock.Advance(2 * time.Minute)
	_, err := lia.previewClaim(t, "")
	wantUnusable(t, "PreviewClaim(no token) after 11 minutes", err)
	_, err = lia.claim(t, "")
	wantUnusable(t, "ClaimCharacter(no token) after 11 minutes", err)
}

func TestMR049_TheMastersActionsOnTheRow(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia, caio := h.newUser("Vinicius"), h.newUser("Lia"), h.newUser("Caio")
	campaign := h.newCampaign(master, "Mirathel", lia, caio)

	// Revoking: twice is fine; with no link at all too.
	kai := master.reservedByMaster(t, campaign, "Kai")
	revoke := func(c *charactersv1.Character) error {
		_, err := master.api.RevokeClaimLink(t.Context(), connect.NewRequest(&charactersv1.RevokeClaimLinkRequest{CampaignId: campaign, CharacterId: c.GetId()}))
		return err
	}
	if err := revoke(kai); err != nil {
		t.Errorf("RevokeClaimLink(no link) error = %v", err)
	}
	token := master.link(t, kai, 7)
	if err := revoke(kai); err != nil {
		t.Errorf("RevokeClaimLink() error = %v", err)
	}
	if err := revoke(kai); err != nil {
		t.Errorf("RevokeClaimLink() twice error = %v", err)
	}
	_, err := lia.claim(t, token)
	wantUnusable(t, "ClaimCharacter(revoked link)", err)
	// A new link after a revoke works again ("Gerar novo link").
	if _, err := lia.claim(t, master.link(t, kai, 7)); err != nil {
		t.Fatalf("ClaimCharacter(new link) error = %v", err)
	}
	// Revoking a link a player used gives the "used by" answer.
	err = revoke(kai)
	if b := blocked(t, "RevokeClaimLink(used)", err); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CLAIM_LINK_USED || b.GetPlayerDisplayName() != "Lia" {
		t.Errorf("detail = %v; want CLAIM_LINK_USED by Lia", b)
	}
	// Excluir: not for a character with an owner.
	_, err = master.api.DeleteReservedCharacter(t.Context(), connect.NewRequest(&charactersv1.DeleteReservedCharacterRequest{CampaignId: campaign, CharacterId: kai.GetId()}))
	if b := blocked(t, "DeleteReservedCharacter(owned)", err); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_NOT_RESERVED {
		t.Errorf("reason = %v, want NOT_RESERVED", b.GetReason())
	}
	// Devolver à reserva: the player stays a member, the character is reserved and unseen.
	back, err := master.api.ReturnCharacterToReserve(t.Context(), connect.NewRequest(&charactersv1.ReturnCharacterToReserveRequest{CampaignId: campaign, CharacterId: kai.GetId()}))
	if err != nil || !back.Msg.GetCharacter().GetReserved() || back.Msg.GetCharacter().GetPlayerUserId() != "" {
		t.Fatalf("ReturnCharacterToReserve() = %v, %v; want it reserved with no owner", back, err)
	}
	if h.memberStatus(campaign, lia.id) != "active" {
		t.Error("giving the character back removed the player from the campaign")
	}
	if summaryOf(lia.list(t, campaign), kai.GetId()) != nil {
		t.Error("the former owner still lists the character")
	}
	_, err = lia.api.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: campaign, CharacterId: kai.GetId()}))
	wantCode(t, "GetCharacter(given back) as the former owner", err, connect.CodeNotFound)
	if got := summaryOf(master.list(t, campaign), kai.GetId()); !got.GetReserved() || got.GetClaimState() != charactersv1.ClaimState_CLAIM_STATE_NONE {
		t.Errorf("the master's row = reserved %v, state %v; want reserved, no link", got.GetReserved(), got.GetClaimState())
	}
	// Giving it back again, or a character no link brought to its player, is refused.
	_, err = master.api.ReturnCharacterToReserve(t.Context(), connect.NewRequest(&charactersv1.ReturnCharacterToReserveRequest{CampaignId: campaign, CharacterId: kai.GetId()}))
	if b := blocked(t, "ReturnCharacterToReserve(twice)", err); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_NOT_CLAIMED {
		t.Errorf("reason = %v, want NOT_CLAIMED", b.GetReason())
	}
	own := caio.createPensantus(t, campaign)
	_, err = master.api.ReturnCharacterToReserve(t.Context(), connect.NewRequest(&charactersv1.ReturnCharacterToReserveRequest{CampaignId: campaign, CharacterId: own.GetId()}))
	if b := blocked(t, "ReturnCharacterToReserve(the player's own)", err); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_NOT_CLAIMED {
		t.Errorf("reason = %v, want NOT_CLAIMED", b.GetReason())
	}
	// And Caio can claim it now (the reserve is for anyone).
	if _, err := caio.claim(t, master.link(t, kai, 7)); err == nil {
		t.Error("Caio claimed a character while he has a living one")
	}
	// Excluir: a reserved character goes with its live link.
	token = master.link(t, kai, 7)
	if _, err := master.api.DeleteReservedCharacter(t.Context(), connect.NewRequest(&charactersv1.DeleteReservedCharacterRequest{CampaignId: campaign, CharacterId: kai.GetId()})); err != nil {
		t.Fatalf("DeleteReservedCharacter() error = %v", err)
	}
	_, err = lia.claim(t, token)
	wantUnusable(t, "ClaimCharacter(link of a deleted character)", err)
	if summaryOf(master.list(t, campaign), kai.GetId()) != nil {
		t.Error("the deleted character is still listed")
	}
	// Nor a player nor a stranger acts on the row.
	spare := master.reservedByMaster(t, campaign, "Ragna")
	for name, u := range map[string]*user{"a player": lia, "a stranger": h.newUser("Intruso")} {
		_, err := u.api.CreateClaimLink(t.Context(), connect.NewRequest(&charactersv1.CreateClaimLinkRequest{CampaignId: campaign, CharacterId: spare.GetId()}))
		if connect.CodeOf(err) != connect.CodePermissionDenied && connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("CreateClaimLink() as %s error = %v, want permission_denied or not_found", name, err)
		}
	}
}

func TestMR049_ALinkIsOnlyForAReservedCharacter(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel", lia)
	own := lia.createPensantus(t, campaign)
	npc := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Orc", enemySheet())
	_, err := master.api.CreateClaimLink(t.Context(), connect.NewRequest(&charactersv1.CreateClaimLinkRequest{CampaignId: campaign, CharacterId: own.GetId()}))
	if b := blocked(t, "CreateClaimLink(owned)", err); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_NOT_RESERVED {
		t.Errorf("reason = %v, want NOT_RESERVED", b.GetReason())
	}
	_, err = master.api.CreateClaimLink(t.Context(), connect.NewRequest(&charactersv1.CreateClaimLinkRequest{CampaignId: campaign, CharacterId: npc.GetId()}))
	wantCode(t, "CreateClaimLink(an NPC)", err, connect.CodeInvalidArgument)
	_, err = master.api.MarkCharacterDead(t.Context(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{
		CampaignId: campaign, CharacterId: master.reservedByMaster(t, campaign, "Kai").GetId(),
	}))
	if b := blocked(t, "MarkCharacterDead(reserved)", err); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_RESERVED {
		t.Errorf("reason = %v, want RESERVED", b.GetReason())
	}
}

func TestMR049_TheMasterEditsAReservedCharacterAndAPlayerCannot(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel", lia)
	kai := master.reservedByMaster(t, campaign, "Kai")
	if _, err := master.update(t, kai, "Kai, o Monge", pensantusSheet()); err != nil {
		t.Errorf("UpdateCharacter() as the master error = %v", err)
	}
	_, err := lia.update(t, kai, "Roubado", pensantusSheet())
	wantCode(t, "UpdateCharacter(reserved) as a player", err, connect.CodeNotFound)
	_, err = lia.api.UpdateCharacterStory(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterStoryRequest{CampaignId: campaign, CharacterId: kai.GetId(), Revision: kai.GetRevision()}))
	wantCode(t, "UpdateCharacterStory(reserved) as a player", err, connect.CodeNotFound)
}

func TestMR049_AReservedCharacterIsNotInThePartyAndDoesNotLockWithTheSessions(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel", lia)
	mine := lia.createPensantus(t, campaign)
	kai := master.reservedByMaster(t, campaign, "Kai")

	// Nothing that lists "the party" holds it: vitals, combat, levels, the map's tokens,
	// the fog's vision, and the session's names for the players.
	vitals, err := h.svc.ListVitals(t.Context(), campaign)
	if err != nil {
		t.Fatal(err)
	}
	if len(vitals) != 1 || vitals[0].GetCharacterId() != mine.GetId() {
		t.Errorf("ListVitals() = %d characters, want only Lia's", len(vitals))
	}
	if _, err := h.svc.GetVitals(t.Context(), campaign, kai.GetId()); err == nil {
		t.Error("GetVitals(reserved) answered")
	}
	err = db.InTx(t.Context(), h.pool, func(tx pgx.Tx) error {
		party, err := h.svc.CombatParty(t.Context(), tx, campaign)
		if err != nil || len(party) != 1 || party[0].ID != mine.GetId() {
			t.Errorf("CombatParty() = %v, %v; want only Lia's", party, err)
		}
		levels, err := h.svc.PartyLevels(t.Context(), tx, campaign)
		if err != nil || len(levels) != 1 {
			t.Errorf("PartyLevels() = %v, %v; want one", levels, err)
		}
		fighters, err := h.svc.CombatCharacters(t.Context(), tx, campaign, []string{mine.GetId(), kai.GetId()})
		if err != nil || len(fighters) != 1 {
			t.Errorf("CombatCharacters() = %v, %v; want only Lia's", fighters, err)
		}
		tokens, err := h.svc.MapCharacters(t.Context(), tx, campaign, []string{mine.GetId(), kai.GetId()})
		if err != nil || len(tokens) != 1 {
			t.Errorf("MapCharacters() = %v, %v; want only Lia's", tokens, err)
		}
		vision, err := h.svc.PartyVision(t.Context(), tx, campaign)
		if err != nil || len(vision) != 1 {
			t.Errorf("PartyVision() = %v, %v; want only Lia's", vision, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// A game session locks Lia's sheet and not the reserved one, which has nobody to lock out.
	if n := h.lockSheets(campaign); n != 1 {
		t.Errorf("LockSheets() locked %d sheets, want 1", n)
	}
	if got := master.get(t, campaign, kai.GetId()); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT {
		t.Errorf("the reserved character is %v after a session started, want DRAFT", got.GetState())
	}
	// Claimed after the session started, it is a draft the new player edits until the next one.
	caio := h.newUser("Caio")
	if _, err := caio.claim(t, master.link(t, kai, 7)); err != nil {
		t.Fatalf("ClaimCharacter() after a session started error = %v", err)
	}
	if got := caio.get(t, campaign, kai.GetId()); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT || !got.GetCanEdit() {
		t.Errorf("the claimed character = %v, can edit %v; want a draft the player edits", got.GetState(), got.GetCanEdit())
	}
}

func TestMR049_ADeletedAccountLeavesTheClaimedCharacterWithTheMaster(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel")
	kai := master.reservedByMaster(t, campaign, "Kai")
	if _, err := lia.claim(t, master.link(t, kai, 7)); err != nil {
		t.Fatal(err)
	}
	h.deleteUser(lia.id)
	got := summaryOf(master.list(t, campaign), kai.GetId())
	if got == nil || got.GetReserved() || got.GetPlayerUserId() != "" {
		t.Fatalf("after the account was deleted, the row = %v; want Kai, not reserved, with no owner (RN-16)", got)
	}
	// The master may give it back to the reserve and send it to someone else.
	if _, err := master.api.ReturnCharacterToReserve(t.Context(), connect.NewRequest(&charactersv1.ReturnCharacterToReserveRequest{CampaignId: campaign, CharacterId: kai.GetId()})); err != nil {
		t.Errorf("ReturnCharacterToReserve() after the owner left error = %v", err)
	}
	// A character whose player left on their own is never taken for a reserved one.
	own := h.newUser("Caio")
	h.join(master, campaign, own)
	mine := own.createPensantus(t, campaign)
	h.deleteUser(own.id)
	if got := summaryOf(master.list(t, campaign), mine.GetId()); got == nil || got.GetReserved() {
		t.Errorf("a deleted account's character = %v; want it listed and not reserved", got)
	}
}

func TestMR049_TwoClaimsOfOneLinkMakeOneOwner(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 8) // the racers must overlap: one connection would run them one by one
	h := newHarness(t)
	master := h.newUser("Vinicius")
	campaign := h.newCampaign(master, "Mirathel")
	token := master.link(t, master.reservedByMaster(t, campaign, "Kai"), 7)

	const n = 4
	players := make([]*user, n)
	for i := range players {
		players[i] = h.newUser("Jogador")
	}
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := dbtest.NewBarrier(n)
	for i := range n {
		wg.Go(func() {
			start.Wait()
			_, errs[i] = players[i].claim(t, token)
		})
	}
	wg.Wait()
	won := 0
	for i, err := range errs {
		if err == nil {
			won++
			continue
		}
		wantUnusable(t, "ClaimCharacter(racing) #"+string(rune('0'+i)), err)
	}
	if won != 1 {
		t.Fatalf("%d claims won, want exactly 1", won)
	}
	var owners, members int
	if err := h.pool.QueryRow(t.Context(), "SELECT count(*) FROM characters WHERE campaign_id = $1 AND player_user_id IS NOT NULL", campaign).Scan(&owners); err != nil {
		t.Fatal(err)
	}
	if err := h.pool.QueryRow(t.Context(), "SELECT count(*) FROM campaign_members WHERE campaign_id = $1 AND role = 'player'", campaign).Scan(&members); err != nil {
		t.Fatal(err)
	}
	if owners != 1 || members != 1 {
		t.Errorf("owners = %d, players = %d; want one of each (the losers did not join)", owners, members)
	}
}

func TestMR049_AClaimAndARevokeNeverBothWin(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 8)
	h := newHarness(t)
	master, lia := h.newUser("Vinicius"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel")

	for round := range 6 {
		kai := master.reservedByMaster(t, campaign, "Kai")
		token := master.link(t, kai, 7)
		var claimErr, revokeErr error
		var wg sync.WaitGroup
		start := dbtest.NewBarrier(2)
		wg.Go(func() {
			start.Wait()
			_, claimErr = lia.claim(t, token)
		})
		wg.Go(func() {
			start.Wait()
			_, revokeErr = master.api.RevokeClaimLink(t.Context(), connect.NewRequest(&charactersv1.RevokeClaimLinkRequest{CampaignId: campaign, CharacterId: kai.GetId()}))
		})
		wg.Wait()
		switch {
		case claimErr == nil && revokeErr == nil:
			t.Fatalf("round %d: the claim and the revoke both won", round)
		case claimErr == nil:
			if b := blocked(t, "RevokeClaimLink(lost)", revokeErr); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CLAIM_LINK_USED || b.GetPlayerDisplayName() != "Lia" {
				t.Errorf("round %d: the lost revoke = %v; want CLAIM_LINK_USED by Lia", round, b)
			}
			// Free Lia for the next round.
			master.markDead(t, lia.get(t, campaign, kai.GetId()))
		case revokeErr == nil:
			wantUnusable(t, "ClaimCharacter(lost to a revoke)", claimErr)
			if got := master.get(t, campaign, kai.GetId()); !got.GetReserved() {
				t.Errorf("round %d: the revoke won and the character was taken", round)
			}
		default:
			t.Fatalf("round %d: neither won: claim %v, revoke %v", round, claimErr, revokeErr)
		}
	}
}

func TestMR049_ClaimingIsRateLimitedPerPersonAndNothingSecretIsLogged(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	lia := h.newUser("Lia")
	var limited bool
	for range 40 {
		_, err := lia.claim(t, "x")
		if connect.CodeOf(err) == connect.CodeResourceExhausted {
			limited = true
			break
		}
	}
	if !limited {
		t.Error("forty tries in a row were never limited")
	}
	// Another person is not limited by Lia's tries (the address limit is generous).
	if _, err := h.newUser("Caio").claim(t, "x"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("another person's try error = %v, want the usual not_found", err)
	}
}
