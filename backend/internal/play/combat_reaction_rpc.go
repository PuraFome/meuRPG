package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// AnswerReaction implements playv1connect.CombatServiceHandler.
func (s *Service) AnswerReaction(
	_ context.Context,
	_ *connect.Request[playv1.AnswerReactionRequest],
) (*connect.Response[playv1.AnswerReactionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not built yet"))
}

// ResolveConcentrationSave implements playv1connect.CombatServiceHandler.
func (s *Service) ResolveConcentrationSave(
	_ context.Context,
	_ *connect.Request[playv1.ResolveConcentrationSaveRequest],
) (*connect.Response[playv1.ResolveConcentrationSaveResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not built yet"))
}
