package entities

import (
	"strings"
	"unicode"

	"golang.org/x/net/idna"
)

const (
	LABEL_MAX_CHAR = 63
	LABEL_MIN_CHAR = 1
)

// TODO: replace all reference to this method with the functions in the Label.go file

// Helper function to validate Labels. It will return a boolean indicating if the label is valid or not.
// It expects a normalized input string and will fail if the input contains invalid characters
// It does not consider uppercase and lowercase letters to be different, please lowecase all domain names before storing them
// A label is a section of a FQDN separated by a dot
// A label can contain letters, digits and hyphens
// A label can be between 1 and 63 characters long
// A label cannot start or end with a hyphen
// A NON-IDN label cannot contain two consecutive hyphens
// AN IDN label must be convertible to Unicode
func IsValidLabel(label string) bool {
	invalidChar := findInvalidLabelCharacters(label)
	// It contains invalid characters
	if invalidChar != "" {
		return false
	}
	// It is too short or too long
	if (len(label) < LABEL_MIN_CHAR) || len(label) > LABEL_MAX_CHAR {
		return false
	}
	// It starts or ends with a hyphen
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return false
	}
	// It contains two consecutive hyphens
	if !(strings.HasPrefix(label, "xn--")) && strings.Contains(label, "--") {
		return false
	}
	// It is an IDN label and is not valid
	if strings.HasPrefix(label, "xn--") {
		_, err := idna.Lookup.ToUnicode(label)
		if err != nil {
			return false
		}
	}
	return true
}

// Helper function to find any invalid characters in a label. It will return the first invalid character or an empty string if the label has no invalid characters
// A label is a section of a FQDN separated by a dot
// A label can contain letters, digits and hyphens
func findInvalidLabelCharacters(label string) string {
	for _, char := range label {
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
