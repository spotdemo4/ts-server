package interceptors

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	"github.com/spotdemo4/ts-server/internal/auth"
)

type AuthInterceptor struct {
	auth *auth.Auth
}

const CookieTokenName = "token"

func NewAuthInterceptor(auth *auth.Auth) *AuthInterceptor {
	return &AuthInterceptor{
		auth: auth,
	}
}

func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	// Same as previous UnaryInterceptorFunc.
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		// Check if the request is from a client
		if req.Spec().IsClient {
			return next(ctx, req)
		}

		// Check if the request contains a valid cookie token
		cookies := getCookies(req.Header().Get("Cookie"))
		for _, cookie := range cookies {
			if cookie.Name == CookieTokenName {
				user, err := i.auth.GetUserFromToken(cookie.Value)
				if err == nil {
					return next(i.auth.NewContext(ctx, user), req)
				}
			}
		}

		// Check if the request contains a valid authorization bearer token
		authorization := req.Header().Get("Authorization")
		if authorization != "" && len(authorization) > 7 {
			user, err := i.auth.GetUserFromToken(authorization[7:])
			if err == nil {
				return next(i.auth.NewContext(ctx, user), req)
			}
		}

		// Return without setting the context
		return next(ctx, req)
	})
}

func (*AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return connect.StreamingClientFunc(func(
		ctx context.Context,
		spec connect.Spec,
	) connect.StreamingClientConn {
		return next(ctx, spec)
	})
}

func (i *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) error {
		// Check if the request contains a valid cookie token
		cookies := getCookies(conn.RequestHeader().Get("Cookie"))
		for _, cookie := range cookies {
			if cookie.Name == CookieTokenName {
				user, err := i.auth.GetUserFromToken(cookie.Value)
				if err == nil {
					return next(i.auth.NewContext(ctx, user), conn)
				}
			}
		}

		// Check if the request contains a valid authorization bearer token
		authorization := conn.RequestHeader().Get("Authorization")
		if authorization != "" && len(authorization) > 7 {
			user, err := i.auth.GetUserFromToken(authorization[7:])
			if err == nil {
				return next(i.auth.NewContext(ctx, user), conn)
			}
		}

		// Return without setting the context
		return next(ctx, conn)
	})
}

func getCookies(rawCookies string) []*http.Cookie {
	header := http.Header{}
	header.Add("Cookie", rawCookies)
	request := http.Request{Header: header}

	return request.Cookies()
}
