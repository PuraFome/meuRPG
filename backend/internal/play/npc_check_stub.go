package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// RollNpcCheck implements playv1connect.PlayServiceHandler.
func (s *Service) RollNpcCheck(context.Context, *connect.Request[playv1.RollNpcCheckRequest]) (*connect.Response[playv1.RollNpcCheckResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not yet"))
}

// SkipNpcCheckAsks implements playv1connect.PlayServiceHandler.
func (s *Service) SkipNpcCheckAsks(context.Context, *connect.Request[playv1.SkipNpcCheckAsksRequest]) (*connect.Response[playv1.SkipNpcCheckAsksResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not yet"))
}

// ListNpcCheckPicker implements playv1connect.PlayServiceHandler.
func (s *Service) ListNpcCheckPicker(context.Context, *connect.Request[playv1.ListNpcCheckPickerRequest]) (*connect.Response[playv1.ListNpcCheckPickerResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not yet"))
}
