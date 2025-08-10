package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/aarondl/opt/null"
	"github.com/aarondl/opt/omit"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob"
	"golang.org/x/crypto/bcrypt"

	"github.com/spotdemo4/ts-server/internal/models"
)

type Auth struct {
	Web    *webauthn.WebAuthn
	issuer string
	key    string

	db *bob.DB
}

// New creates a new Auth instance.
func New(db *bob.DB, issuer string, key string, web *webauthn.WebAuthn) *Auth {
	return &Auth{
		Web:    web,
		issuer: issuer,
		key:    key,

		db: db,
	}
}

type NewUserParams struct {
	Username string
	Password string
}

// NewUser creates a new user in the database.
func (a *Auth) NewUser(ctx context.Context, params NewUserParams) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = models.Users.Insert(
		&models.UserSetter{
			Username:   omit.From(params.Username),
			Password:   omit.From(string(hash)),
			WebauthnID: omit.From(uuid.New().String()),
		},
	).Exec(ctx, a.db)
	if err != nil {
		return err
	}

	return nil
}

// GetUser retrieves a user by their ID.
func (a *Auth) GetUser(ctx context.Context, userid int32) (User, error) {
	user, err := models.Users.Query(
		models.SelectWhere.Users.ID.EQ(userid),
	).One(ctx, a.db)
	if err != nil {
		return User{}, err
	}

	return User{
		User: *user,
		db:   a.db,
		auth: a,
	}, nil
}

// GetUserByName retrieves a user by their username.
func (a *Auth) GetUserByName(ctx context.Context, username string) (User, error) {
	user, err := models.Users.Query(
		models.SelectWhere.Users.Username.EQ(username),
	).One(ctx, a.db)
	if err != nil {
		return User{}, err
	}

	return User{
		User: *user,
		db:   a.db,
		auth: a,
	}, nil
}

type Claims struct {
	jwt.RegisteredClaims

	Password         string `json:"password"`
	ProfilePictureID *int32 `json:"profilePictureID"`
	WebauthnID       string `json:"webauthnID"`
}

// GetUserFromToken retrieves a user from a JWT token.
func (a *Auth) GetUserFromToken(tokenString string) (User, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(a.key), nil
	})
	if err != nil {
		return User{}, err
	}
	if !token.Valid {
		return User{}, errors.New("token not valid")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return User{}, errors.New("could not parse claims")
	}

	userid, err := strconv.ParseInt(claims.ID, 10, 32)
	if err != nil {
		return User{}, errors.New("invalid id")
	}

	return User{
		User: models.User{
			ID:               int32(userid),
			Username:         claims.Subject,
			Password:         claims.Password,
			ProfilePictureID: null.FromPtr(claims.ProfilePictureID),
			WebauthnID:       claims.WebauthnID,
		},

		db:   a.db,
		auth: a,
	}, nil
}

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type key int32

//nolint:gochecknoglobals // userKey is the key for user.User values in Contexts.
var userKey key

// NewContext returns a new Context that carries value user.User.
func (*Auth) NewContext(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// GetContext retrieves the user.User from the context, if it exists.
func (*Auth) GetContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey).(User)
	return u, ok
}
