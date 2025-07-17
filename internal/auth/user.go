package auth

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spotdemo4/ts-server/internal/models"
	"github.com/spotdemo4/ts-server/internal/putil"
	"github.com/stephenafamo/bob"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	models.User

	db   *bob.DB
	auth *Auth
}

func (u User) WebAuthnID() []byte {
	return []byte(u.WebauthnID)
}

func (u User) WebAuthnName() string {
	return u.Username
}

func (u User) WebAuthnDisplayName() string {
	return u.Username
}

func (u User) WebAuthnCredentials() []webauthn.Credential {
	creds, _ := models.Credentials.Query(
		models.SelectWhere.Credentials.UserID.EQ(u.ID),
	).All(context.Background(), u.db)

	webcreds := NewCreds(creds)

	return webcreds
}

func (u User) Validate(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	if err != nil {
		return false
	}

	return true
}

func (u User) ProfilePictureInt() *int32 {
	var ppid *int32
	if u.ProfilePictureID.Valid {
		ppid = &u.ProfilePictureID.V
	}

	return ppid
}

func (u User) Token(expiration time.Time) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Password:         u.Password,
		ProfilePictureID: u.ProfilePictureInt(),
		WebauthnID:       u.WebauthnID,

		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  u.auth.issuer,
			ID:      strconv.Itoa(int(u.ID)),
			Subject: u.Username,
			IssuedAt: &jwt.NumericDate{
				Time: time.Now(),
			},
			NotBefore: &jwt.NumericDate{
				Time: time.Now(),
			},
			ExpiresAt: &jwt.NumericDate{
				Time: expiration,
			},
		},
	})

	tokenString, _ := token.SignedString([]byte(u.auth.key))

	return tokenString
}

func (u User) Cookie(expiration time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     "token",
		Value:    u.Token(time.Now().Add(expiration)),
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}

// Insert/Update Profile Picture
func (u *User) SetProfilePicture(ctx context.Context, name string, data []byte) error {
	// Get file
	file, err := models.Files.Query(
		models.SelectWhere.Files.UserID.EQ(u.ID),
	).One(ctx, u.db)
	if err != nil {
		return err
	}

	if file == nil {
		// Insert
		file, err = models.Files.Insert(
			&models.FileSetter{
				Name:   &name,
				Data:   &data,
				UserID: &u.ID,
			},
		).One(ctx, u.db)
		if err != nil {
			return err
		}

		u.ProfilePictureID = sql.Null[int32]{
			V:     file.ID,
			Valid: true,
		}
	} else {
		// Update
		err := file.Update(ctx, u.db, &models.FileSetter{
			Name: &name,
			Data: &data,
		})
		if err != nil {
			return nil
		}
	}

	return nil
}

// Update user password
func (u *User) SetPassword(ctx context.Context, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Update user
	err = u.Update(ctx, u.db, &models.UserSetter{
		Password: putil.ToPointer(string(hash)),
	})
	if err != nil {
		return err
	}

	u.Password = string(hash)

	return nil
}
