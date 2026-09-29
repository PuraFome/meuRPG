package campaigns

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/campaigns/campaignsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/secret"
)

// An invite link is https://<app>/convite#t=<token>. The token is a random
// secret (package secret) that the server returns once, to the master, and
// then only knows as a SHA-256 hash. It travels in the URL fragment, which
// browsers never send to a server, and the app posts it to AcceptInvite in
// the request body (ADR-0009, docs/privacidade.md).

// newInviteToken returns a new invite token and the hash to store.
func newInviteToken() (token string, hash []byte) { return secret.New() }

// errInviteNotFound is the answer for a token or invite ID that matches no
// invite of the campaign.
func errInviteNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("invite not found"))
}

// inviteLimits applies the defaults and limits to a CreateInvite request.
func inviteLimits(req *campaignsv1.CreateInviteRequest) (maxUses int32, lifetime time.Duration, err error) {
	maxUses = req.GetMaxUses()
	if maxUses == 0 {
		maxUses = DefaultInviteUses
	}
	if maxUses < 1 || maxUses > MaxInviteUses {
		return 0, 0, invalidArgument("max_uses", fmt.Errorf("must be between 1 and %d", MaxInviteUses))
	}

	lifetime = DefaultInviteLifetime
	if req.GetExpiresIn() != nil {
		if err := req.GetExpiresIn().CheckValid(); err != nil {
			return 0, 0, invalidArgument("expires_in", errors.New("must be a valid duration"))
		}
		lifetime = req.GetExpiresIn().AsDuration()
	}
	if lifetime < MinInviteLifetime || lifetime > MaxInviteLifetime {
		return 0, 0, invalidArgument("expires_in", fmt.Errorf("must be between %v and %v", MinInviteLifetime, MaxInviteLifetime))
	}
	return maxUses, lifetime, nil
}

// inviteState says whether the invite works at now. When several reasons
// apply, revoked wins (the master's explicit decision), then used up (it
// warns a player that someone else may have used their link), then expired.
func inviteState(invite campaignsdb.CampaignInvite, now time.Time) campaignsv1.InviteState {
	switch {
	case invite.RevokedAt != nil:
		return campaignsv1.InviteState_INVITE_STATE_REVOKED
	case invite.UseCount >= invite.MaxUses:
		return campaignsv1.InviteState_INVITE_STATE_USED_UP
	case !now.Before(invite.ExpiresAt):
		return campaignsv1.InviteState_INVITE_STATE_EXPIRED
	default:
		return campaignsv1.InviteState_INVITE_STATE_ACTIVE
	}
}

// errInviteUnusable is AcceptInvite's answer for an invite that exists but
// does not work. The InviteUnusable detail tells the app which message to
// show.
func errInviteUnusable(state campaignsv1.InviteState) error {
	msg := map[campaignsv1.InviteState]string{
		campaignsv1.InviteState_INVITE_STATE_EXPIRED: "this invite has expired; ask the master for a new one",
		campaignsv1.InviteState_INVITE_STATE_REVOKED: "this invite was revoked; ask the master for a new one",
		campaignsv1.InviteState_INVITE_STATE_USED_UP: "this invite was already used; ask the master for a new one",
	}[state]
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	if detail, detailErr := connect.NewErrorDetail(&campaignsv1.InviteUnusable{State: state}); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

// joinResult is what acceptInvite did.
type joinResult struct {
	campaign      campaignsdb.Campaign
	role          authz.Role
	alreadyMember bool
}

// acceptInvite makes userID a player of the campaign the token invites to,
// in one transaction:
//
//  1. Find the invite by the token's hash and lock it (FOR UPDATE).
//  2. Already a member? Return the campaign; nothing changes, and no use of
//     the invite is spent. This makes AcceptInvite idempotent, so a double
//     click or a retry is harmless.
//  3. Otherwise the invite must work: not revoked, not used up, not expired.
//  4. Spend one use and add the membership.
//
// Two people racing for an invite's last use cannot both get in: the lock
// makes the second transaction wait for the first, and it then finds the
// invite used up (TestAcceptInviteRaceForTheLastUse). The UPDATE in step 4
// repeats the rules, and a CHECK keeps use_count <= max_uses, so even a bug
// here could not overspend an invite.
func (s *Service) acceptInvite(ctx context.Context, token, userID string) (joinResult, error) {
	tokenHash, ok := secret.Hash(token)
	if !ok {
		return joinResult{}, errInviteNotFound() // no invite could have this token
	}
	now := s.now()

	var result joinResult
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)

		invite, err := q.GetInviteByTokenHashForUpdate(ctx, tokenHash)
		if errors.Is(err, pgx.ErrNoRows) {
			return errInviteNotFound()
		}
		if err != nil {
			return fmt.Errorf("get invite: %w", err)
		}
		campaign, err := q.GetCampaign(ctx, invite.CampaignID)
		if err != nil {
			return fmt.Errorf("get campaign: %w", err)
		}

		role, err := q.GetMemberRole(ctx, campaignsdb.GetMemberRoleParams{CampaignID: invite.CampaignID, UserID: userID})
		switch {
		case err == nil:
			result = joinResult{campaign: campaign, role: authz.Role(role), alreadyMember: true}
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("get member role: %w", err)
		}

		if state := inviteState(invite, now); state != campaignsv1.InviteState_INVITE_STATE_ACTIVE {
			return errInviteUnusable(state)
		}
		spent, err := q.IncrementInviteUses(ctx, campaignsdb.IncrementInviteUsesParams{ID: invite.ID, Now: now})
		if err != nil {
			return fmt.Errorf("spend an invite use: %w", err)
		}
		if spent == 0 {
			// Unreachable while the row is locked and checked above; kept
			// so that a future change cannot turn into a free pass.
			return errInviteUnusable(campaignsv1.InviteState_INVITE_STATE_USED_UP)
		}
		_, err = q.InsertMember(ctx, campaignsdb.InsertMemberParams{
			CampaignID: invite.CampaignID,
			UserID:     userID,
			Role:       string(authz.RolePlayer),
		})
		if err != nil {
			return fmt.Errorf("insert player: %w", err)
		}
		result = joinResult{campaign: campaign, role: authz.RolePlayer}
		return nil
	})
	return result, err
}

func inviteToProto(invite campaignsdb.CampaignInvite, now time.Time) *campaignsv1.Invite {
	res := &campaignsv1.Invite{
		Id:        invite.ID,
		MaxUses:   invite.MaxUses,
		UseCount:  invite.UseCount,
		CreatedAt: timestamppb.New(invite.CreatedAt),
		ExpiresAt: timestamppb.New(invite.ExpiresAt),
		State:     inviteState(invite, now),
	}
	if invite.RevokedAt != nil {
		res.RevokedAt = timestamppb.New(*invite.RevokedAt)
	}
	return res
}

// parseUUID returns id in canonical form, or false if it is not a UUID.
func parseUUID(id string) (string, bool) {
	u, err := uuid.Parse(id)
	if err != nil {
		return "", false
	}
	return u.String(), true
}
