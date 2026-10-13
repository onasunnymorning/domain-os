package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPCredentialService manages registrars' EPP logins, passwords, certificate
// pins and allowed networks (ADR 0014).
type EPPCredentialService interface {
	GetAccess(ctx context.Context, clid string) (*entities.EPPRegistrarAccess, error)
	CreateLogin(ctx context.Context, clid string, cmd commands.CreateEPPLoginCommand) (*entities.EPPLogin, error)
	UpdateLogin(ctx context.Context, clid, loginID string, cmd commands.UpdateEPPLoginCommand) (*entities.EPPLogin, error)
	// AddPassword returns the generated password when the command carried
	// none; it is not stored and cannot be retrieved again.
	AddPassword(ctx context.Context, clid, loginID string, cmd commands.AddEPPPasswordCommand) (*entities.EPPPassword, string, error)
	AddCertificatePin(ctx context.Context, clid string, cmd commands.AddEPPCertificatePinCommand) (*entities.EPPCertificatePin, error)
	AddAllowedNetwork(ctx context.Context, clid string, cmd commands.AddEPPAllowedNetworkCommand) (*entities.EPPAllowedNetwork, error)
	RetireCredential(ctx context.Context, clid string, ref entities.EPPCredentialRef, cmd commands.RetireEPPCredentialCommand) error
}
