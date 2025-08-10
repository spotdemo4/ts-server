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
		if c.Transports.IsValue() {
			for t := range strings.SplitSeq(c.Transports.GetOrZero(), " ") {
				transports = append(transports, protocol.AuthenticatorTransport(t))
			}
		}

		flags := webauthn.CredentialFlags{}
		if c.UserVerified.IsValue() {
			flags.UserVerified = c.UserVerified.GetOrZero()
		}
		if c.BackupEligible.IsValue() {
			flags.BackupEligible = c.BackupEligible.GetOrZero()
		}
		if c.BackupState.IsValue() {
			flags.BackupState = c.BackupState.GetOrZero()
		}

		attestation := webauthn.CredentialAttestation{}
		if c.AttestationObject.IsValue() {
			attestation.Object = c.AttestationObject.GetOrZero()
		}
		if c.AttestationClientData.IsValue() {
			attestation.ClientDataJSON = c.AttestationClientData.GetOrZero()
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
