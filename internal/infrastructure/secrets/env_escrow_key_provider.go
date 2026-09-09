// Package secrets holds the adapters that stand at the secrets/key-management
// boundary. Custody of secret values belongs to the secrets service (Doppler
// today, the approved IAM-backed service once alpaca-infra names it); this
// package only turns injected material into unlocked in-memory keys and never
// exposes, logs or returns the material itself. See ADR-0007.
package secrets

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
)

// Environment variables read by NewEnvEscrowKeyProviderFromEnv. Both are
// registered as secrets in internal/config/env_registry.go.
const (
	EnvEscrowPrivateKeys          = "ESCROW_VALIDATION_PRIVATE_KEYS"
	EnvEscrowPrivateKeyPassphrase = "ESCROW_VALIDATION_PRIVATE_KEY_PASSPHRASE" //nolint:gosec // the variable name, not a credential
)

var (
	// ErrNoEscrowPrivateKeys is returned when the keyring variable is unset or empty.
	ErrNoEscrowPrivateKeys = errors.New("escrow private keyring is not configured")
	// ErrEscrowPrivateKeysUnreadable is returned when the material cannot be
	// parsed or unlocked. The message never includes any of the input.
	ErrEscrowPrivateKeysUnreadable = errors.New("escrow private keyring could not be read or unlocked")
)

// EnvEscrowKeyProvider implements interfaces.EscrowDecryptionKeyProvider from
// Doppler-injected environment variables.
//
// The keyring variable holds one or more ASCII-armored private key blocks,
// concatenated; every key is unlocked at construction with the shared
// passphrase and held in memory only. Rotation today is: add the new key to
// the variable, publish its public half to registries, and remove the old key
// once no deposit encrypted to it can still arrive. Which key decrypted each
// run is recorded on the run by the pipeline.
type EnvEscrowKeyProvider struct {
	ring openpgp.EntityList
}

var _ interfaces.EscrowDecryptionKeyProvider = (*EnvEscrowKeyProvider)(nil)

// NewEnvEscrowKeyProviderFromEnv reads the keyring from the environment.
func NewEnvEscrowKeyProviderFromEnv() (*EnvEscrowKeyProvider, error) {
	return NewEnvEscrowKeyProvider(os.Getenv(EnvEscrowPrivateKeys), os.Getenv(EnvEscrowPrivateKeyPassphrase))
}

// NewEnvEscrowKeyProvider builds a provider from armored private key material
// and an optional passphrase. Errors are fixed text: nothing from the inputs
// is ever echoed.
func NewEnvEscrowKeyProvider(armoredKeys, passphrase string) (*EnvEscrowKeyProvider, error) {
	if strings.TrimSpace(armoredKeys) == "" {
		return nil, ErrNoEscrowPrivateKeys
	}
	// ReadArmoredKeyRing stops after the first armored block, so a ring of
	// concatenated blocks (the rollover case) is split and parsed block by block.
	var ring openpgp.EntityList
	for _, block := range splitArmoredBlocks(armoredKeys) {
		el, err := openpgp.ReadArmoredKeyRing(strings.NewReader(block))
		if err != nil || len(el) == 0 {
			return nil, ErrEscrowPrivateKeysUnreadable
		}
		ring = append(ring, el...)
	}
	if len(ring) == 0 {
		return nil, ErrEscrowPrivateKeysUnreadable
	}
	var unlocked openpgp.EntityList
	for _, e := range ring {
		if e.PrivateKey == nil {
			continue // a public key in the ring is harmless but useless here
		}
		if e.PrivateKey.Encrypted || anySubkeyEncrypted(e) {
			if passphrase == "" {
				return nil, ErrEscrowPrivateKeysUnreadable
			}
			if err := e.DecryptPrivateKeys([]byte(passphrase)); err != nil {
				return nil, ErrEscrowPrivateKeysUnreadable
			}
		}
		unlocked = append(unlocked, e)
	}
	if len(unlocked) == 0 {
		return nil, ErrEscrowPrivateKeysUnreadable
	}
	// Fingerprints are public: logging them once at startup is the audit
	// hook operators need to confirm which keys a worker holds.
	fps := make([]string, len(unlocked))
	for i, e := range unlocked {
		fps[i] = rdevalidate.FingerprintHex(e)
	}
	log.Printf("escrow validation: loaded %d decryption key(s): %s", len(unlocked), strings.Join(fps, ", "))
	return &EnvEscrowKeyProvider{ring: unlocked}, nil
}

func anySubkeyEncrypted(e *openpgp.Entity) bool {
	for _, sk := range e.Subkeys {
		if sk.PrivateKey != nil && sk.PrivateKey.Encrypted {
			return true
		}
	}
	return false
}

const armorBegin = "-----BEGIN PGP "

// splitArmoredBlocks splits concatenated ASCII-armored blocks. Text outside a
// block is ignored; block boundaries are the BEGIN markers.
func splitArmoredBlocks(s string) []string {
	var blocks []string
	for {
		start := strings.Index(s, armorBegin)
		if start < 0 {
			return blocks
		}
		s = s[start:]
		next := strings.Index(s[len(armorBegin):], armorBegin)
		if next < 0 {
			blocks = append(blocks, strings.TrimSpace(s))
			return blocks
		}
		blocks = append(blocks, strings.TrimSpace(s[:next+len(armorBegin)]))
		s = s[next+len(armorBegin):]
	}
}

// DecryptionKeyring returns the unlocked keys in the order they were configured.
func (p *EnvEscrowKeyProvider) DecryptionKeyring(_ context.Context) (openpgp.EntityList, error) {
	if p == nil || len(p.ring) == 0 {
		return nil, ErrNoEscrowPrivateKeys
	}
	return p.ring, nil
}

// Fingerprints returns the primary-key fingerprints of the loaded keys.
func (p *EnvEscrowKeyProvider) Fingerprints() []string {
	out := make([]string, 0, len(p.ring))
	for _, e := range p.ring {
		out = append(out, rdevalidate.FingerprintHex(e))
	}
	return out
}

// String never reveals key material.
func (p *EnvEscrowKeyProvider) String() string {
	return fmt.Sprintf("EnvEscrowKeyProvider(keys=%d)", len(p.ring))
}
