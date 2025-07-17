package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/spotdemo4/ts-server/internal/models"
	"github.com/spotdemo4/ts-server/internal/putil"
	"github.com/stephenafamo/bob"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	issuer string
	key    string

	db *bob.DB
}

func New(db *bob.DB, issuer string, key string) *Auth {
	return &Auth{
		issuer: issuer,
		key:    key,

		db: db,
	}
}

type NewUserParams struct {
	Username string
	Password string
}

// Insert new user
func (a *Auth) NewUser(ctx context.Context, params NewUserParams) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = models.Users.Insert(
		&models.UserSetter{
			Username:   &params.Username,
			Password:   putil.ToPointer(string(hash)),
			WebauthnID: putil.ToPointer(uuid.New().String()),
		},
	).Exec(ctx, a.db)
	if err != nil {
		return err
	}

	return nil
}

// userid -> db -> user
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

// username -> db -> user
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
	Password         string `json:"password"`
	ProfilePictureID *int32 `json:"profilePictureID"`
	WebauthnID       string `json:"webauthnID"`
	jwt.RegisteredClaims
}

// token -> user
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

	// Convert sql profile pictrue ID to int32
	var ppid int32
	if claims.ProfilePictureID != nil {
		ppid = *claims.ProfilePictureID
	}

	return User{
		User: models.User{
			ID:       int32(userid),
			Username: claims.Subject,
			Password: claims.Password,
			ProfilePictureID: sql.Null[int32]{
				V:     ppid,
				Valid: true,
			},
			WebauthnID: claims.WebauthnID,
		},

		db:   a.db,
		auth: a,
	}, nil
}

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type key int32

// userKey is the key for user.User values in Contexts. It is
// unexported; clients use user.NewContext and user.FromContext
// instead of using this key directly.
var userKey key

// user -> context
func (*Auth) NewContext(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// context -> user
func (*Auth) GetContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey).(User)
	return u, ok
}
