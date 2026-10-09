package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Rests and the hit dice (SRD 5.1, "Resting"): the master takes a short or a long
// rest for the party, and a player spends hit dice on a short rest. The numbers
// are the characters module's (RestKeeper); this file decides who may ask and
// when, writes the history, and tells the streams.

// The kinds of session_events rows of the rests (session_event_kinds).
const (
	eventRestTaken    = "rest_taken"
	eventHitDiceSpent = "hit_dice_spent"
)

// RestKeeper is what the characters module does for the rests and the resources
// that move points between a character's vitals. Like VitalsKeeper's methods, they
// take no caller and run inside the transaction they are given; the refusals
// that are rules are link's errors, which resourceError turns into the typed answer.
type RestKeeper interface {
	// PreviewRest says what a rest would give back to each living, active player
	// character of the campaign, writing nothing.
	PreviewRest(ctx context.Context, tx pgx.Tx, campaignID string, kind playv1.RestKind) ([]*playv1.RestPreview, error)
	// TakeRest gives the party what the rest gives back, and returns the vitals before
	// and after of the characters it changed.
	TakeRest(ctx context.Context, tx pgx.Tx, campaignID string, req *playv1.TakeRestRequest) (before, after []*playv1.CharacterVitals, err error)
	// SpendHitDie spends one hit die of the size and heals the face plus the
	// Constitution modifier (at least 0, up to the maximum).
	SpendHitDie(ctx context.Context, tx pgx.Tx, campaignID, characterID string, faces, face int) (before, after *playv1.CharacterVitals, conMod, healed int, err error)
	// CreateSpellSlot is Flexible Casting's creation of a slot; ConvertSpellSlot its
	// conversion into sorcery points.
	CreateSpellSlot(ctx context.Context, tx pgx.Tx, campaignID, characterID string, level int) (before, after *playv1.CharacterVitals, cost int, err error)
	ConvertSpellSlot(ctx context.Context, tx pgx.Tx, campaignID, characterID string, level int) (before, after *playv1.CharacterVitals, gain int, err error)
	// UndoCreateSpellSlot and UndoConvertSpellSlot take them back (the master's undo).
	UndoCreateSpellSlot(ctx context.Context, tx pgx.Tx, campaignID, characterID string, level, cost int) (*playv1.CharacterVitals, error)
	UndoConvertSpellSlot(ctx context.Context, tx pgx.Tx, campaignID, characterID string, level, gain int) (*playv1.CharacterVitals, error)
}

// resourceBlocked is the failed_precondition of a rule's refusal, with the
// ResourceBlocked detail that tells the app why.
func resourceBlocked(reason playv1.ResourceBlockedReason, msg string, edit ...func(*playv1.ResourceBlocked)) error {
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	blocked := &playv1.ResourceBlocked{Reason: reason}
	for _, e := range edit {
		e(blocked)
	}
	if detail, detailErr := connect.NewErrorDetail(blocked); detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}

// resourceError turns a refusal of the characters module into the typed answer;
// any other error passes as it is.
func resourceError(err error) error {
	if err == nil {
		return nil
	}
	points := func(b *playv1.ResourceBlocked) {
		if pe, ok := errors.AsType[*link.PointsError](err); ok {
			b.Needed, b.Available = clamp32(pe.Needed, 0, 1<<20), clamp32(pe.Available, 0, 1<<20) //nolint:mnd // a bound far above any pool
		}
	}
	switch {
	case errors.Is(err, link.ErrNoHitDiceLeft):
		return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_HIT_DICE_LEFT, "the character has no hit die of that size left")
	case errors.Is(err, link.ErrNotEnoughPoints):
		return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_ENOUGH_POINTS, "there are not enough points", points)
	case errors.Is(err, link.ErrSlotLevelTooHigh):
		return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_SLOT_LEVEL_TOO_HIGH, "Flexible Casting makes slots of the 1st to the 5th level")
	case errors.Is(err, link.ErrNoFreeSlot):
		return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_FREE_SLOT, "there is no free spell slot of that level")
	case errors.Is(err, link.ErrPointsFull):
		return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_SORCERY_POINTS_FULL, "the sorcerer has the most sorcery points already", points)
	case errors.Is(err, link.ErrPointsOver):
		return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_SORCERY_POINTS_OVER, "the slot would take the sorcery points past the maximum", points)
	case errors.Is(err, link.ErrNoResource):
		return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_AVAILABLE, "the character does not have this feature")
	case errors.Is(err, link.ErrBadHitDiceChoice):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("the hit dice are not the character's"))
	}
	return err
}

