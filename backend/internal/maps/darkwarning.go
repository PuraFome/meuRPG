package maps

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
)

// ListCharactersInTheDark implements mapsv1connect.MapServiceHandler.
func (s *Service) ListCharactersInTheDark(
	_ context.Context,
	_ *connect.Request[mapsv1.ListCharactersInTheDarkRequest],
) (*connect.Response[mapsv1.ListCharactersInTheDarkResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not yet"))
}
