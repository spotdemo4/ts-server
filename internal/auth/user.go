package auth

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stephenafamo/bob"
	"golang.org/x/crypto/bcrypt"

	"github.com/spotdemo4/ts-server/internal/bob/models"
)

const CookieMaxAge = 86400 // 1 day

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

// Validate checks if the provided password matches the user's password.
func (u User) Validate(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// Token generates a JWT token for the user.
func (u User) Token(expiration time.Time) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Password:         u.Password,
		ProfilePictureID: u.ProfilePictureID.Ptr(),
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

// Cookie returns a cookie with the user's token.
func (u User) Cookie(expiration time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     "token",
		Value:    u.Token(time.Now().Add(expiration)),
		Path:     "/",
		MaxAge:   CookieMaxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}

// SetProfilePicture sets a users profile picture.
func (u User) SetProfilePicture(ctx context.Context, name string, data []byte) error {
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
				Name:   omit.From(name),
				Data:   omit.From(data),
				UserID: omit.From(u.ID),
			},
		).One(ctx, u.db)
		if err != nil {
			return err
		}

		// Update user with profile picture ID
		err = u.Update(ctx, u.db, &models.UserSetter{
			ProfilePictureID: omitnull.From(file.ID),
		})
		if err != nil {
			return err
		}
	} else {
		// Update
		err = file.Update(ctx, u.db, &models.FileSetter{
			Name: omit.From(name),
			Data: omit.From(data),
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// SetPassword updates a users password.
func (u User) SetPassword(ctx context.Context, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Update user
	err = u.Update(ctx, u.db, &models.UserSetter{
		Password: omit.From(string(hash)),
	})
	if err != nil {
		return err
	}

	u.Password = string(hash)

	return nil
}
