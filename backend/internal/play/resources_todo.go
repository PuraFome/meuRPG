package play

import (
	"context"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

func (s *Service) UseLayOnHands(context.Context, *connect.Request[playv1.UseLayOnHandsRequest]) (*connect.Response[playv1.UseLayOnHandsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Service) CreateSpellSlot(context.Context, *connect.Request[playv1.CreateSpellSlotRequest]) (*connect.Response[playv1.CreateSpellSlotResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Service) ConvertSpellSlot(context.Context, *connect.Request[playv1.ConvertSpellSlotRequest]) (*connect.Response[playv1.ConvertSpellSlotResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Service) GiveBardicInspiration(context.Context, *connect.Request[playv1.GiveBardicInspirationRequest]) (*connect.Response[playv1.GiveBardicInspirationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Service) AnswerBardicInspiration(context.Context, *connect.Request[playv1.AnswerBardicInspirationRequest]) (*connect.Response[playv1.AnswerBardicInspirationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
