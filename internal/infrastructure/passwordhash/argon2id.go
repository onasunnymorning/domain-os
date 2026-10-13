// Package passwordhash hashes EPP passwords with Argon2id (RFC 9106).
package passwordhash

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are the Argon2id cost parameters.
type Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultParams are OWASP's first recommended Argon2id configuration: 19 MiB,
// two passes, one lane. A login verifies at most a couple of hashes, and the
// memory cost is per verification, so it is kept modest: the EPP server also
// bounds how many verifications run at once.
var DefaultParams = Params{MemoryKiB: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}

// ErrMalformedHash is returned for a stored hash this package cannot read.
var ErrMalformedHash = errors.New("malformed argon2id hash")

// Argon2id hashes passwords into the PHC string format,
// $argon2id$v=19$m=<KiB>,t=<iterations>,p=<lanes>$<salt>$<hash>. Verify reads
// the parameters from the hash, so raising DefaultParams leaves old hashes
// verifiable.
type Argon2id struct {
	params Params
}

// New returns a hasher using p for new hashes.
func New(p Params) *Argon2id {
	return &Argon2id{params: p}
}

// Hash returns the encoded hash of plain with a fresh random salt.
func (h *Argon2id) Hash(plain string) (string, error) {
	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2id salt: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, h.params.Iterations, h.params.MemoryKiB, h.params.Parallelism, h.params.KeyLength)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.params.MemoryKiB, h.params.Iterations, h.params.Parallelism,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify reports whether plain matches encoded, in constant time.
func (h *Argon2id) Verify(encoded, plain string) (bool, error) {
	p, salt, key, err := decode(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(plain), salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(key))) //nolint:gosec // G115: len(key) is bounded by decode
	return subtle.ConstantTimeCompare(got, key) == 1, nil
}

// maxMemoryKiB bounds the memory a stored hash can make Verify allocate, so a
// corrupted or planted hash cannot exhaust the server.
const maxMemoryKiB = 256 * 1024

func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Iterations, &p.Parallelism); err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	if p.MemoryKiB == 0 || p.MemoryKiB > maxMemoryKiB || p.Iterations == 0 || p.Iterations > 16 || p.Parallelism == 0 {
		return Params{}, nil, nil, ErrMalformedHash
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return Params{}, nil, nil, ErrMalformedHash
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) < 16 || len(key) > 64 {
		return Params{}, nil, nil, ErrMalformedHash
	}
	return p, salt, key, nil
}
