package entities

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"
)

// A domain's authorization information (its transfer code) is kept as
// RFC 9154 requires: a salted SHA-256 hash, never the code itself.
//
//   - A code is set by the sponsoring registrar only when the registrant wants
//     to transfer the domain, and the registry clears it once a transfer
//     completes. Unset is the normal state: an empty hash.
//   - The code itself is never stored, logged or returned. <domain:info> can
//     say only whether one is set, and only to the sponsor.
//   - A code presented to the registry (a transfer request, an <info> by
//     another registrar) is hashed with the stored salt and compared in
//     constant time. Nothing matches an unset code, and an empty code
//     matches nothing (RFC 9154 §4.4).
//
// SHA-256 rather than a slow password hash: a code the registry accepts
// carries at least 128 bits of entropy (ValidateSecureAuthInfo), which no
// offline search can exhaust. Codes set before this registry adopted
// RFC 9154 were shorter; they are hashed as they are, and replaced by the
// next code their registrar sets.

const (
	// authInfoHashScheme prefixes every stored hash, naming its algorithm.
	authInfoHashScheme = "$sha256$"
	// authInfoSaltBytes is the per-code salt: 128 bits (RFC 9154 §4.3).
	authInfoSaltBytes = 16
	// SecureAuthInfoBits is the least entropy a code must carry
	// (RFC 9154 §4.1).
	SecureAuthInfoBits = 128
	// SecureAuthInfoMaxLength bounds a code, so a frame cannot make the
	// registry hash megabytes.
	SecureAuthInfoMaxLength = 128
	// secureAuthInfoMinDistinct is the least number of different characters
	// a code must use: a long code of few characters is not random, whatever
	// its character classes.
	secureAuthInfoMinDistinct = 8
)

var (
	// ErrAuthInfoNotSecure is a code with less than SecureAuthInfoBits of
	// estimated entropy, or otherwise not random (RFC 9154 §5.2: EPP 2202).
	ErrAuthInfoNotSecure = fmt.Errorf("the authorization information is not secure: give a random code of at least %d bits (for example 20 printable ASCII characters, or 25 lower-case letters and digits)", SecureAuthInfoBits)
	// ErrInvalidAuthInfoHash is a stored value that is not a hash this
	// registry wrote.
	ErrInvalidAuthInfoHash = errors.New("invalid authorization information hash")
)

// AuthInfoHash is a domain's authorization information as the registry
// keeps it: a salted SHA-256 hash, or empty when no code is set.
type AuthInfoHash string

// HashAuthInfo hashes a code with a new random salt. An empty code is no
// code: the result is empty, unset.
func HashAuthInfo(code string) (AuthInfoHash, error) {
	if code == "" {
		return "", nil
	}
	salt := make([]byte, authInfoSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt the authorization information: %w", err)
	}
	return AuthInfoHash(authInfoHashScheme + base64.RawStdEncoding.EncodeToString(salt) + "$" +
		base64.RawStdEncoding.EncodeToString(digest(salt, code))), nil
}

func digest(salt []byte, code string) []byte {
	sum := sha256.Sum256(append(append([]byte{}, salt...), code...))
	return sum[:]
}

// IsSet reports whether a code is set.
func (h AuthInfoHash) IsSet() bool { return h != "" }

// Matches reports whether code is the one set. Nothing matches an unset
// code, and an empty code matches nothing (RFC 9154 §4.4).
func (h AuthInfoHash) Matches(code string) bool {
	if !h.IsSet() || code == "" {
		return false
	}
	salt, sum, err := h.parts()
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(digest(salt, code), sum) == 1
}

// Validate checks that the value is empty or a hash this registry wrote.
func (h AuthInfoHash) Validate() error {
	if !h.IsSet() {
		return nil
	}
	_, _, err := h.parts()
	return err
}

func (h AuthInfoHash) parts() (salt, sum []byte, err error) {
	rest, ok := strings.CutPrefix(string(h), authInfoHashScheme)
	if !ok {
		return nil, nil, ErrInvalidAuthInfoHash
	}
	s, d, ok := strings.Cut(rest, "$")
	if !ok {
		return nil, nil, ErrInvalidAuthInfoHash
	}
	if salt, err = base64.RawStdEncoding.DecodeString(s); err != nil || len(salt) != authInfoSaltBytes {
		return nil, nil, ErrInvalidAuthInfoHash
	}
	if sum, err = base64.RawStdEncoding.DecodeString(d); err != nil || len(sum) != sha256.Size {
		return nil, nil, ErrInvalidAuthInfoHash
	}
	return salt, sum, nil
}

// IsAuthInfoHash reports whether s is in the form HashAuthInfo writes, so
// a migration can tell a hash from a code stored before RFC 9154.
func IsAuthInfoHash(s string) bool {
	return AuthInfoHash(s).IsSet() && AuthInfoHash(s).Validate() == nil
}

// ValidateSecureAuthInfo checks that a code a registrar sets carries at least
// SecureAuthInfoBits of entropy, estimated as RFC 9154 §4.1 sizes codes:
// length times log2 of the alphabet, the alphabet being the character
// classes the code uses (26 lower-case letters, 26 upper-case, 10 digits, 33
// other printable ASCII characters). It also refuses a code of fewer than 8
// different characters or of non-ASCII characters. The check is a heuristic
// (RFC 9154 §5.2): a registrar refused should generate another random code.
func ValidateSecureAuthInfo(code string) error {
	if len(code) > SecureAuthInfoMaxLength {
		return fmt.Errorf("%w: at most %d characters", ErrAuthInfoNotSecure, SecureAuthInfoMaxLength)
	}
	var lower, upper, digit, other bool
	distinct := map[rune]bool{}
	for _, r := range code {
		switch {
		case r < 0x20 || r > 0x7e:
			return fmt.Errorf("%w: printable ASCII only", ErrAuthInfoNotSecure)
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		default:
			other = true
		}
		distinct[r] = true
	}
	alphabet := 0
	for _, c := range []struct {
		used bool
		size int
	}{{lower, 26}, {upper, 26}, {digit, 10}, {other, 33}} {
		if c.used {
			alphabet += c.size
		}
	}
	if alphabet == 0 || float64(len(code))*math.Log2(float64(alphabet)) < SecureAuthInfoBits || len(distinct) < secureAuthInfoMinDistinct {
		return ErrAuthInfoNotSecure
	}
	return nil
}

// ValidateAdminAuthInfo checks a code given through the admin API, which
// accepts what registrars must send (ValidateSecureAuthInfo) and the
// registry's earlier rule (AuthInfoType.Validate), so existing tooling and
// imports keep working. An empty code is no code, and valid.
func ValidateAdminAuthInfo(code string) error {
	if code == "" || ValidateSecureAuthInfo(code) == nil {
		return nil
	}
	return AuthInfoType(code).Validate()
}
