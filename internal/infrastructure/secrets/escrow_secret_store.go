package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/secrets/awssm"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EnvEscrowKeyStoreBackend selects the escrow key store (ADR-0009).
const EnvEscrowKeyStoreBackend = "ESCROW_CUSTODY_BACKEND"

// Escrow key store backends.
const (
	EscrowKeyStoreNone  = "none"
	EscrowKeyStoreAWSSM = "aws-secrets-manager"
)

// NewEscrowSecretStoreFromEnv builds the configured store. With no backend
// configured it returns entities.ErrEscrowKeyStoreNotConfigured, which callers
// treat as "this process holds no private key material", not as a failure to
// start: public keys, arrangements and unsigned deposits need no store.
func NewEscrowSecretStoreFromEnv(ctx context.Context) (interfaces.EscrowSecretStore, error) {
	switch backend := strings.TrimSpace(os.Getenv(EnvEscrowKeyStoreBackend)); backend {
	case "", EscrowKeyStoreNone:
		return nil, entities.ErrEscrowKeyStoreNotConfigured
	case EscrowKeyStoreAWSSM:
		cfg, err := awssm.ConfigFromEnv()
		if err != nil {
			return nil, fmt.Errorf("escrow key store: %w", err)
		}
		return awssm.New(ctx, cfg)
	default:
		return nil, fmt.Errorf("escrow key store: %s must be %q or %q", EnvEscrowKeyStoreBackend, EscrowKeyStoreNone, EscrowKeyStoreAWSSM)
	}
}
