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
	"github.com/aarondl/opt/omit"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stephenafamo/bob"

	"github.com/spotdemo4/ts-server/internal/app"
	"github.com/spotdemo4/ts-server/internal/auth"
	"github.com/spotdemo4/ts-server/internal/bob/models"
	userv1 "github.com/spotdemo4/ts-server/internal/connect/user/v1"
	"github.com/spotdemo4/ts-server/internal/connect/user/v1/userv1connect"
	"github.com/spotdemo4/ts-server/internal/putil"
)

const DefaultCookiDuration = time.Hour * 8 // 8 hours

type AuthHandler struct {
	db   *bob.DB
	auth *auth.Auth

	sessions *map[string]*webauthn.SessionData
	mu       sync.Mutex
}

func (h *AuthHandler) Login(
	ctx context.Context,
	req *connect.Request[userv1.LoginRequest],
) (*connect.Response[userv1.LoginResponse], error) {
	// Get user
	user, err := h.auth.GetUserByName(ctx, req.Msg.GetUsername())
	if err != nil {
		return nil, putil.CheckNotFound(err)
	}

	// Check password
	if !user.Validate(req.Msg.GetPassword()) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("invalid username or password"))
	}

	// Create response
	res := connect.NewResponse(&userv1.LoginResponse{
		Token: user.Token(time.Now().Add(DefaultCookiDuration)),
	})
	res.Header().Set("Set-Cookie", user.Cookie(DefaultCookiDuration).String())

	return res, nil
}

func (h *AuthHandler) SignUp(
	ctx context.Context,
	req *connect.Request[userv1.SignUpRequest],
) (*connect.Response[userv1.SignUpResponse], error) {
	// Check if user already exists
	_, err := h.auth.GetUserByName(ctx, req.Msg.GetUsername())
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("user already exists"))
	}

	// Check if confirmation passwords match
	if req.Msg.GetPassword() != req.Msg.GetConfirmPassword() {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("passwords do not match"))
	}

	// Create new user
	err = h.auth.NewUser(ctx, auth.NewUserParams{
		Username: req.Msg.GetUsername(),
		Password: req.Msg.GetPassword(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&userv1.SignUpResponse{})
	return res, nil
}

func (h *AuthHandler) Logout(
	_ context.Context,
	_ *connect.Request[userv1.LogoutRequest],
) (*connect.Response[userv1.LogoutResponse], error) {
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

func (h *AuthHandler) BeginPasskeyLogin(
	ctx context.Context,
	req *connect.Request[userv1.BeginPasskeyLoginRequest],
) (*connect.Response[userv1.BeginPasskeyLoginResponse], error) {
	// Get user
	user, err := h.auth.GetUserByName(ctx, req.Msg.GetUsername())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get options for user
	options, session, err := h.auth.Web.BeginLogin(user)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Turn the options into json
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Set session for validation later
	h.setSession(req.Msg.GetUsername(), session)

	return connect.NewResponse(&userv1.BeginPasskeyLoginResponse{
		OptionsJson: string(optionsJSON),
	}), nil
}

func (h *AuthHandler) FinishPasskeyLogin(
	ctx context.Context,
	req *connect.Request[userv1.FinishPasskeyLoginRequest],
) (*connect.Response[userv1.FinishPasskeyLoginResponse], error) {
	// Get user
	user, err := h.auth.GetUserByName(ctx, req.Msg.GetUsername())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Get the session data previously set
	session, err := h.getSession(req.Msg.GetUsername())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Parse the attestation response
	parsedResponse, err := protocol.ParseCredentialRequestResponseBytes([]byte(req.Msg.GetAttestation()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Validate the login
	credential, err := h.auth.Web.ValidateLogin(user, *session, parsedResponse)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get cred
	cred, err := models.Credentials.Query(
		models.SelectWhere.Credentials.CredID.EQ(string(credential.ID)),
		models.SelectWhere.Credentials.UserID.EQ(user.ID),
	).One(ctx, h.db)
	if err != nil {
		return nil, putil.CheckNotFound(err)
	}

	// Update cred
	err = cred.Update(ctx, h.db, &models.CredentialSetter{
		LastUsed:  omit.From(time.Now()),
		SignCount: omit.From(int32(credential.Authenticator.SignCount)),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create response
	resp := connect.NewResponse(&userv1.FinishPasskeyLoginResponse{
		Token: user.Token(time.Now().Add(DefaultCookiDuration)),
	})
	resp.Header().Set("Set-Cookie", user.Cookie(DefaultCookiDuration).String())

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

func NewAuth(app *app.App, interceptors connect.Option) (string, http.Handler) {
	sd := map[string]*webauthn.SessionData{}
	return userv1connect.NewAuthServiceHandler(
		&AuthHandler{
			db:   app.DB,
			auth: app.Auth,

			sessions: &sd,
			mu:       sync.Mutex{},
		},
		interceptors,
	)
}
