package interceptors

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
)

func WithAuthRedirect(next http.Handler, key string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathItems := strings.Split(r.URL.Path, "/")

		if len(pathItems) < 2 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Check if the user is authenticated
		authenticated := false
		cookies := getCookies(r.Header.Get("Cookie"))
		for _, cookie := range cookies {
			if cookie.Name == "token" {
				subject, err := validateToken(cookie.Value, key)
				if err == nil {
					ctx, err := newUserContext(r.Context(), subject)
					if err == nil {
						r = r.WithContext(ctx)
						authenticated = true
					}
				}

				break
			}
		}

		switch pathItems[1] {

		case "auth":
			if authenticated {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)

		case "_app", "favicon.png", "icon.png":
			next.ServeHTTP(w, r)

		default:
			if authenticated {
				next.ServeHTTP(w, r)
				return
			}

			// Redirect if not authenticated
			pathRedir := url.QueryEscape(r.URL.Path)
			http.Redirect(w, r, fmt.Sprintf("/auth?redir=%s", pathRedir), http.StatusFound)
		}
	})
}

type AuthInterceptor struct {
	key string
}

func NewAuthInterceptor(key string) *AuthInterceptor {
	return &AuthInterceptor{
		key: key,
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
			if cookie.Name == "token" {
				subject, err := validateToken(cookie.Value, i.key)
				if err == nil {
					ctx, err = newUserContext(ctx, subject)
					if err == nil {
						return next(ctx, req)
					}
				}
			}
		}

		// Check if the request contains a valid authorization bearer token
		authorization := req.Header().Get("Authorization")
		if authorization != "" && len(authorization) > 7 {
			subject, err := validateToken(authorization[7:], i.key)
			if err == nil {
				ctx, err = newUserContext(ctx, subject)
				if err == nil {
					return next(ctx, req)
				}
			}
		}

		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("could not authenticate"),
		)
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
			if cookie.Name == "token" {
				subject, err := validateToken(cookie.Value, i.key)
				if err == nil {
					ctx, err = newUserContext(ctx, subject)
					if err == nil {
						return next(ctx, conn)
					}
				}
			}
		}

		// Check if the request contains a valid authorization bearer token
		authorization := conn.RequestHeader().Get("Authorization")
		if authorization != "" && len(authorization) > 7 {
			subject, err := validateToken(authorization[7:], i.key)
			if err == nil {
				ctx, err = newUserContext(ctx, subject)
				if err == nil {
					return next(ctx, conn)
				}
			}
		}

		return connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("could not authenticate"),
		)
	})
}

func getCookies(rawCookies string) []*http.Cookie {
	header := http.Header{}
	header.Add("Cookie", rawCookies)
	request := http.Request{Header: header}

	return request.Cookies()
}

func validateToken(tokenString string, key string) (subject string, err error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(key), nil
	})
	if err != nil {
		return "", err
	}

	switch {
	case token.Valid:
		subject, err := token.Claims.GetSubject()
		if err != nil {
			return "", err
		}

		return subject, nil

	case errors.Is(err, jwt.ErrTokenMalformed):
		log.Println("Token is malformed")
		return "", err

	case errors.Is(err, jwt.ErrSignatureInvalid):
		log.Println("Token signature is invalid")
		return "", err

	case errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet):
		log.Println("Token is expired or not valid yet")
		return "", err

	default:
		log.Println("Token is invalid")
		return "", err
	}
}

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type key int64

// userKey is the key for user.User values in Contexts. It is
// unexported; clients use user.NewContext and user.FromContext
// instead of using this key directly.
var userKey key

// newUserContext returns a new Context that carries value u.
func newUserContext(ctx context.Context, subject string) (context.Context, error) {
	id, err := strconv.Atoi(subject)
	if err != nil {
		return nil, err
	}

	return context.WithValue(ctx, userKey, int64(id)), nil
}

// getUserContext returns the User value stored in ctx, if any.
func GetUserContext(ctx context.Context) (int64, bool) {
	u, ok := ctx.Value(userKey).(int64)
	return u, ok
}
