package play

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
	"uuid"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Revivify outside a combat (SRD 5.1, Revivify). The spell reaches back one minute and the app
// does not count time outside a combat, so the cast waits for the master (REVIVIFY_TIME_PENDING):
// nothing is spent, and the target is not touched, until he says whether the creature died less
// than a minute ago. "Já passou" ends it at no cost, and the caster is told only that the master
// said it does not work (RN-10). A cast inside a combat is CastSpell.

// The database's statuses of a revivify request.
const (
	revivifyPending   = "pending"
	revivifyConfirmed = "confirmed"
	revivifyDenied    = "denied"
)

var revivifyStatusToProto = map[string]playv1.RevivifyRequestStatus{
	revivifyPending:   playv1.RevivifyRequestStatus_REVIVIFY_REQUEST_STATUS_PENDING,
	revivifyConfirmed: playv1.RevivifyRequestStatus_REVIVIFY_REQUEST_STATUS_CONFIRMED,
	revivifyDenied:    playv1.RevivifyRequestStatus_REVIVIFY_REQUEST_STATUS_DENIED,
}

// errRevivifyRequestNotFound is the answer for a request the caller may not answer or read:
// the same for one that does not exist, so no pending_id is probed.
func errRevivifyRequestNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("revivify request not found"))
}

// castersCharacter finds the living character that casts outside a combat, a player's own for a
// player. not_found for any other, as the same answer for one that is not there.
func (s *Service) castersCharacter(ctx context.Context, tx pgx.Tx, m authz.Membership, characterID string) (link.Character, error) {
	id, err := uuid.Parse(characterID)
	if err != nil {
		return link.Character{}, connect.NewError(connect.CodeNotFound, errors.New("character not found"))
	}
	chars, err := s.roster.CombatCharacters(ctx, tx, m.CampaignID, []string{id.String()})
	if err != nil {
		return link.Character{}, err
	}
	if len(chars) != 1 || !chars[0].Player {
		return link.Character{}, connect.NewError(connect.CodeNotFound, errors.New("character not found"))
	}
	if m.Role != authz.RoleMaster && chars[0].PlayerUserID != m.UserID {
		return link.Character{}, connect.NewError(connect.CodePermissionDenied, errors.New("only the character's player or the master may do this"))
	}
	return chars[0], nil
}

// outsideOptions is what the caster could cast now, outside a combat: a fresh turn.
func (s *Service) outsideOptions(ctx context.Context, tx pgx.Tx, campaignID string, caster link.Character) (castable, error) {
	opts, err := s.roster.CombatTurnOptions(ctx, tx, campaignID, caster.ID, link.Turn{})
	if err != nil {
		return castable{}, err
	}
	return castableOf(opts, revivifyKey)
}

// previewOutside is PreviewRevivify with no combat: every dead character of the campaign, to be
// confirmed by the master when the spell is cast.
func (s *Service) previewOutside(ctx context.Context, m authz.Membership, req *playv1.PreviewRevivifyRequest) (*connect.Response[playv1.PreviewRevivifyResponse], error) {
	caster, err := s.castersCharacter(ctx, nil, m, req.GetCasterCharacterId())
	if err != nil {
		return nil, err
	}
	cast, err := s.outsideOptions(ctx, nil, m.CampaignID, caster)
	if err != nil {
		return nil, err
	}
	dead, err := s.roster.DeadCharacters(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the dead", err)
	}
	res := &playv1.PreviewRevivifyResponse{}
	res.Slot, res.SlotsFree = revivifySlot(cast)
	for _, d := range dead {
		switch {
		case !d.RevivifyBlocked:
			res.Targets = append(res.Targets, &playv1.RevivifyTarget{TargetId: d.ID, Name: d.Name, NeedsMasterConfirmation: true})
		case m.Role == authz.RoleMaster:
			res.Unavailable = append(res.Unavailable, &playv1.RevivifyUnavailable{
				TargetId: d.ID, Name: d.Name, Reason: playv1.RevivifyUnavailableReason_REVIVIFY_UNAVAILABLE_REASON_MASTER_BLOCKED,
			})
		}
	}
	return connect.NewResponse(res), nil
}

