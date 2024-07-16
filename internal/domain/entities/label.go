package entities

import (
	"strings"
	"unicode"

	"errors"
	"golang.org/x/net/idna"
)

type Label string

const (
	LABEL_MAX_LEN = 63
	LABEL_MIN_LEN = 1
)

var (
	ErrInvalidLabelLength            = errors.New("invalid label length: each label must be between 1 and 63 characters long")
	ErrInvalidLabelDash              = errors.New("invalid label: each label cannot start or end with a hyphen")
	ErrInvalidLabelDoubleDash        = errors.New("invalid label: each non-IDN label cannot contain two consecutive hyphens")
	ErrInvalidLabelIDN               = errors.New("invalid label: each IDN label must be convertible to Unicode")
	ErrLabelContainsInvalidCharacter = errors.New("invalid label: invalid character")
)

// Validate checks if the value is valid
// Validate checks if the label is valid according to the defined rules.
// It returns an error if the label is too short or too long, starts or ends with a hyphen,
// contains two consecutive hyphens (unless it is an IDN label), is an invalid IDN label,
// or contains invalid characters.
func (t Label) Validate() error {
	// It is too short or too long
	if len(t) > LABEL_MAX_LEN || len(t) < LABEL_MIN_LEN {
		return ErrInvalidLabelLength
	}
	// It starts or ends with a hyphen
	if strings.HasPrefix(t.String(), "-") || strings.HasSuffix(t.String(), "-") {
		return ErrInvalidLabelDash
	}
	// It contains two consecutive hyphens in position 3 and 4 and is not an IDN label
	if len(t) > 3 && !(strings.HasPrefix(t.String(), "xn--")) && t[2:4] == "--" {
		return ErrInvalidLabelDoubleDash
	}
	// It is an IDN label and is not valid
	if strings.HasPrefix(t.String(), "xn--") {
		_, err := idna.Lookup.ToUnicode(t.String())
		if err != nil {
			return ErrInvalidLabelIDN
		}
	}
	// It contains invalid characters
	invalidChar := t.findInvalidLabelCharacters()
	if invalidChar != "" {
		return ErrLabelContainsInvalidCharacter
	}
	return nil
}

// String returns the label as a string
func (t Label) String() string {
	return string(t)
}

// ToUnicode converts the label to Unicode
func (t Label) ToUnicode() (string, error) {
	return idna.Lookup.ToUnicode(t.String())
}

// Helper function to find any invalid characters in a label. It will return the first invalid character or an empty string if the label has no invalid characters
// A label is a section of a FQDN separated by a dot
// A label can contain letters, digits and hyphens
func (l *Label) findInvalidLabelCharacters() string {
	for _, char := range l.String() {
		// If it's not ASCII, it's invalid
		if !IsASCII(string(char)) {
			return string(char)
		}
		// If it's not a letter, digit or hyphen, it's invalid
		if !(unicode.IsLetter(char)) {
			if !(unicode.IsDigit(char)) {
				if !(string(char) == "-") {
					return string(char)
				}
			}
		}
	}
	return ""
}
