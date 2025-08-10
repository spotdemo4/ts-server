package user

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stephenafamo/bob"

	"github.com/spotdemo4/ts-server/internal/app"
	"github.com/spotdemo4/ts-server/internal/auth"
	userv1 "github.com/spotdemo4/ts-server/internal/connect/user/v1"
	"github.com/spotdemo4/ts-server/internal/connect/user/v1/userv1connect"
	"github.com/spotdemo4/ts-server/internal/models"
)

type Handler struct {
	db   *bob.DB
	auth *auth.Auth

	sessions *map[int32]*webauthn.SessionData
	mu       sync.Mutex
}

func (h *Handler) GetUser(
	ctx context.Context,
	_ *connect.Request[userv1.GetUserRequest],
) (*connect.Response[userv1.GetUserResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	return connect.NewResponse(&userv1.GetUserResponse{
		User: &userv1.User{
			Id:               user.ID,
			Username:         user.Username,
			ProfilePictureId: user.ProfilePictureID.Ptr(),
		},
	}), nil
}

func (h *Handler) UpdatePassword(
	ctx context.Context,
	req *connect.Request[userv1.UpdatePasswordRequest],
) (*connect.Response[userv1.UpdatePasswordResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Validate
	if !user.Validate(req.Msg.GetOldPassword()) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("invalid password"))
	}
	if req.Msg.GetNewPassword() != req.Msg.GetConfirmPassword() {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("passwords do not match"))
	}

	// Update password
	err := user.SetPassword(ctx, req.Msg.GetNewPassword())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&userv1.UpdatePasswordResponse{})
	return res, nil
}

const DefaultAPIKeyDuration = time.Hour * 24

func (h *Handler) GetAPIKey(
	ctx context.Context,
	req *connect.Request[userv1.GetAPIKeyRequest],
) (*connect.Response[userv1.GetAPIKeyResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Validate
	if !user.Validate(req.Msg.GetPassword()) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("invalid username or password"))
	}
	if req.Msg.GetPassword() != req.Msg.GetConfirmPassword() {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("passwords do not match"))
	}

	res := connect.NewResponse(&userv1.GetAPIKeyResponse{
		Key: user.Token(time.Now().Add(DefaultAPIKeyDuration)),
	})
	return res, nil
}

func (h *Handler) UpdateProfilePicture(
	ctx context.Context,
	req *connect.Request[userv1.UpdateProfilePictureRequest],
) (*connect.Response[userv1.UpdateProfilePictureResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Validate file
	fileType := http.DetectContentType(req.Msg.GetData())
	if fileType != "image/jpeg" && fileType != "image/png" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid file type"))
	}

	// Update profile picture
	err := user.SetProfilePicture(ctx, req.Msg.GetFileName(), req.Msg.GetData())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userv1.UpdateProfilePictureResponse{
		User: &userv1.User{
			Id:               user.ID,
			Username:         user.Username,
			ProfilePictureId: user.ProfilePictureID.Ptr(),
		},
	}), nil
}

func (h *Handler) BeginPasskeyRegistration(
	ctx context.Context,
	_ *connect.Request[userv1.BeginPasskeyRegistrationRequest],
) (*connect.Response[userv1.BeginPasskeyRegistrationResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Get options for user
	options, session, err := h.auth.Web.BeginRegistration(user)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Turn options into json
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Set session for validation later
	h.setSession(user.ID, session)

	return connect.NewResponse(&userv1.BeginPasskeyRegistrationResponse{
		OptionsJson: string(optionsJSON),
	}), nil
}

func (h *Handler) FinishPasskeyRegistration(
	ctx context.Context,
	req *connect.Request[userv1.FinishPasskeyRegistrationRequest],
) (*connect.Response[userv1.FinishPasskeyRegistrationResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Get the session data previously set
	session, err := h.getSession(user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Parse the attestation response
	parsedResponse, err := protocol.ParseCredentialCreationResponseBytes([]byte(req.Msg.GetAttestation()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Create the credential
	credential, err := h.auth.Web.CreateCredential(user, *session, parsedResponse)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Turn transports into strings to save in db
	transports := transportsToString(credential.Transport)

	// Save the credential
	_, err = models.Credentials.Insert(
		&models.CredentialSetter{
			CredID:                omit.From(string(credential.ID)),
			CredPublicKey:         omit.From(credential.PublicKey),
			SignCount:             omit.From(int32(credential.Authenticator.SignCount)),
			Transports:            omitnull.From(transports),
			UserVerified:          omitnull.From(credential.Flags.UserVerified),
			BackupEligible:        omitnull.From(credential.Flags.BackupEligible),
			BackupState:           omitnull.From(credential.Flags.BackupState),
			AttestationObject:     omitnull.From(credential.Attestation.Object),
			AttestationClientData: omitnull.From(credential.Attestation.ClientDataJSON),
			CreatedAt:             omit.From(time.Now()),
			LastUsed:              omit.From(time.Now()),
			UserID:                omit.From(user.ID),
		},
	).Exec(ctx, h.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userv1.FinishPasskeyRegistrationResponse{}), nil
}

func (h *Handler) getSession(userid int32) (*webauthn.SessionData, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	session, ok := (*h.sessions)[userid]
	if !ok {
		return nil, errors.New("session does not exist")
	}

	delete(*h.sessions, userid)
	return session, nil
}

func (h *Handler) setSession(userid int32, data *webauthn.SessionData) {
	h.mu.Lock()
	defer h.mu.Unlock()

	(*h.sessions)[userid] = data
}

func transportsToString(transports []protocol.AuthenticatorTransport) string {
	s := ""
	for _, transport := range transports {
		s += string(transport) + ", "
	}
	return s
}

func New(app *app.App, interceptors connect.Option) (string, http.Handler) {
	sd := map[int32]*webauthn.SessionData{}
	return userv1connect.NewUserServiceHandler(
		&Handler{
			db:   app.DB,
			auth: app.Auth,

			sessions: &sd,
			mu:       sync.Mutex{},
		},
		interceptors,
	)
}
