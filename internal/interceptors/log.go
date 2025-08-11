package interceptors

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
)

type LoggingInterceptor struct {
	log *slog.Logger
}

func NewLoggingInterceptor(log *slog.Logger) *LoggingInterceptor {
	return &LoggingInterceptor{
		log,
	}
}

func (i *LoggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		i.log.Info("connect",
			"type", "unary",
			"client", req.Spec().IsClient,
			"procedure", req.Spec().Procedure,
			"peer", req.Peer(),
		)

		return next(ctx, req)
	})
}

func (i *LoggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return connect.StreamingClientFunc(func(
		ctx context.Context,
		spec connect.Spec,
	) connect.StreamingClientConn {
		i.log.Info("connect",
			"type", "streaming",
			"client", true,
			"procedure", spec.Procedure,
		)

		return next(ctx, spec)
	})
}

func (i *LoggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) error {
		i.log.Info("connect",
			"type", "streaming",
			"client", false,
			"procedure", conn.Spec().Procedure,
			"peer", conn.Peer(),
		)

		return next(ctx, conn)
	})
}
