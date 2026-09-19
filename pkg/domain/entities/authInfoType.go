package entities

import (
	"crypto/rand"
	"fmt"
	"io"
	"unicode"
)

// The RFC does not specify complexity requirements for the authInfo field
// We should ensure a strong password is used on the authInfo field
// The password must be between 8 and 22 characters long
// It must contain at least one uppercase letter, one lowercase letter, one number, and one special character
// The special characters are: !"#$%&'()*+,-./:;<=>?@[\]^_“{|}~
const (
	AUTHINFO_MIN_LENGTH = 8
	AUTHINFO_MAX_LENGTH = 22
)

var (
	// We can't use this in our code because GO does not support lookaheads
	AUTHINFO_REGEX     = fmt.Sprintf("^(?=.*[A-Z])(?=.*[a-z])(?=.*[\\W_]).{%d,%d}$", AUTHINFO_MIN_LENGTH, AUTHINFO_MAX_LENGTH)
	ErrInvalidAuthInfo = fmt.Errorf("invalid authInfo. It must be between %d and %d characters long and contain at least one uppercase letter, one lowercase letter, one number, and one special character. Or using Regex %s", AUTHINFO_MIN_LENGTH, AUTHINFO_MAX_LENGTH, AUTHINFO_REGEX)
)

// AuthInfoType is the type of the authInfo field
type AuthInfoType string

// IsValid checks if the authInfo field is valid
func (a AuthInfoType) Validate() error {
	if len(a.String()) < AUTHINFO_MIN_LENGTH || len(a.String()) > AUTHINFO_MAX_LENGTH {
		return ErrInvalidAuthInfo
	}

	hasUpper := false
	hasLower := false
	hasSpecial := false

	for _, char := range a.String() {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	// If the password has at least one uppercase letter, one lowercase letter, one number, and one special character
	if hasUpper && hasLower && hasSpecial {
		return nil
	}
	// If the password does not have at least one uppercase letter, one lowercase letter, one number, and one special character
	return ErrInvalidAuthInfo
}

// NewAuthInfoType creates a new AuthInfoType
func NewAuthInfoType(authInfo string) (AuthInfoType, error) {
	if err := AuthInfoType(authInfo).Validate(); err == nil {
		return AuthInfoType(authInfo), nil
	}
	return "", ErrInvalidAuthInfo
}

// String returns the string representation of the authInfo field
func (a AuthInfoType) String() string {
	return string(a)
}

const (
	// GENERATED_AUTHINFO_LENGTH is the length of a generated authInfo. It sits inside
	// [AUTHINFO_MIN_LENGTH, AUTHINFO_MAX_LENGTH] and, over the 76 character alphabet
	// below, carries roughly 100 bits of entropy.
	GENERATED_AUTHINFO_LENGTH = 16

	authInfoUpper  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	authInfoLower  = "abcdefghijklmnopqrstuvwxyz"
	authInfoDigits = "0123456789"
	// Punctuation or symbols that need no escaping in EPP XML or JSON: no <, >, &, quotes or backslash.
	// They are not URL-safe (#, % and ? are reserved), which is acceptable because an authInfo travels in
	// request bodies and EPP messages here, not in a URL; one that did would have to be percent-encoded, as
	// the wider alphabet of the frontend's generator already requires.
	//
	// No "." on purpose: NormalizeString, which NewContact applies to the authInfo, strips a trailing
	// dot, so a generated value ending in one would lose that character (and, when it was the only
	// special character, stop being valid). Anything added here has to survive NormalizeString unchanged
	// in any position; TestGenerateAuthInfo_SurvivesNormalization enforces that.
	authInfoSpecial = "!#$%*+-:=?@^_~"
)

// generatedAuthInfoClasses are the character classes a generated authInfo draws from. One
// character is guaranteed from each so the result passes Validate, and satisfies the
// "at least one number" rule the comment at the top of this file states but Validate
// does not check.
var generatedAuthInfoClasses = [...]string{authInfoUpper, authInfoLower, authInfoDigits, authInfoSpecial}

// GenerateAuthInfo returns a new, random authInfo that satisfies Validate. Use it wherever we
// must mint an authInfo the registrant never chose, such as importing an escrow deposit, which
// (RFC 9022) does not carry one. It never returns a fixed or derived value: an authInfo shared
// between objects, or one that can be recomputed, is a transfer credential for all of them.
//
// It reads from the operating system CSPRNG and returns an error if that fails, in which case
// the caller must fail rather than fall back to a weaker value.
func GenerateAuthInfo() (AuthInfoType, error) {
	return generateAuthInfo(rand.Reader)
}

// generateAuthInfo is GenerateAuthInfo with an injectable entropy source.
func generateAuthInfo(r io.Reader) (AuthInfoType, error) {
	var pool string
	for _, class := range generatedAuthInfoClasses {
		pool += class
	}

	out := make([]byte, 0, GENERATED_AUTHINFO_LENGTH)
	src := &byteSource{r: r}

	// One character from every class, then the remainder from the whole pool.
	for _, class := range generatedAuthInfoClasses {
		c, err := src.pick(class)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	for len(out) < GENERATED_AUTHINFO_LENGTH {
		c, err := src.pick(pool)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}

	// The guaranteed characters are at the front; shuffle so the position of a class carries no information.
	for i := len(out) - 1; i > 0; i-- {
		j, err := src.index(i + 1)
		if err != nil {
			return "", err
		}
		out[i], out[j] = out[j], out[i]
	}

	// Cannot fail unless the classes above are edited to break the rules; keep the guard so that shows up here.
	return NewAuthInfoType(string(out))
}

// byteSource hands out unbiased indexes from a reader. Random bytes are read in blocks, so
// generating an authInfo costs one read from the entropy source rather than one per character,
// which matters when an import mints one per object across millions of objects.
type byteSource struct {
	r   io.Reader
	buf [64]byte
	pos int
	end int
}

// next returns one random byte.
func (s *byteSource) next() (byte, error) {
	if s.pos == s.end {
		n, err := io.ReadFull(s.r, s.buf[:])
		if err != nil {
			return 0, fmt.Errorf("reading entropy for authInfo: %w", err)
		}
		s.pos, s.end = 0, n
	}
	b := s.buf[s.pos]
	s.pos++
	return b, nil
}

// index returns a uniformly distributed integer in [0, n) for 0 < n <= 256, by rejection
// sampling so that no index is favoured when n does not divide 256.
func (s *byteSource) index(n int) (int, error) {
	limit := 256 - 256%n
	for {
		b, err := s.next()
		if err != nil {
			return 0, err
		}
		if int(b) < limit {
			return int(b) % n, nil
		}
	}
}

// pick returns a uniformly chosen character from set.
func (s *byteSource) pick(set string) (byte, error) {
	i, err := s.index(len(set))
	if err != nil {
		return 0, err
	}
	return set[i], nil
}