// restEvent is the payload of a rest_taken event: the kind and the characters the
// rest changed. IDs only (docs/privacy.md).
type restEvent struct {
	Kind       string   `json:"kind"`
	Characters []string `json:"character_ids,omitempty"`
}

// hitDieEvent is the payload of a hit_dice_spent event: the die, the face
// rolled, the Constitution modifier and what the character regained.
type hitDieEvent struct {
	Faces    int32 `json:"faces"`
	Face     int32 `json:"face"`
	Modifier int32 `json:"modifier,omitempty"`
	Healed   int32 `json:"healed"`
	Physical bool  `json:"physical,omitempty"`
}

// activeCombatBlocks refuses with COMBAT_OPEN while a combat is running: a rest in
// the middle of a fight has no place in the turn order.
func activeCombatBlocks(ctx context.Context, q *playdb.Queries, sessionID string) error {
	enc, err := q.GetOpenEncounter(ctx, sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find the open encounter: %w", err)
	}
	if enc.Status == statusActive {
		return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_COMBAT_OPEN, "a combat is running: finish it before resting")
	}
	return nil
}

// restKindName names the kind of rest TakeRest and GetRestPreview take.
func restKindName(kind playv1.RestKind) (string, error) {
	switch kind {
	case playv1.RestKind_REST_KIND_SHORT:
		return "short", nil
	case playv1.RestKind_REST_KIND_LONG:
		return "long", nil
	}
	return "", connect.NewError(connect.CodeInvalidArgument, errors.New("kind must be a short or a long rest"))
}

