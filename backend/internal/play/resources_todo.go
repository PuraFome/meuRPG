package play

import (
	"context"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

func (s *Service) AnswerBardicInspiration(context.Context, *connect.Request[playv1.AnswerBardicInspirationRequest]) (*connect.Response[playv1.AnswerBardicInspirationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
