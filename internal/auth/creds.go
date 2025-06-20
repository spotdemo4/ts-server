package auth

import (
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/spotdemo4/ts-server/internal/sqlc"
)

func NewCreds(creds []sqlc.Credential) []webauthn.Credential {
	webauthnCreds := []webauthn.Credential{}

	for _, c := range creds {
		transports := []protocol.AuthenticatorTransport{}
		if c.Transports != nil {
			for t := range strings.SplitSeq(*c.Transports, " ") {
				transports = append(transports, protocol.AuthenticatorTransport(t))
			}
		}

		flags := webauthn.CredentialFlags{}
		if c.UserVerified != nil {
			flags.UserVerified = *c.UserVerified
		}
		if c.BackupEligible != nil {
			flags.BackupEligible = *c.BackupEligible
		}
		if c.BackupState != nil {
			flags.BackupState = *c.BackupState
		}

		webauthnCreds = append(webauthnCreds, webauthn.Credential{
			ID:        []byte(c.CredID),
			PublicKey: c.CredPublicKey,
			Authenticator: webauthn.Authenticator{
				SignCount: uint32(c.SignCount),
			},
			Transport: transports,
			Flags:     flags,
			Attestation: webauthn.CredentialAttestation{
				Object:         c.AttestationObject,
				ClientDataJSON: c.AttestationClientData,
			},
		})
	}

	return webauthnCreds
}
