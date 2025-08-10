package interceptors

import (
	"context"
	"errors"
	"sync"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RatelimitInterceptor struct {
	visitors map[string]*visitor
	mu       sync.Mutex
}

func NewRateLimitInterceptor() *RatelimitInterceptor {
	rl := &RatelimitInterceptor{
		visitors: make(map[string]*visitor),
		mu:       sync.Mutex{},
	}

	go rl.cleanupVisitors()

	return rl
}

func (i *RatelimitInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	// Same as previous UnaryInterceptorFunc.
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		// Check if the request is from a client
		if req.Spec().IsClient {
			return next(ctx, req)
		}

		// Get user agent
		limiter := i.getVisitor(req.Header().Get("User-Agent"))
		if !limiter.Allow() {
			return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("rate limit exceeded"))
		}

		return next(ctx, req)
	})
}

func (*RatelimitInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return connect.StreamingClientFunc(func(
		ctx context.Context,
		spec connect.Spec,
	) connect.StreamingClientConn {
		return next(ctx, spec)
	})
}

func (i *RatelimitInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) error {
		// Get user agent
		limiter := i.getVisitor(conn.RequestHeader().Get("User-Agent"))
		if !limiter.Allow() {
			return connect.NewError(connect.CodeResourceExhausted, errors.New("rate limit exceeded"))
		}

		return next(ctx, conn)
	})
}

const RateLimit = 3 // Rate Limit up to 3 requests per second

// getVisitor retrieves the visitor for the given user agent, or creates a new one if it doesn't exist.
func (i *RatelimitInterceptor) getVisitor(userAgent string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	v, exists := i.visitors[userAgent]
	if !exists {
		limiter := rate.NewLimiter(1, RateLimit)
		// Include the current time when creating a new visitor.
		i.visitors[userAgent] = &visitor{limiter, time.Now()}
		return limiter
	}

	// Update the last seen time for the visitor.
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors checks the map for visitors that haven't been seen for more than 3 minutes.
func (i *RatelimitInterceptor) cleanupVisitors() {
	for {
		time.Sleep(time.Minute)

		i.mu.Lock()
		for ip, v := range i.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(i.visitors, ip)
			}
		}
		i.mu.Unlock()
	}
}
