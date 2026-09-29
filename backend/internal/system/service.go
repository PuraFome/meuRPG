// Package system implements meurpg.system.v1.SystemService: operational RPCs
// that are not tied to any game feature.
//
// It is also the smallest possible example of a Connect service in this
// codebase: the .proto contract lives in proto/meurpg/system/v1, `buf
// generate` turns it into Go types and an interface in backend/gen, and this
// package implements that interface.
package system

import (
	"context"

	"connectrpc.com/connect"

	systemv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/system/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/system/v1/systemv1connect"
)

// Service answers SystemService RPCs.
type Service struct {
	version string
	commit  string
}

// The compiler checks that Service implements the generated interface, so a
// contract change that breaks it fails the build instead of a request.
var _ systemv1connect.SystemServiceHandler = (*Service)(nil)

// NewService returns a Service that reports the given build information.
func NewService(version, commit string) *Service {
	return &Service{version: version, commit: commit}
}

// GetServerInfo returns the version and commit of the running binary.
func (s *Service) GetServerInfo(
	_ context.Context,
	_ *connect.Request[systemv1.GetServerInfoRequest],
) (*connect.Response[systemv1.GetServerInfoResponse], error) {
	return connect.NewResponse(&systemv1.GetServerInfoResponse{
		Version: s.version,
		Commit:  s.commit,
	}), nil
}