// RequestRevivify implements playv1connect.RevivifyServiceHandler.
func (s *Service) RequestRevivify(
	ctx context.Context,
	req *connect.Request[playv1.RequestRevivifyRequest],
) (*connect.Response[playv1.RequestRevivifyResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	if !req.Msg.GetMaterialConfirmed() {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("material_confirmed is required: the spell consumes diamonds worth 300 gp"))
	}
	targetID, err := uuid.Parse(req.Msg.GetTargetCharacterId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("character not found"))
	}
	scoped, hash := idem.Scope(m.CampaignID+":"+m.UserID, key), idem.Hash(req.Msg)

	var row playdb.RevivifyRequest
	var replayed bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		replayed = false
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		get := func(ctx context.Context, k *string) (playdb.RevivifyRequest, error) {
			return q.GetRevivifyRequestByCreateKey(ctx, playdb.GetRevivifyRequestByCreateKeyParams{CampaignID: m.CampaignID, CreateKey: k})
		}
		row, replayed, err = idem.Create(ctx, scoped, hash, get,
			func(r playdb.RevivifyRequest) *string { return r.CreateHash },
			func() (playdb.RevivifyRequest, error) {
				return s.openRevivifyRequest(ctx, tx, q, m, session, req.Msg, targetID.String(), scoped, hash)
			})
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "ask for Revivify", err)
	}
	if !replayed {
		logging.Event(ctx, s.logger, "revivify.requested", slog.String("request_id", row.ID), slog.String("character_id", row.CasterCharacterID))
		s.publishRevivifyChanged(m.CampaignID, deref(row.RequestedByUserID))
	}
	out, err := s.revivifyRequestsProto(ctx, m.CampaignID, []playdb.RevivifyRequest{row}, nil)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.RequestRevivifyResponse{Request: out[0]}), nil
}

// openRevivifyRequest checks the cast and writes the request, inside the transaction. It returns
// pgx.ErrNoRows when the key's unique index already holds a request (idem.Create reads it).
func (s *Service) openRevivifyRequest(ctx context.Context, tx pgx.Tx, q *playdb.Queries, m authz.Membership, session playdb.GameSession, req *playv1.RequestRevivifyRequest, targetID string, scoped, hash *string) (playdb.RevivifyRequest, error) {
	caster, err := s.castersCharacter(ctx, tx, m, req.GetCasterCharacterId())
	if err != nil {
		return playdb.RevivifyRequest{}, err
	}
	// A caster in a combat casts inside it, where the minute is counted.
	if enc, err := q.GetOpenEncounter(ctx, session.ID); err == nil {
		cs, err := q.ListCombatants(ctx, enc.ID)
		if err != nil {
			return playdb.RevivifyRequest{}, fmt.Errorf("list the combatants: %w", err)
		}
		if slices.ContainsFunc(cs, func(c playdb.Combatant) bool {
			return c.Kind == kindPlayer && c.CharacterID == caster.ID && !c.Defeated
		}) {
			return playdb.RevivifyRequest{}, connect.NewError(connect.CodeInvalidArgument, errors.New("the caster is in a combat: cast Revivify inside it"))
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return playdb.RevivifyRequest{}, fmt.Errorf("find the open encounter: %w", err)
	}
	cast, err := s.outsideOptions(ctx, tx, m.CampaignID, caster)
	if err != nil {
		return playdb.RevivifyRequest{}, err
	}
	if !cast.enabled {
		if err := castError(cast.reason, true); err != nil {
			return playdb.RevivifyRequest{}, err
		}
	}
	slot, err := slotOf(req.GetSlot(), cast)
	if err != nil {
		return playdb.RevivifyRequest{}, err
	}
	if slot == nil {
		return playdb.RevivifyRequest{}, connect.NewError(connect.CodeInvalidArgument, errors.New("slot must be a spell slot of the 3rd level or more"))
	}
	dead, err := s.roster.DeadCharacters(ctx, tx, m.CampaignID)
	if err != nil {
		return playdb.RevivifyRequest{}, err
	}
	// The same answer for a character that is not dead, one that does not exist and one the
	// master marked (RN-10).
	if !slices.ContainsFunc(dead, func(d link.DeadCharacter) bool { return d.ID == targetID && !d.RevivifyBlocked }) {
		return playdb.RevivifyRequest{}, connect.NewError(connect.CodeNotFound, errors.New("character not found"))
	}
	return q.InsertRevivifyRequest(ctx, playdb.InsertRevivifyRequestParams{
		CampaignID: m.CampaignID, GameSessionID: session.ID, CasterCharacterID: caster.ID, TargetCharacterID: targetID,
		RequestedByUserID: &m.UserID, SlotLevel: slot.Level, SlotPact: slot.Pact, CreatedAt: s.now(), CreateKey: scoped, CreateHash: hash,
	})
}

// ListRevivifyRequests implements playv1connect.RevivifyServiceHandler.
func (s *Service) ListRevivifyRequests(
	ctx context.Context,
	req *connect.Request[playv1.ListRevivifyRequestsRequest],
) (*connect.Response[playv1.ListRevivifyRequestsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	session, err := openSessionWith(ctx, s.queries, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "find the open session", err)
	}
	rows, err := s.queries.ListRevivifyRequests(ctx, session.ID)
	if err != nil {
		return nil, s.dbError(ctx, "list the Revivify requests", err)
	}
	if m.Role != authz.RoleMaster { // a player reads the casts they made
		rows = slices.DeleteFunc(rows, func(r playdb.RevivifyRequest) bool { return deref(r.RequestedByUserID) != m.UserID })
	}
	out, err := s.revivifyRequestsProto(ctx, m.CampaignID, rows, nil)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ListRevivifyRequestsResponse{Requests: out}), nil
}

