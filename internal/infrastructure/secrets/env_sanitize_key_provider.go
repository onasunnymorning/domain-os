package secrets

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
)

// EnvSanitizeTokenKey is the environment variable holding the master HMAC key
// for escrow derivative pseudonymisation, injected by the secrets manager.
const EnvSanitizeTokenKey = "ESCROW_SANITIZE_HMAC_KEY"

// Errors are fixed text: an error path that quoted the variable's contents
// would put key material into logs and workflow history.
var (
	// ErrNoSanitizeTokenKey means the variable is unset or empty.
	ErrNoSanitizeTokenKey = errors.New("no escrow sanitization HMAC key is configured")
	// ErrSanitizeTokenKeyUnreadable means the value could not be decoded, or is
	// too short to be a useful barrier to re-identification.
	ErrSanitizeTokenKeyUnreadable = errors.New("the escrow sanitization HMAC key could not be read: expected at least 32 bytes, base64- or hex-encoded")
)

// EnvSanitizeKeyProvider holds the master key in memory for the life of the
// worker. It is the interim adapter: replacing it with a secrets-service client
// changes nothing behind interfaces.SanitizationTokenKeyProvider.
type EnvSanitizeKeyProvider struct {
	key         []byte
	fingerprint string
}

var _ interfaces.SanitizationTokenKeyProvider = (*EnvSanitizeKeyProvider)(nil)

// NewEnvSanitizeKeyProviderFromEnv reads the key from the environment.
func NewEnvSanitizeKeyProviderFromEnv() (*EnvSanitizeKeyProvider, error) {
	return NewEnvSanitizeKeyProvider(os.Getenv(EnvSanitizeTokenKey))
}

// NewEnvSanitizeKeyProvider decodes and validates the master key. The encoding
// is accepted as base64 or hex so an operator can paste whatever their secrets
// manager produces without a re-encoding step that invites a transcription
// mistake.
func NewEnvSanitizeKeyProvider(encoded string) (*EnvSanitizeKeyProvider, error) {
	raw := strings.TrimSpace(encoded)
	if raw == "" {
		return nil, ErrNoSanitizeTokenKey
	}
	key, err := decodeKey(raw)
	if err != nil || len(key) < rdesanitize.MinTokenKeyBytes {
		return nil, ErrSanitizeTokenKeyUnreadable
	}
	// Deriving a throwaway tokenizer is the only way to obtain the fingerprint,
	// and it proves at startup that the key is usable rather than failing on
	// the first deposit.
	probe, err := rdesanitize.NewTokenizer(key, "startup-probe", rdesanitize.PolicyVersion)
	if err != nil {
		return nil, ErrSanitizeTokenKeyUnreadable
	}
	p := &EnvSanitizeKeyProvider{key: key, fingerprint: probe.KeyFingerprint()}
	// The fingerprint is a MAC over a fixed public string and cannot be
	// inverted, so logging it is safe — and it is the only way an operator can
	// confirm which key a worker holds without handling the key.
	log.Printf("escrow sanitization: token key loaded (fingerprint %s, policy %s)", p.fingerprint, rdesanitize.PolicyVersion)
	return p, nil
}

func decodeKey(raw string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) >= rdesanitize.MinTokenKeyBytes {
		return b, nil
	}
	if b, err := hex.DecodeString(raw); err == nil && len(b) >= rdesanitize.MinTokenKeyBytes {
		return b, nil
	}
	// A long enough literal is accepted as raw bytes: refusing it would push
	// operators towards weaker values that happen to decode.
	if len(raw) >= rdesanitize.MinTokenKeyBytes {
		return []byte(raw), nil
	}
	return nil, ErrSanitizeTokenKeyUnreadable
}

// TokenKey implements interfaces.SanitizationTokenKeyProvider.
func (p *EnvSanitizeKeyProvider) TokenKey(_ context.Context) ([]byte, error) {
	if len(p.key) == 0 {
		return nil, ErrNoSanitizeTokenKey
	}
	return p.key, nil
}

// KeyFingerprint identifies the loaded key without revealing it.
func (p *EnvSanitizeKeyProvider) KeyFingerprint() string { return p.fingerprint }

// String never reveals key material.
func (p *EnvSanitizeKeyProvider) String() string {
	return "EnvSanitizeKeyProvider(fingerprint=" + p.fingerprint + ")"
}
