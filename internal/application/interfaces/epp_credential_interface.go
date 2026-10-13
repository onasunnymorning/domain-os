package interfaces

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"

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

// EPPLoginLockout counts failed logins per key and locks a key out after too
// many. The key is (login, source address), so failures from one place cannot
// lock a registrar out everywhere.
type EPPLoginLockout interface {
	// LockedFor returns how much longer key is locked out, or zero.
	LockedFor(ctx context.Context, key string) (time.Duration, error)
	// RecordFailure counts a failure for key and returns the lockout it
	// started, or zero.
	RecordFailure(ctx context.Context, key string) (time.Duration, error)
	// Clear forgets key's failures and lockouts, after a successful login.
	Clear(ctx context.Context, key string) error
}

// EPPAuthenticator is what the EPP server asks about credentials.
type EPPAuthenticator interface {
	// Login authenticates a <login>. Every refusal wraps
	// entities.ErrEPPAuthentication; an unreachable credential store is
	// entities.ErrEPPCredentialsUnavailable; an unacceptable <newPW> is
	// entities.ErrInvalidEPPPassword.
	// A <newPW> is checked but not applied: see ReplacePassword.
	Login(ctx context.Context, cmd commands.EPPLoginCommand) (*entities.EPPAuthenticatedLogin, error)
	// ReplacePassword applies <newPW> for a login that Login authenticated
	// and the session registry admitted, and returns the new password's ID.
	ReplacePassword(ctx context.Context, login *entities.EPPAuthenticatedLogin, newPW string) (uuid.UUID, error)
	// AllowedNetworks returns every registrar's usable allowed networks.
	AllowedNetworks(ctx context.Context) ([]netip.Prefix, error)
	// CompromisedSince returns credentials retired as compromised since.
	CompromisedSince(ctx context.Context, since time.Time) ([]uuid.UUID, error)
}
