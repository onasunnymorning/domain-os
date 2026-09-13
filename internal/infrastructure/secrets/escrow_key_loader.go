package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// escrowSecretPayload is the JSON value stored for one key version. It is the
// only format this package writes or reads; the store itself sees opaque bytes.
//
// An OpenPGP version keeps its armored private key exactly as imported, still
// protected by its passphrase, so the backend's encryption is not the only
// layer. A symmetric version holds raw key bytes (base64 in JSON).
type escrowSecretPayload struct {
	Format            string `json:"format"`
	ArmoredPrivateKey string `json:"armoredPrivateKey,omitempty"`
	Passphrase        string `json:"passphrase,omitempty"` //nolint:gosec // G117: the field holds the passphrase by design; this struct only ever travels to and from the secret store
	Key               []byte `json:"key,omitempty"`
}

// Payload format labels, not credentials.
const (
	escrowSecretFormatOpenPGP   = "openpgp-private/v1" // #nosec G101 -- a format label, not a credential
	escrowSecretFormatSymmetric = "symmetric/v1"       // #nosec G101 -- a format label, not a credential
)

// EncodeOpenPGPSecret builds the stored value for an OpenPGP private key.
func EncodeOpenPGPSecret(armoredPrivateKey, passphrase string) ([]byte, error) {
	return json.Marshal(escrowSecretPayload{Format: escrowSecretFormatOpenPGP, ArmoredPrivateKey: armoredPrivateKey, Passphrase: passphrase})
}

// EncodeSymmetricSecret builds the stored value for a symmetric key.
func EncodeSymmetricSecret(key []byte) ([]byte, error) {
	return json.Marshal(escrowSecretPayload{Format: escrowSecretFormatSymmetric, Key: key})
}

func decodeEscrowSecret(raw []byte, format string) (escrowSecretPayload, error) {
	var p escrowSecretPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.Format != format {
		return escrowSecretPayload{}, entities.ErrEscrowKeyMaterialUnreadable
	}
	return p, nil
}

// EscrowKeyLoader implements interfaces.EscrowKeyMaterialLoader over an
// EscrowSecretStore. Unlocked material is cached in memory by version id for a
// short time, so a validation that retries does not refetch and re-unlock; the
// cache is never the authority on whether a version may be used — callers
// re-check the version's state before every use.
type EscrowKeyLoader struct {
	store interfaces.EscrowSecretStore
	ttl   time.Duration
	now   func() time.Time

	mu    sync.Mutex
	cache map[uuid.UUID]cachedEscrowKey
}

type cachedEscrowKey struct {
	fingerprint string
	entity      *openpgp.Entity
	symmetric   []byte
	expires     time.Time
}

var _ interfaces.EscrowKeyMaterialLoader = (*EscrowKeyLoader)(nil)

// DefaultEscrowKeyCacheTTL bounds how long unlocked material stays in memory.
const DefaultEscrowKeyCacheTTL = 5 * time.Minute

// NewEscrowKeyLoader creates a loader. A nil store is allowed: every load then
// fails with entities.ErrEscrowKeyStoreNotConfigured, which lets a worker
// without a key store still run everything that needs no private material.
func NewEscrowKeyLoader(store interfaces.EscrowSecretStore, ttl time.Duration) *EscrowKeyLoader {
	if ttl <= 0 {
		ttl = DefaultEscrowKeyCacheTTL
	}
	return &EscrowKeyLoader{store: store, ttl: ttl, now: time.Now, cache: map[uuid.UUID]cachedEscrowKey{}}
}

func (l *EscrowKeyLoader) cached(ref entities.EscrowKeyVersionRef) (cachedEscrowKey, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.cache[ref.ID]
	if !ok || l.now().After(c.expires) || c.fingerprint != ref.Fingerprint {
		delete(l.cache, ref.ID)
		return cachedEscrowKey{}, false
	}
	return c, true
}

func (l *EscrowKeyLoader) remember(ref entities.EscrowKeyVersionRef, c cachedEscrowKey) {
	l.mu.Lock()
	defer l.mu.Unlock()
	c.fingerprint, c.expires = ref.Fingerprint, l.now().Add(l.ttl)
	l.cache[ref.ID] = c
}

// Forget drops a version from the cache, e.g. after it was revoked.
func (l *EscrowKeyLoader) Forget(id uuid.UUID) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.cache, id)
}

func (l *EscrowKeyLoader) fetch(ctx context.Context, ref entities.EscrowKeyVersionRef) ([]byte, error) {
	if l.store == nil {
		return nil, entities.ErrEscrowKeyStoreNotConfigured
	}
	if ref.SecretRef == nil || ref.SecretRef.Name == "" {
		return nil, entities.ErrEscrowKeyMaterialNotApplicable
	}
	return l.store.Get(ctx, *ref.SecretRef)
}

// OpenPGPPrivateKey fetches, unlocks and fingerprint-checks an OpenPGP version.
func (l *EscrowKeyLoader) OpenPGPPrivateKey(ctx context.Context, ref entities.EscrowKeyVersionRef) (*openpgp.Entity, error) {
	if c, ok := l.cached(ref); ok && c.entity != nil {
		return c.entity, nil
	}
	raw, err := l.fetch(ctx, ref)
	if err != nil {
		return nil, err
	}
	p, err := decodeEscrowSecret(raw, escrowSecretFormatOpenPGP)
	if err != nil {
		return nil, err
	}
	ring, err := ParseAndUnlockArmoredPrivateKeys(p.ArmoredPrivateKey, p.Passphrase)
	if err != nil || len(ring) != 1 {
		return nil, entities.ErrEscrowKeyMaterialUnreadable
	}
	if fp := rdevalidate.FingerprintHex(ring[0]); fp != ref.Fingerprint {
		return nil, fmt.Errorf("version %s: %w", ref.ID, entities.ErrEscrowKeyFingerprintMismatch)
	}
	l.remember(ref, cachedEscrowKey{entity: ring[0]})
	return ring[0], nil
}

// SymmetricKey fetches, decodes and fingerprint-checks a symmetric version.
func (l *EscrowKeyLoader) SymmetricKey(ctx context.Context, ref entities.EscrowKeyVersionRef) ([]byte, error) {
	if c, ok := l.cached(ref); ok && c.symmetric != nil {
		return c.symmetric, nil
	}
	raw, err := l.fetch(ctx, ref)
	if err != nil {
		return nil, err
	}
	p, err := decodeEscrowSecret(raw, escrowSecretFormatSymmetric)
	if err != nil {
		return nil, err
	}
	fp, err := rdesanitize.MasterKeyFingerprint(p.Key)
	if err != nil {
		return nil, entities.ErrEscrowKeyMaterialUnreadable
	}
	if fp != ref.Fingerprint {
		return nil, fmt.Errorf("version %s: %w", ref.ID, entities.ErrEscrowKeyFingerprintMismatch)
	}
	l.remember(ref, cachedEscrowKey{symmetric: p.Key})
	return p.Key, nil
}

// IsEscrowKeyStoreTransient reports whether a load failure may succeed on
// retry: the store was unreachable. Everything else is a fact about the
// material or the configuration and will not change by retrying.
func IsEscrowKeyStoreTransient(err error) bool {
	return errors.Is(err, entities.ErrEscrowKeyStoreUnavailable)
}
