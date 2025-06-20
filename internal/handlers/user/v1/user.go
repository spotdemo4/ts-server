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
	"github.com/spotdemo4/ts-server/internal/auth"
	userv1 "github.com/spotdemo4/ts-server/internal/connect/user/v1"
	"github.com/spotdemo4/ts-server/internal/connect/user/v1/userv1connect"
	"github.com/spotdemo4/ts-server/internal/interceptors"
	"github.com/spotdemo4/ts-server/internal/putil"
	"github.com/spotdemo4/ts-server/internal/sqlc"
	"golang.org/x/crypto/bcrypt"
)

func userToConnect(item sqlc.User) *userv1.User {
	return &userv1.User{
		Id:               item.ID,
		Username:         item.Username,
		ProfilePictureId: item.ProfilePictureID,
	}
}

type Handler struct {
	db       *sqlc.Queries
	webAuthn *webauthn.WebAuthn
	key      []byte
	name     string

	sessions *map[int64]*webauthn.SessionData
	mu       sync.Mutex
}

func (h *Handler) GetUser(ctx context.Context, _ *connect.Request[userv1.GetUserRequest]) (*connect.Response[userv1.GetUserResponse], error) {
	userid, ok := interceptors.GetUserContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	// Get user
	user, err := h.db.GetUser(ctx, userid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&userv1.GetUserResponse{
		User: userToConnect(user),
	})
	return res, nil
}

func (h *Handler) UpdatePassword(ctx context.Context, req *connect.Request[userv1.UpdatePasswordRequest]) (*connect.Response[userv1.UpdatePasswordResponse], error) {
	userid, ok := interceptors.GetUserContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	// Get user
	user, err := h.db.GetUser(ctx, userid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Validate
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Msg.OldPassword)); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("invalid password"))
	}
	if req.Msg.NewPassword != req.Msg.ConfirmPassword {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("passwords do not match"))
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Msg.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update password
	err = h.db.UpdateUser(ctx, sqlc.UpdateUserParams{
		Password: putil.ToPointer(string(hash)),
		ID:       userid,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&userv1.UpdatePasswordResponse{})
	return res, nil
}

func (h *Handler) GetAPIKey(ctx context.Context, req *connect.Request[userv1.GetAPIKeyRequest]) (*connect.Response[userv1.GetAPIKeyResponse], error) {
	userid, ok := interceptors.GetUserContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	// Get user
	user, err := h.db.GetUser(ctx, userid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Validate
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Msg.Password)); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("invalid username or password"))
	}
	if req.Msg.Password != req.Msg.ConfirmPassword {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("passwords do not match"))
	}

	// Generate JWT
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:  h.name,
		Subject: strconv.FormatInt(user.ID, 10),
		IssuedAt: &jwt.NumericDate{
			Time: time.Now(),
		},
	})
	ss, err := t.SignedString(h.key)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&userv1.GetAPIKeyResponse{
		Key: ss,
	})
	return res, nil
}

func (h *Handler) UpdateProfilePicture(ctx context.Context, req *connect.Request[userv1.UpdateProfilePictureRequest]) (*connect.Response[userv1.UpdateProfilePictureResponse], error) {
	userid, ok := interceptors.GetUserContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	// Validate file
	fileType := http.DetectContentType(req.Msg.Data)
	if fileType != "image/jpeg" && fileType != "image/png" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid file type"))
	}

	// Save bytes into file
	fileID, err := h.db.InsertFile(ctx, sqlc.InsertFileParams{
		Name:   req.Msg.FileName,
		Data:   req.Msg.Data,
		UserID: userid,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get user
	user, err := h.db.GetUser(ctx, userid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update user profile picture
	err = h.db.UpdateUser(ctx, sqlc.UpdateUserParams{
		// set
		ProfilePictureID: &fileID,

		// where
		ID: userid,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Delete old profile picture if exists
	if user.ProfilePictureID != nil {
		err = h.db.DeleteFile(ctx, sqlc.DeleteFileParams{
			ID:     *user.ProfilePictureID,
			UserID: userid,
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	res := connect.NewResponse(&userv1.UpdateProfilePictureResponse{
		User: userToConnect(user),
	})
	return res, nil
}

func (h *Handler) BeginPasskeyRegistration(ctx context.Context, _ *connect.Request[userv1.BeginPasskeyRegistrationRequest]) (*connect.Response[userv1.BeginPasskeyRegistrationResponse], error) {
	userid, ok := interceptors.GetUserContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	// Get user
	pUser, err := h.getPasskeyUser(ctx, userid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Get options for user
	options, session, err := h.webAuthn.BeginRegistration(pUser)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Turn options into json
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Set session for validation later
	h.setSession(userid, session)

	return connect.NewResponse(&userv1.BeginPasskeyRegistrationResponse{
		OptionsJson: string(optionsJSON),
	}), nil
}

func (h *Handler) FinishPasskeyRegistration(ctx context.Context, req *connect.Request[userv1.FinishPasskeyRegistrationRequest]) (*connect.Response[userv1.FinishPasskeyRegistrationResponse], error) {
	userid, ok := interceptors.GetUserContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	// Get user
	pUser, err := h.getPasskeyUser(ctx, userid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Get the session data previously set
	session, err := h.getSession(userid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Parse the attestation response
	parsedResponse, err := protocol.ParseCredentialCreationResponseBytes([]byte(req.Msg.Attestation))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Create the credential
	credential, err := h.webAuthn.CreateCredential(pUser, *session, parsedResponse)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	transports := transportsToString(credential.Transport)

	// Save the credential
	err = h.db.InsertCredential(ctx, sqlc.InsertCredentialParams{
		CredID:                string(credential.ID),
		CredPublicKey:         credential.PublicKey,
		SignCount:             int64(credential.Authenticator.SignCount),
		Transports:            &transports,
		UserVerified:          &credential.Flags.UserVerified,
		BackupEligible:        &credential.Flags.BackupEligible,
		BackupState:           &credential.Flags.BackupState,
		AttestationObject:     credential.Attestation.Object,
		AttestationClientData: credential.Attestation.ClientDataJSON,
		CreatedAt:             time.Now(),
		LastUsed:              time.Now(),
		UserID:                userid,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userv1.FinishPasskeyRegistrationResponse{}), nil
}

func (h *Handler) getPasskeyUser(ctx context.Context, userid int64) (*auth.User, error) {
	user, err := h.db.GetUser(ctx, userid)
	if err != nil {
		return nil, err
	}

	creds, err := h.db.GetCredentials(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	webCreds := auth.NewCreds(creds)
	webUser := auth.NewUser(user.WebauthnID, user.Username, webCreds)

	return &webUser, nil
}

func (h *Handler) getSession(userid int64) (*webauthn.SessionData, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	session, ok := (*h.sessions)[userid]
	if !ok {
		return nil, errors.New("session does not exist")
	}

	delete(*h.sessions, userid)
	return session, nil
}

func (h *Handler) setSession(userid int64, data *webauthn.SessionData) {
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

func NewHandler(vi *validate.Interceptor, db *sqlc.Queries, webauth *webauthn.WebAuthn, name string, key string) (string, http.Handler) {
	interceptors := connect.WithInterceptors(vi, interceptors.NewAuthInterceptor(key))

	sd := map[int64]*webauthn.SessionData{}
	return userv1connect.NewUserServiceHandler(
		&Handler{
			db:       db,
			webAuthn: webauth,
			key:      []byte(key),
			name:     name,

			sessions: &sd,
			mu:       sync.Mutex{},
		},
		interceptors,
	)
}
