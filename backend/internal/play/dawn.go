package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The dawn of the magic items (SRD 5.1): the master's Amanhecer (ChargesService.Dawn) and the long rest with
// TakeRestRequest.restore_dawn both give the campaign's items their dawn through ItemKeeper.RestoreDawn, which holds
// the numbers; this file rolls the dice, writes one session event for each item that regained charges and tells the
// streams. The dice are the app's, also on a table of real dice: the master rolls them for the table, as the app rolls
// the NPCs' dice (RN-18), and a typed face would need a field for every item.

// dawnEvent is the payload of dawn_taken: the items that regained charges, enough to answer the retry of a key.
type dawnEvent struct {
	Items []dawnEventItem `json:"items"`
}

type dawnEventItem struct {
	Character string `json:"character_id"`
	Entry     string `json:"entry_id"`
	Regained  int32  `json:"regained"`
}

// dawnRoll rolls the dice of an item's dawn with the app's roller.
func (s *Service) dawnRoll(count, sides, bonus int) (int32, error) {
	res, err := dice.Roll(s.roller, dice.Expr{Count: count, Sides: sides, Modifier: bonus})
	if err != nil {
		return 0, fmt.Errorf("roll the dice of the dawn: %w", err)
	}
	return clamp32(res.Total, 0, 1<<20), nil
}

// restoreDawn gives the items their dawn in tx and writes one item_charges_restored event for each item that regained
// charges (no key: the call's own event holds it), as a plain session event of the master's. It tells the streams that
// the characters' sheets changed.
func (s *Service) restoreDawn(ctx context.Context, tx pgx.Tx, q *playdb.Queries, h *hints, session playdb.GameSession, userID, campaignID string) ([]link.DawnItem, error) {
	items, err := s.vitals.RestoreDawn(ctx, tx, campaignID, s.dawnRoll)
	if err != nil {
		return nil, err
	}
	var told []string
	for _, it := range items {
		name, err := s.itemName(ctx, tx, campaignID, it.Item.GetKey())
		if err != nil {
			return nil, err
		}
		ev := itemUseEvent{
			Entry: it.Item.GetId(), Item: it.Item.GetKey(), Name: name, Amount: it.Regained, Left: it.Item.GetChargesLeft(), Max: it.Item.GetChargesMax(),
			Outcome: itemOutcomeRegained,
		}
		id := it.CharacterID
		if err := s.appendKeyedEvent(ctx, q, keyedEvent{sessionID: session.ID, kind: eventItemChargesRestored, userID: userID, characterID: &id}, ev); err != nil {
			return nil, err
		}
		told = append(told, id)
	}
	s.tellVitalsAfter(h, campaignID, told...)
	if len(items) > 0 {
		h.touch()
	}
	return items, nil
}

// Dawn implements playv1connect.ChargesServiceHandler: the master's Amanhecer.
func (s *Service) Dawn(ctx context.Context, req *connect.Request[playv1.DawnRequest]) (*connect.Response[playv1.DawnResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := uuid.Parse(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key must be a UUID"))
	}
	call := chargeCall{m: m, keyText: key.String(), hash: idem.Hash(req.Msg)}
	var res *playv1.DawnResponse
	repeated := false
	err = s.txOpen(ctx, call.m.CampaignID, func(tx pgx.Tx, h *hints, session playdb.GameSession) error {
		res, repeated = nil, false // a retry starts over
		var err error
		res, repeated, err = s.dawnTx(ctx, tx, h, session, call)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "take the dawn", err)
	}
	if !repeated {
		logging.Event(ctx, s.logger, "item_charges.dawn", slog.Int("items", len(res.GetRestored())))
	}
	return connect.NewResponse(res), nil
}

// dawnTx is the transaction of Dawn (the session is locked): retry, the dawn, its events and the call's own.
func (s *Service) dawnTx(ctx context.Context, tx pgx.Tx, h *hints, session playdb.GameSession, call chargeCall) (*playv1.DawnResponse, bool, error) {
	q := s.queries.WithTx(tx)
	done, again, err := eventByKey(ctx, q, session.ID, call.keyText, eventDawnTaken, call.hash)
	if err != nil {
		return nil, false, err
	}
	if again {
		res, err := s.dawnAgain(ctx, tx, h, call, done.Payload)
		return res, true, err
	}
	items, err := s.restoreDawn(ctx, tx, q, h, session, call.m.UserID, call.m.CampaignID)
	if err != nil {
		return nil, false, err
	}
	ev := dawnEvent{Items: make([]dawnEventItem, 0, len(items))}
	res := &playv1.DawnResponse{}
	for _, it := range items {
		ev.Items = append(ev.Items, dawnEventItem{Character: it.CharacterID, Entry: it.Item.GetId(), Regained: it.Regained})
		res.Restored = append(res.Restored, &playv1.DawnRestored{CharacterId: it.CharacterID, Item: it.Item, Regained: it.Regained})
	}
	err = s.appendKeyedEvent(ctx, q, keyedEvent{sessionID: session.ID, kind: eventDawnTaken, userID: call.m.UserID, key: call.keyText, hash: call.hash}, ev)
	if err != nil {
		return nil, false, err
	}
	h.touch() // a dawn that only counted a day for the figurines still wrote its event
	return res, false, nil
}

// dawnAgain answers the retry of a key found stored: the items the first call restored, as they stand now, and the
// hints the first call may have died before telling. It only reads, so it locks nothing.
func (s *Service) dawnAgain(ctx context.Context, tx pgx.Tx, h *hints, call chargeCall, payload []byte) (*playv1.DawnResponse, error) {
	var ev dawnEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return nil, fmt.Errorf("read the dawn event: %w", err)
	}
	res := &playv1.DawnResponse{}
	var told []string
	for _, it := range ev.Items {
		item, err := s.vitals.ChargedItemRead(ctx, tx, call.m.CampaignID, it.Character, it.Entry)
		if connect.CodeOf(err) == connect.CodeNotFound {
			continue // the character or the item left the sheet since
		}
		if err != nil {
			return nil, err
		}
		res.Restored = append(res.Restored, &playv1.DawnRestored{CharacterId: it.Character, Item: item, Regained: it.Regained})
		told = append(told, it.Character)
	}
	h.replay()
	s.tellVitalsAfter(h, call.m.CampaignID, told...)
	return res, nil
}
