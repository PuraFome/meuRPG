package progression

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// ListTreasuresToConvert implements progressionv1connect.ProgressionServiceHandler:
// the found treasures no award converted yet, for "Voltar à cidade" (MR-041).
// Only the master reads the list: it is the input of an award. The treasure
// points are the maps' tables, so the list comes through the Treasures
// interface; the finders' names are the party's.
func (s *Service) ListTreasuresToConvert(
	ctx context.Context,
	req *connect.Request[progressionv1.ListTreasuresToConvertRequest],
) (*connect.Response[progressionv1.ListTreasuresToConvertResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	found, err := s.treasures.ListUnconverted(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the treasures to convert", err)
	}
	total := int32(len(found)) //nolint:gosec // a campaign's treasures
	found = found[:min(len(found), maxAwardTreasures)]
	var characterIDs []string
	for _, t := range found {
		characterIDs = append(characterIDs, t.FinderIDs...)
	}
	names, err := s.party.Names(ctx, m.CampaignID, characterIDs)
	if err != nil {
		return nil, s.dbError(ctx, "read the finders' names", fmt.Errorf("treasures: %w", err))
	}
	res := &progressionv1.ListTreasuresToConvertResponse{Total: total}
	for _, t := range found {
		v := &progressionv1.TreasureToConvert{
			PointId: t.PointID, MapId: t.MapID, MapName: t.MapName, Name: t.Name, ValuePo: t.ValuePO,
			FoundAt: timestamppb.New(t.FoundAt), FoundInSession: t.InSession,
		}
		// The maps module does not read the sheets: the order by name is ours.
		finders := slices.SortedFunc(slices.Values(t.FinderIDs), func(a, b string) int {
			return cmp.Or(strings.Compare(names[a], names[b]), strings.Compare(a, b))
		})
		for _, id := range finders {
			v.FoundBy = append(v.FoundBy, &progressionv1.TreasureFinder{CharacterId: id, CharacterName: names[id]})
		}
		res.Treasures = append(res.Treasures, v)
	}
	return connect.NewResponse(res), nil
}
