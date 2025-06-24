package auth

import (
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/spotdemo4/ts-server/internal/models"
)

func NewCreds(creds models.CredentialSlice) []webauthn.Credential {
	webauthnCreds := []webauthn.Credential{}

	for _, c := range creds {
		transports := []protocol.AuthenticatorTransport{}
		if c.Transports.Valid {
			for t := range strings.SplitSeq(c.Transports.V, " ") {
				transports = append(transports, protocol.AuthenticatorTransport(t))
			}
		}

		flags := webauthn.CredentialFlags{}
		if c.UserVerified.Valid {
			flags.UserVerified = c.UserVerified.V
		}
		if c.BackupEligible.Valid {
			flags.BackupEligible = c.BackupEligible.V
		}
		if c.BackupState.Valid {
			flags.BackupState = c.BackupState.V
		}

		attestation := webauthn.CredentialAttestation{}
		if c.AttestationObject.Valid {
			attestation.Object = c.AttestationObject.V
		}
		if c.AttestationClientData.Valid {
			attestation.ClientDataJSON = c.AttestationClientData.V
		}

		webauthnCreds = append(webauthnCreds, webauthn.Credential{
			ID:        []byte(c.CredID),
			PublicKey: c.CredPublicKey,
			Authenticator: webauthn.Authenticator{
				SignCount: uint32(c.SignCount),
			},
			Transport:   transports,
			Flags:       flags,
			Attestation: attestation,
		})
	}

	return webauthnCreds
}
