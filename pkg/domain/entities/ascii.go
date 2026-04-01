package entities

import (
	"unicode"
)

// IsASCII Determines weither all characters in a string are ASCII
func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// RemoveNonASCII removes all non-ASCII characters from a string
func RemoveNonASCII(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] <= unicode.MaxASCII {
			b = append(b, s[i])
		}
	}
	return string(b)
}

// RemoveNonAlphaNumeric removes all non-alphanumeric characters from a string except for dashes '-'
func RemoveNonAlphaNumeric(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if unicode.IsLetter(rune(s[i])) || unicode.IsNumber(rune(s[i])) || s[i] == '-' {
			b = append(b, s[i])
		}
	}
	return string(b)
}
