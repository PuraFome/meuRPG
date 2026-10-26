package maps

import (
	"context"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/platform/uid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// The master's light warning: which player characters see nothing where they stand.
// It is the fog's own question (vision.Lit.InTheDark, over the scene newSight builds
// for GetMapVision and the players' views), so every light of the map counts, a
// hidden NPC's included. The answer is the master's alone (RN-10): it is worked out
// from lights a player cannot see.

// ListCharactersInTheDark implements mapsv1connect.MapServiceHandler.
func (s *Service) ListCharactersInTheDark(
	ctx context.Context,
	req *connect.Request[mapsv1.ListCharactersInTheDarkRequest],
) (*connect.Response[mapsv1.ListCharactersInTheDarkResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := uid.Canonical(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	row, err := s.campaignMap(ctx, s.queries, m.CampaignID, mapID)
	if err != nil {
		return nil, s.dbError(ctx, "find the map", err)
	}
	out := &mapsv1.ListCharactersInTheDarkResponse{}
	if !fogged(row) {
		return connect.NewResponse(out), nil // with no fog everyone sees everything
	}
	size, err := s.queries.GetMapGrid(ctx, mapsdb.GetMapGridParams{CampaignID: m.CampaignID, ID: mapID})
	if err != nil {
		return nil, s.dbError(ctx, "read the map's grid", err)
	}
	in := fogInputOfRow(row, size.ImageWidth, size.ImageHeight)
	if !in.g.Valid() {
		return connect.NewResponse(out), nil
	}
	points, err := s.queries.ListMapPoints(ctx, mapID)
	if err != nil {
		return nil, s.dbError(ctx, "list the points", err)
	}
	tokens, err := s.queries.ListMapTokens(ctx, mapID)
	if err != nil {
		return nil, s.dbError(ctx, "list the tokens", err)
	}
	sg, err := s.newSight(ctx, nil, in, points, tokens)
	if err != nil {
		return nil, s.dbError(ctx, "work out the light", err)
	}
	dark := sg.inTheDark()
	if len(dark) == 0 {
		return connect.NewResponse(out), nil
	}
	ids := make([]string, 0, len(dark))
	for id := range dark {
		ids = append(ids, id)
	}
	names, order, err := s.characterNames(ctx, m.CampaignID, ids)
	if err != nil {
		return nil, s.dbError(ctx, "read the characters", err)
	}
	for _, id := range order { // the party in the order the characters module lists it
		sq := dark[id]
		out.Characters = append(out.Characters, &mapsv1.CharacterInTheDark{
			CharacterId: id, CharacterName: names[id], Col: conv.Int32(sq.Col), Row: conv.Int32(sq.Row),
			Cause: mapsv1.DarknessCause_DARKNESS_CAUSE_NO_LIGHT_NO_DARKVISION,
		})
	}
	return connect.NewResponse(out), nil
}

// inTheDark is the living characters of the party that stand on the map and see
// nothing where they stand, with their squares, by character ID.
func (sg *sight) inTheDark() map[string]grid.Square {
	out := map[string]grid.Square{}
	for _, m := range sg.members {
		sq, on := sg.stands[m.CharacterID]
		if m.Dead || !on {
			continue
		}
		if sg.entry.lit.InTheDark(vision.Viewer{At: sq, Senses: m.Senses}) {
			out[m.CharacterID] = sq
		}
	}
	return out
}
