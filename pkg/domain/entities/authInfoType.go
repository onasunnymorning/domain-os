package entities

import (
	"fmt"
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
