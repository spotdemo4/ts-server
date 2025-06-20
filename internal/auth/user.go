package auth

import "github.com/go-webauthn/webauthn/webauthn"

type User struct {
	id          string
	username    string
	credentials []webauthn.Credential
}

func NewUser(id string, username string, credentials []webauthn.Credential) User {
	return User{
		id:          id,
		username:    username,
		credentials: credentials,
	}
}

func (u User) WebAuthnID() []byte {
	return []byte(u.id)
}

func (u User) WebAuthnName() string {
	return u.username
}

func (u User) WebAuthnDisplayName() string {
	return u.username
}

func (u User) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}