// ConfirmRevivifyTime implements playv1connect.RevivifyServiceHandler.
func (s *Service) ConfirmRevivifyTime(
	ctx context.Context,
	req *connect.Request[playv1.ConfirmRevivifyTimeRequest],
) (*connect.Response[playv1.ConfirmRevivifyTimeResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	if m.Role != authz.RoleMaster {
		return nil, errRevivifyRequestNotFound() // a player gets this for any pending_id
	}
	id, err := uuid.Parse(req.Msg.GetPendingId())
	if err != nil {
		return nil, errRevivifyRequestNotFound()
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	scoped, hash := idem.Scope(m.CampaignID+":"+m.UserID, key), idem.Hash(req.Msg)

	var row playdb.RevivifyRequest
	var replayed bool
	var vitals []*playv1.CharacterVitals
	var slotsLeft int32
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		replayed, vitals, slotsLeft = false, nil, 0
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		row, err = q.GetRevivifyRequestForUpdate(ctx, playdb.GetRevivifyRequestForUpdateParams{CampaignID: m.CampaignID, ID: id.String()})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.GameSessionID != session.ID) {
			return errRevivifyRequestNotFound()
		}
		if err != nil {
			return fmt.Errorf("read the request: %w", err)
		}
		if row.Status != revivifyPending {
			if row.AnswerKey != nil && *row.AnswerKey == *scoped {
				if err := idem.SameRequest(row.AnswerHash, hash); err != nil {
					return err
				}
				replayed = true // the same answer again
				return nil
			}
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("the request was answered already"))
		}
		status := revivifyDenied
		if req.Msg.GetWithinMinute() {
			status = revivifyConfirmed
			if vitals, slotsLeft, err = s.reviveOutside(ctx, tx, q, session, m, row); err != nil {
				return err
			}
		}
		at := s.now()
		row, err = q.AnswerRevivifyRequest(ctx, playdb.AnswerRevivifyRequestParams{ID: row.ID, Status: status, AnsweredAt: &at, AnswerKey: scoped, AnswerHash: hash})
		if err != nil {
			return fmt.Errorf("answer the request: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "answer Revivify", err)
	}
	if !replayed {
		logging.Event(ctx, s.logger, "revivify.answered", slog.String("request_id", row.ID), slog.String("status", row.Status))
		s.publishRevivifyChanged(m.CampaignID, deref(row.RequestedByUserID))
		for _, v := range vitals {
			s.publishVitals(m.CampaignID, v)
		}
		if row.Status == revivifyConfirmed {
			s.PublishCharacterEvent(ctx, m.CampaignID, s.ownerOf(ctx, m.CampaignID, row.TargetCharacterID), row.TargetCharacterID, characterRevived)
		}
	}
	left := map[string]int32{row.ID: slotsLeft}
	out, err := s.revivifyRequestsProto(ctx, m.CampaignID, []playdb.RevivifyRequest{row}, left)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.ConfirmRevivifyTimeResponse{Request: out[0]}), nil
}