// GetRestPreview implements playv1connect.ResourceServiceHandler.
func (s *Service) GetRestPreview(
	ctx context.Context,
	req *connect.Request[playv1.GetRestPreviewRequest],
) (*connect.Response[playv1.GetRestPreviewResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	if _, err := restKindName(req.Msg.GetKind()); err != nil {
		return nil, err
	}
	res := &playv1.GetRestPreviewResponse{}
	err = db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		*res = playv1.GetRestPreviewResponse{}
		q := s.queries.WithTx(tx)
		session, err := openSessionWith(ctx, q, m.CampaignID)
		if err != nil {
			return err
		}
		if res.Characters, err = s.vitals.PreviewRest(ctx, tx, m.CampaignID, req.Msg.GetKind()); err != nil {
			return err
		}
		if req.Msg.GetKind() == playv1.RestKind_REST_KIND_LONG {
			if res.LongRestAlreadyTaken, err = q.HasLongRestInSession(ctx, session.ID); err != nil {
				return fmt.Errorf("find the long rest of the session: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "preview a rest", err)
	}
	return connect.NewResponse(res), nil
}

// eventByKey finds the event a change with this key wrote and checks that the key
// is not being reused for another change: the same kind, the same request. It says
// repeated for a retry. An event of another kind or request is `invalid_argument`.
func eventByKey(ctx context.Context, q *playdb.Queries, sessionID, key, kind string, hash *string) (done playdb.GetSessionEventByIdempotencyKeyRow, repeated bool, err error) {
	done, err = q.GetSessionEventByIdempotencyKey(ctx, playdb.GetSessionEventByIdempotencyKeyParams{GameSessionID: sessionID, IdempotencyKey: &key})
	switch {
	case err == nil:
		if done.Kind != kind || hashDiffers(done.IdempotencyHash, hash) {
			return done, false, connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was already used for another change"))
		}
		return done, true, nil
	case errors.Is(err, pgx.ErrNoRows):
		return done, false, nil
	}
	return done, false, fmt.Errorf("find the event of this idempotency key: %w", err)
}

// publishVitalsOf tells the streams the new vitals of the characters.
func (s *Service) publishVitalsOf(campaignID string, vitals []*playv1.CharacterVitals) {
	for _, v := range vitals {
		s.publishVitals(campaignID, v)
	}
}

// TakeRest implements playv1connect.ResourceServiceHandler: the master's short or
// long rest.
func (s *Service) TakeRest(
	ctx context.Context,
	req *connect.Request[playv1.TakeRestRequest],
) (*connect.Response[playv1.TakeRestResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	kind, err := restKindName(req.Msg.GetKind())
	if err != nil {
		return nil, err
	}
	key, err := uuid.Parse(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key must be a UUID"))
	}
	keyText := key.String()
	hash := idem.Hash(req.Msg)

	var after []*playv1.CharacterVitals
	var repeated []string
	var touched []playdb.Encounter
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		after, repeated, touched = nil, nil, nil // a retry starts over
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		done, again, err := eventByKey(ctx, q, session.ID, keyText, eventRestTaken, hash)
		if err != nil {
			return err
		}
		if again { // a retry of a rest already taken
			var ev restEvent
			if err := json.Unmarshal(done.Payload, &ev); err != nil {
				return fmt.Errorf("read the rest event: %w", err)
			}
			repeated = append([]string{}, ev.Characters...)
			return nil
		}
		if err := activeCombatBlocks(ctx, q, session.ID); err != nil {
			return err
		}
		if _, after, err = s.vitals.TakeRest(ctx, tx, m.CampaignID, req.Msg); err != nil {
			return resourceError(err)
		}
		ids := make([]string, 0, len(after))
		for _, v := range after {
			ids = append(ids, v.GetCharacterId())
			// A combat in setup shows the character's state from the vitals.
			enc, err := s.touchCombatOf(ctx, q, session.ID, v.GetCharacterId(), nil)
			if err != nil {
				return err
			}
			if enc != nil && !slices.ContainsFunc(touched, func(e playdb.Encounter) bool { return e.ID == enc.ID }) {
				touched = append(touched, *enc)
			}
		}
		payload, err := json.Marshal(restEvent{Kind: kind, Characters: ids})
		if err != nil {
			return fmt.Errorf("encode the event payload: %w", err)
		}
		seq, err := q.NextSessionEventSeq(ctx, session.ID)
		if err != nil {
			return fmt.Errorf("next event number: %w", err)
		}
		if _, err := q.InsertSessionEvent(ctx, playdb.InsertSessionEventParams{
			GameSessionID: session.ID, Seq: seq, Kind: eventRestTaken, ActorUserID: &m.UserID,
			Payload: payload, IdempotencyKey: &keyText, IdempotencyHash: hash, CreatedAt: s.now(),
		}); err != nil {
			return fmt.Errorf("insert session event: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "take a rest", err)
	}
	if repeated != nil {
		// Nothing changed and nothing goes out: answer with the vitals as they are now.
		res := &playv1.TakeRestResponse{}
		for _, id := range repeated {
			if v, err := s.vitals.GetVitals(ctx, m.CampaignID, id); err == nil {
				res.Vitals = append(res.Vitals, v)
			}
		}
		return connect.NewResponse(res), nil
	}
	s.publishVitalsOf(m.CampaignID, after)
	pctx, stop := afterCommit(ctx)
	defer stop()
	for _, enc := range touched {
		s.publishEncounterChanged(pctx, m.CampaignID, enc)
	}
	logging.Event(ctx, s.logger, "rest.taken", slog.String("kind", kind), slog.Int("characters", len(after)))
	return connect.NewResponse(&playv1.TakeRestResponse{Vitals: after}), nil
}

// SpendHitDice implements playv1connect.ResourceServiceHandler: a player spends one
// hit die on a short rest.
func (s *Service) SpendHitDice(
	ctx context.Context,
	req *connect.Request[playv1.SpendHitDiceRequest],
) (*connect.Response[playv1.SpendHitDiceResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	characterID, err := uuid.Parse(req.Msg.GetCharacterId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("character not found"))
	}
	key, err := uuid.Parse(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key must be a UUID"))
	}
	faces := int(req.Msg.GetFaces())
	if faces < 1 || faces > maxHitDie {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("faces must be the size of one of the character's hit dice"))
	}
	var in rollInput
	switch roll := req.Msg.GetRoll().(type) {
	case *playv1.SpendHitDiceRequest_RollInApp:
		if !roll.RollInApp {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_in_app must be true"))
		}
		in.inApp = true
	case *playv1.SpendHitDiceRequest_TypedFace:
		in.typed = int(roll.TypedFace)
		if in.typed < 1 || in.typed > faces {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("typed_face must be a face of the die"))
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set roll_in_app or typed_face"))
	}
	keyText, charText := key.String(), characterID.String()
	hash := idem.Hash(req.Msg)

	var ev hitDieEvent
	var after *playv1.CharacterVitals
	var repeated bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		ev, after, repeated = hitDieEvent{}, nil, false // a retry starts over
		q := s.queries.WithTx(tx)
		session, err := q.GetOpenGameSessionForUpdate(ctx, m.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoOpenSession()
		}
		if err != nil {
			return fmt.Errorf("lock the open session: %w", err)
		}
		done, again, err := eventByKey(ctx, q, session.ID, keyText, eventHitDiceSpent, hash)
		if err != nil {
			return err
		}
		if again {
			if done.CharacterID == nil || *done.CharacterID != charText {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was already used for another change"))
			}
			if err := json.Unmarshal(done.Payload, &ev); err != nil {
				return fmt.Errorf("read the hit die event: %w", err)
			}
			repeated = true
			return nil
		}
		// The character's player or the master. Another player's character is not theirs.
		current, err := s.vitals.GetVitalsTx(ctx, tx, m.CampaignID, charText)
		if err != nil {
			return err
		}
		if m.Role != authz.RoleMaster && (current.GetPlayerUserId() == "" || current.GetPlayerUserId() != m.UserID) {
			return connect.NewError(connect.CodePermissionDenied, errors.New("only the character's player or the master may do this"))
		}
		if err := activeCombatBlocks(ctx, q, session.ID); err != nil {
			return err
		}
		if err := s.mustRollThisWay(ctx, tx, m, in); err != nil {
			return err
		}
		face := in.typed
		if in.inApp {
			rolled, err := dice.Roll(s.roller, dice.Expr{Count: 1, Sides: faces})
			if err != nil {
				return fmt.Errorf("roll the hit die: %w", err)
			}
			face = rolled.Faces[0]
		}
		_, afterVitals, conMod, healed, err := s.vitals.SpendHitDie(ctx, tx, m.CampaignID, charText, faces, face)
		if err != nil {
			return resourceError(err)
		}
		after = afterVitals
		ev = hitDieEvent{Faces: clamp32(faces, 1, maxHitDie), Face: clamp32(face, 1, maxHitDie), Modifier: clamp32(conMod, -maxHitDie, maxHitDie), Healed: clamp32(healed, 0, 1<<20), Physical: !in.inApp} //nolint:mnd // a bound far above any hit points
		payload, err := json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("encode the event payload: %w", err)
		}
		seq, err := q.NextSessionEventSeq(ctx, session.ID)
		if err != nil {
			return fmt.Errorf("next event number: %w", err)
		}
		if _, err := q.InsertSessionEvent(ctx, playdb.InsertSessionEventParams{
			GameSessionID: session.ID, Seq: seq, Kind: eventHitDiceSpent, ActorUserID: &m.UserID, CharacterID: &charText,
			Payload: payload, IdempotencyKey: &keyText, IdempotencyHash: hash, CreatedAt: s.now(),
		}); err != nil {
			return fmt.Errorf("insert session event: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "spend a hit die", err)
	}
	if repeated {
		current, err := s.vitals.GetVitals(ctx, m.CampaignID, charText)
		if err != nil {
			return nil, s.dbError(ctx, "get vitals", err)
		}
		return connect.NewResponse(spendResponse(current, ev)), nil
	}
	s.publishVitals(m.CampaignID, after)
	logging.Event(ctx, s.logger, "hit_dice.spent", slog.String("character_id", charText), slog.Int("faces", faces))
	return connect.NewResponse(spendResponse(after, ev)), nil
}

// maxHitDie is the largest hit die there is (the d12), a bound for the faces a request names.
const maxHitDie = 12

func spendResponse(v *playv1.CharacterVitals, ev hitDieEvent) *playv1.SpendHitDiceResponse {
	return &playv1.SpendHitDiceResponse{Vitals: v, Face: ev.Face, ConstitutionModifier: ev.Modifier, Healed: ev.Healed}
}
