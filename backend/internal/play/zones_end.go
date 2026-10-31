package play

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// EndSpellZone implements playv1connect.CombatServiceHandler: the master ends the area a spell keeps on the map
// (the caster's concentration ended outside the app). The line says the master ended it.
func (s *Service) EndSpellZone(
	ctx context.Context,
	req *connect.Request[playv1.EndSpellZoneRequest],
) (*connect.Response[playv1.EndSpellZoneResponse], error) {
	m, key, encID, err := masterCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetIdempotencyKey(), req.Msg.GetEncounterId())
	if err != nil {
		return nil, err
	}
	zoneID, err := parseCombatID(req.Msg.GetZoneId(), "zone")
	if err != nil {
		return nil, err
	}
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventLastingEnded, encounterID: encID}, func(c *combatTx) (any, error) {
		return s.endZoneIn(ctx, c, zoneID)
	})
	if err != nil {
		return nil, s.dbError(ctx, "end a spell zone", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, true)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.EndSpellZoneResponse{Encounter: out}), nil
}

// endZoneIn is the transaction of EndSpellZone.
func (s *Service) endZoneIn(ctx context.Context, c *combatTx, zoneID string) (any, error) {
	if err := notEnded(c.enc); err != nil {
		return nil, err
	}
	row, err := c.q.GetSpellZone(ctx, playdb.GetSpellZoneParams{ID: zoneID, EncounterID: c.enc.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("zone not found"))
	}
	if err != nil {
		return nil, fmt.Errorf("find the spell zone: %w", err)
	}
	z, _ := readZone(row)
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return nil, err
	}
	if err := c.q.DeleteSpellZone(ctx, zoneID); err != nil {
		return nil, fmt.Errorf("end the spell zone: %w", err)
	}
	c.zoneChanged(z)
	if err := s.closeZoneWindows(ctx, c, zoneID, endByMaster); err != nil {
		return nil, err
	}
	if err := touch(ctx, c); err != nil {
		return nil, err
	}
	if z == nil { // a zone of a spell the catalog no longer knows: gone, with no line to name it
		z = &zone{row: row}
	}
	return zoneEvent(z, c.enc.Round, ownerHidden(cs, z), "ended", endByMaster), nil
}
