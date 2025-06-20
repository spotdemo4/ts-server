package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/spotdemo4/ts-server/internal/auth"
	userv1 "github.com/spotdemo4/ts-server/internal/connect/user/v1"
	"github.com/spotdemo4/ts-server/internal/connect/user/v1/userv1connect"
	"github.com/spotdemo4/ts-server/internal/interceptors"
	"github.com/spotdemo4/ts-server/internal/sqlc"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db       *sqlc.Queries
	webAuthn *webauthn.WebAuthn
	key      []byte
	name     string

	sessions *map[string]*webauthn.SessionData
	mu       sync.Mutex
}

func (h *AuthHandler) Login(ctx context.Context, req *connect.Request[userv1.LoginRequest]) (*connect.Response[userv1.LoginResponse], error) {
	// Get user
	user, err := h.db.GetUserbyUsername(ctx, req.Msg.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}

		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Msg.Password)); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("invalid username or password"))
	}

	// Create JWT
	token, cookie, err := h.createJWT(user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create response
	res := connect.NewResponse(&userv1.LoginResponse{
		Token: token,
	})
	res.Header().Set("Set-Cookie", cookie.String())

	return res, nil
}

func (h *AuthHandler) SignUp(ctx context.Context, req *connect.Request[userv1.SignUpRequest]) (*connect.Response[userv1.SignUpResponse], error) {
	// Get user
	_, err := h.db.GetUserbyUsername(ctx, req.Msg.Username)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("user already exists"))
	}

	// Check if confirmation passwords match
	if req.Msg.Password != req.Msg.ConfirmPassword {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("passwords do not match"))
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Msg.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create user
	_, err = h.db.InsertUser(ctx, sqlc.InsertUserParams{
		Username:   req.Msg.Username,
		Password:   string(hash),
		WebauthnID: uuid.New().String(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&userv1.SignUpResponse{})
	return res, nil
}

func (h *AuthHandler) Logout(_ context.Context, _ *connect.Request[userv1.LogoutRequest]) (*connect.Response[userv1.LogoutResponse], error) {
	// Clear cookie
	cookie := http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}

	res := connect.NewResponse(&userv1.LogoutResponse{})
	res.Header().Set("Set-Cookie", cookie.String())

	return res, nil
}

func (h *AuthHandler) BeginPasskeyLogin(ctx context.Context, req *connect.Request[userv1.BeginPasskeyLoginRequest]) (*connect.Response[userv1.BeginPasskeyLoginResponse], error) {
	// Get user
	pUser, _, err := h.getPasskeyUser(ctx, req.Msg.Username)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get options for user
	options, session, err := h.webAuthn.BeginLogin(pUser)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Turn the options into json
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Set session for validation later
	h.setSession(req.Msg.Username, session)

	return connect.NewResponse(&userv1.BeginPasskeyLoginResponse{
		OptionsJson: string(optionsJSON),
	}), nil
}

func (h *AuthHandler) FinishPasskeyLogin(ctx context.Context, req *connect.Request[userv1.FinishPasskeyLoginRequest]) (*connect.Response[userv1.FinishPasskeyLoginResponse], error) {
	// Get user
	pUser, userID, err := h.getPasskeyUser(ctx, req.Msg.Username)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Get the session data previously set
	session, err := h.getSession(req.Msg.Username)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Parse the attestation response
	parsedResponse, err := protocol.ParseCredentialRequestResponseBytes([]byte(req.Msg.Attestation))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Validate the login
	credential, err := h.webAuthn.ValidateLogin(pUser, *session, parsedResponse)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Update the credential in the database
	lastUsed := time.Now()
	signCount := int64(credential.Authenticator.SignCount)
	err = h.db.UpdateCredential(ctx, sqlc.UpdateCredentialParams{
		// set
		LastUsed:  &lastUsed,
		SignCount: &signCount,

		// where
		ID:     string(credential.ID),
		UserID: userID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create JWT
	token, cookie, err := h.createJWT(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create response
	resp := connect.NewResponse(&userv1.FinishPasskeyLoginResponse{
		Token: token,
	})
	resp.Header().Set("Set-Cookie", cookie.String())

	return resp, nil
}

func (h *AuthHandler) getPasskeyUser(ctx context.Context, username string) (*auth.User, int64, error) {
	user, err := h.db.GetUserbyUsername(ctx, username)
	if err != nil {
		return nil, 0, err
	}

	creds, err := h.db.GetCredentials(ctx, user.ID)
	if err != nil {
		return nil, 0, err
	}

	webCreds := auth.NewCreds(creds)
	webUser := auth.NewUser(user.WebauthnID, user.Username, webCreds)

	return &webUser, user.ID, nil
}

func (h *AuthHandler) getSession(username string) (*webauthn.SessionData, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	session, ok := (*h.sessions)[username]
	if !ok {
		return nil, errors.New("session does not exist")
	}

	delete(*h.sessions, username)
	return session, nil
}

func (h *AuthHandler) setSession(username string, data *webauthn.SessionData) {
	h.mu.Lock()
	defer h.mu.Unlock()

	(*h.sessions)[username] = data
}

func (h *AuthHandler) createJWT(userid int64) (string, *http.Cookie, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:  h.name,
		Subject: strconv.Itoa(int(userid)),
		IssuedAt: &jwt.NumericDate{
			Time: time.Now(),
		},
		ExpiresAt: &jwt.NumericDate{
			Time: time.Now().Add(time.Hour * 24),
		},
	})

	tokenString, err := token.SignedString(h.key)
	if err != nil {
		return "", nil, err
	}

	cookie := http.Cookie{
		Name:     "token",
		Value:    tokenString,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}

	return tokenString, &cookie, nil
}

func NewAuthHandler(vi *validate.Interceptor, db *sqlc.Queries, webauth *webauthn.WebAuthn, name string, key string) (string, http.Handler) {
	interceptors := connect.WithInterceptors(vi, interceptors.NewRateLimitInterceptor(key))

	sd := map[string]*webauthn.SessionData{}
	return userv1connect.NewAuthServiceHandler(
		&AuthHandler{
			db:       db,
			webAuthn: webauth,
			name:     name,
			key:      []byte(key),

			sessions: &sd,
			mu:       sync.Mutex{},
		},
		interceptors,
	)
}