// reviveOutside is the master's "less than a minute": the slot is spent, the diamonds are noted,
// and the target lives again with 1 hit point. RN-03 may refuse, and nothing is spent then.
func (s *Service) reviveOutside(ctx context.Context, tx pgx.Tx, q *playdb.Queries, session playdb.GameSession, m authz.Membership, row playdb.RevivifyRequest) ([]*playv1.CharacterVitals, int32, error) {
	c := &combatTx{tx: tx, q: q, session: session, svc: s}
	slot := slotRef{Level: row.SlotLevel, Pact: row.SlotPact}
	spent, err := s.spendSlot(ctx, c, row.CasterCharacterID, slot, 1)
	if err != nil {
		return nil, 0, err
	}
	if _, err := s.roster.ReviveDead(ctx, tx, m.CampaignID, row.TargetCharacterID, s.now()); err != nil {
		return nil, 0, err
	}
	after, err := s.vitals.GetVitalsTx(ctx, tx, m.CampaignID, row.TargetCharacterID)
	if err != nil {
		return nil, 0, err
	}
	body := fmt.Sprintf(`{"character_id":%q,"caster_character_id":%q,"spell":"revivify","material":true}`, row.TargetCharacterID, row.CasterCharacterID)
	if _, err := s.AppendEvent(ctx, tx, m.CampaignID, eventCharacterRevived, m.UserID, []byte(body), s.now()); err != nil {
		return nil, 0, err
	}
	return []*playv1.CharacterVitals{spent, after}, slotsLeftOf(spent, slot), nil
}

// slotsLeftOf is how many slots of the level are free in the vitals.
func slotsLeftOf(v *playv1.CharacterVitals, slot slotRef) int32 {
	if slot.Pact {
		return max(v.GetPactSlots().GetTotal()-v.GetPactSlots().GetUsed(), 0)
	}
	for _, u := range v.GetSpellSlots() {
		if u.GetLevel() == slot.Level {
			return max(u.GetTotal()-u.GetUsed(), 0)
		}
	}
	return 0
}

// ownerOf is the account that plays the character, empty for none.
func (s *Service) ownerOf(ctx context.Context, campaignID, characterID string) string {
	chars, err := s.roster.SessionCharacters(ctx, nil, campaignID, []string{characterID})
	if err != nil || len(chars) != 1 {
		return ""
	}
	return chars[0].PlayerUserID
}

// revivifyRequestsProto builds the requests as the caller reads them, with the names of the
// characters. slotsLeft, when not nil, is the slots left after a confirmed cast, by request.
func (s *Service) revivifyRequestsProto(ctx context.Context, campaignID string, rows []playdb.RevivifyRequest, slotsLeft map[string]int32) ([]*playv1.RevivifyRequest, error) {
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.CasterCharacterID, r.TargetCharacterID)
	}
	names := map[string]string{}
	chars, err := s.roster.SessionCharacters(ctx, nil, campaignID, ids)
	if err != nil {
		return nil, s.dbError(ctx, "read the names", err)
	}
	for _, c := range chars {
		names[c.ID] = c.Name
	}
	out := make([]*playv1.RevivifyRequest, 0, len(rows))
	for _, r := range rows {
		p := &playv1.RevivifyRequest{
			Id: r.ID, Status: revivifyStatusToProto[r.Status], CasterCharacterId: r.CasterCharacterID, CasterName: names[r.CasterCharacterID],
			TargetCharacterId: r.TargetCharacterID, TargetName: names[r.TargetCharacterID],
			Slot: &playv1.SpellSlot{Level: r.SlotLevel, Pact: r.SlotPact}, CreatedAt: timestamppb.New(r.CreatedAt),
		}
		if r.AnsweredAt != nil {
			p.AnsweredAt = timestamppb.New(*r.AnsweredAt)
		}
		if r.Status == revivifyConfirmed {
			p.SlotsLeft = slotsLeft[r.ID]
		}
		out = append(out, p)
	}
	return out, nil
}

// publishRevivifyChanged tells the master and the caster's player that the casts changed.
func (s *Service) publishRevivifyChanged(campaignID, casterUserID string) {
	s.hub.Publish(campaignID, live.Event{
		Audience: live.Audience{Master: true, UserID: casterUserID},
		Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_RevivifyChanged_{
			RevivifyChanged: &playv1.WatchGameSessionResponse_RevivifyChanged{},
		}},
	})
}
