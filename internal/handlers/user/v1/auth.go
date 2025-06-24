package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/spotdemo4/ts-server/internal/auth"
	userv1 "github.com/spotdemo4/ts-server/internal/connect/user/v1"
	"github.com/spotdemo4/ts-server/internal/connect/user/v1/userv1connect"
	"github.com/spotdemo4/ts-server/internal/models"
	"github.com/spotdemo4/ts-server/internal/putil"
	"github.com/stephenafamo/bob"
)

type AuthHandler struct {
	db       *bob.DB
	webAuthn *webauthn.WebAuthn
	auth     *auth.Auth

	sessions *map[string]*webauthn.SessionData
	mu       sync.Mutex
}

func (h *AuthHandler) Login(ctx context.Context, req *connect.Request[userv1.LoginRequest]) (*connect.Response[userv1.LoginResponse], error) {
	// Get user
	user, err := h.auth.GetUserByName(ctx, req.Msg.Username)
	if err != nil {
		return nil, putil.CheckNotFound(err)
	}

	// Check password
	if !user.Validate(req.Msg.Password) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("invalid username or password"))
	}

	// Create response
	res := connect.NewResponse(&userv1.LoginResponse{
		Token: user.Token(time.Now().Add(8 * time.Hour)),
	})
	res.Header().Set("Set-Cookie", user.Cookie(8*time.Hour).String())

	return res, nil
}

func (h *AuthHandler) SignUp(ctx context.Context, req *connect.Request[userv1.SignUpRequest]) (*connect.Response[userv1.SignUpResponse], error) {
	// Check if user already exists
	_, err := h.auth.GetUserByName(ctx, req.Msg.Username)
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

	// Create new user
	err = h.auth.NewUser(ctx, auth.NewUserParams{
		Username: req.Msg.Username,
		Password: req.Msg.Password,
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
	user, err := h.auth.GetUserByName(ctx, req.Msg.Username)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get options for user
	options, session, err := h.webAuthn.BeginLogin(user)
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
	user, err := h.auth.GetUserByName(ctx, req.Msg.Username)
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
	credential, err := h.webAuthn.ValidateLogin(user, *session, parsedResponse)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get cred
	cred, err := models.Credentials.Query(
		models.SelectWhere.Credentials.CredID.EQ(string(credential.ID)),
		models.SelectWhere.Credentials.UserID.EQ(user.ID),
	).One(ctx, h.db)

	// Update cred
	err = cred.Update(ctx, h.db, &models.CredentialSetter{
		LastUsed:  putil.ToPointer(time.Now()),
		SignCount: putil.ToPointer(int32(credential.Authenticator.SignCount)),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create response
	resp := connect.NewResponse(&userv1.FinishPasskeyLoginResponse{
		Token: user.Token(time.Now().Add(8 * time.Hour)),
	})
	resp.Header().Set("Set-Cookie", user.Cookie(8*time.Hour).String())

	return resp, nil
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

func NewAuth(db *bob.DB, auth *auth.Auth, webauth *webauthn.WebAuthn, interceptors connect.Option) (string, http.Handler) {
	sd := map[string]*webauthn.SessionData{}
	return userv1connect.NewAuthServiceHandler(
		&AuthHandler{
			db:       db,
			webAuthn: webauth,
			auth:     auth,

			sessions: &sd,
			mu:       sync.Mutex{},
		},
		interceptors,
	)
}
