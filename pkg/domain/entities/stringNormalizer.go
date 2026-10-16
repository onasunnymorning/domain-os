package entities

import (
	"strings"
)

// NormalizeString replaces newlines (\n), tabs (\t) and carriage returns
// (\r) with spaces, collapses runs of spaces into one, and trims leading and
// trailing spaces.
//
// It keeps a trailing dot: in a name ("Example Inc."), an address or an
// authorization password the dot is part of the value. Callers whose value is
// a DNS name in absolute form ("example.com.") remove it themselves, as
// NewDomainName does.
func NormalizeString(s string) string {
	s = RemoveNewlines(s)
	s = RemoveTabs(s)
	s = RemoveCarriageReturns(s)
	s = ReplaceMultipleSpaces(s)
	return strings.TrimSpace(s)
}

// NormalizeEmail normalizes an email address as NormalizeString does, in
// lower case, and drops a trailing dot: after an address it is the end of a
// sentence ("write to ops@example.com."), as in some IANA registrar records,
// never part of the address, which would not parse with it.
func NormalizeEmail(s string) string {
	return strings.ToLower(RemoveTrailingDot(NormalizeString(s)))
}

// RemoveTrailingDot removes a trailing dot from a string, such as the root of
// a DNS name in absolute form
func RemoveTrailingDot(s string) string {
	return strings.TrimSuffix(s, ".")
}

// Remove \n (newlines) from a string and replace with a space
func RemoveNewlines(s string) string {
	return strings.ReplaceAll(s, "\n", " ")
}

// Remove \t (tabs) from a string and replace with a space
func RemoveTabs(s string) string {
	return strings.ReplaceAll(s, "\t", " ")
}

// Remove \r (carriage returns) from a string and replace with a space
func RemoveCarriageReturns(s string) string {
	return strings.ReplaceAll(s, "\r", " ")
}

// Replace multiple spaces with one space
func ReplaceMultipleSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Runs StandardizeString on all elements of a slice of strings
func NormalizeStringSlice(s []string) []string {
	for i, v := range s {
		s[i] = NormalizeString(v)
	}
	return s
}
