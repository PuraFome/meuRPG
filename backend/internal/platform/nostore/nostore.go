// Package nostore marks Connect responses as not cacheable.
//
// Every response of a service that describes the caller's data (their
// characters, their game sessions) must carry `Cache-Control: no-store`, so
// neither the browser nor a proxy keeps a copy (docs/privacy.md). The
// campaigns module has its own copy of this interceptor from before this
// package existed.
package nostore

import (
	"context"
	"errors"

	"connectrpc.com/connect"
)

// Interceptor returns a Connect interceptor that marks every response,
// errors included, `Cache-Control: no-store`. It uses Set, so a response
// that already says so (identity's unauthenticated error does) is not sent
// the header twice. Mount it first, so it also covers the errors of the
// interceptors after it.
func Interceptor() connect.Interceptor {
	return interceptor{}
}

type interceptor struct{}

// WrapUnary implements connect.Interceptor.
func (interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if req.Spec().IsClient {
			return next(ctx, req)
		}
		res, err := next(ctx, req)
		if err != nil {
			if connectErr, ok := errors.AsType[*connect.Error](err); ok {
				connectErr.Meta().Set("Cache-Control", "no-store")
			}
			return nil, err
		}
		res.Header().Set("Cache-Control", "no-store")
		return res, nil
	}
}

// WrapStreamingClient implements connect.Interceptor; clients pass through.
func (interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements connect.Interceptor.
func (interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		conn.ResponseHeader().Set("Cache-Control", "no-store")
		return next(ctx, conn)
	}
}
