package ratelimit

import (
	"context"

	"connectrpc.com/connect"
)

// UserFunc says who is calling: the user's ID, or false for a caller with no
// session (identity's UserID, which fails then). Such a call is not limited
// here: the IP limit (Middleware) already covered it, and every handler
// refuses it anyway.
type UserFunc func(ctx context.Context) (userID string, ok bool)

// Interceptor limits the signed-in user's Connect calls: one token per
// unary call and one per stream opened (a stream's messages are the
// server's, not the user's). It must sit after the session interceptor,
// which is what makes the user known.
func Interceptor(l *Limiter, who UserFunc, n *Notifier) connect.Interceptor {
	return &interceptor{limiter: l, who: who, notify: n}
}

type interceptor struct {
	limiter *Limiter
	who     UserFunc
	notify  *Notifier
}

// check returns the error to answer with, or nil.
func (i *interceptor) check(ctx context.Context) error {
	userID, ok := i.who(ctx)
	if !ok {
		return nil
	}
	if allowed, wait := i.limiter.Allow(userID); !allowed {
		i.notify.Hit(ctx) // the context carries the user's ID on the line
		return RPCError(wait)
	}
	return nil
}

// WrapUnary implements connect.Interceptor.
func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if req.Spec().IsClient {
			return next(ctx, req)
		}
		if err := i.check(ctx); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

// WrapStreamingClient implements connect.Interceptor; clients pass through.
func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements connect.Interceptor.
func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := i.check(ctx); err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

// Chain runs the interceptors in order, the first outermost, as one
// interceptor. Services take a single session interceptor, and this is how
// the limit is made to run right after it.
func Chain(interceptors ...connect.Interceptor) connect.Interceptor {
	return chain(interceptors)
}

type chain []connect.Interceptor

func (c chain) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	for i := len(c) - 1; i >= 0; i-- {
		next = c[i].WrapUnary(next)
	}
	return next
}

func (c chain) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	for i := len(c) - 1; i >= 0; i-- {
		next = c[i].WrapStreamingClient(next)
	}
	return next
}

func (c chain) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	for i := len(c) - 1; i >= 0; i-- {
		next = c[i].WrapStreamingHandler(next)
	}
	return next
}
