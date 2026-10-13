package schema

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Base is the XSD built-in type a Simple restricts.
type Base int

// The built-in types the EPP schemas use.
const (
	String Base = iota
	NormalizedString
	Token
	AnyURI
	Language
	Boolean
	UnsignedShort
)

// Simple is an XSD simple type: a built-in base and the facets restricting
// it. Facets apply to the value after the base's whitespace processing, as
// XSD specifies. Zero MinLen and MaxLen mean no bound.
type Simple struct {
	Base           Base
	MinLen, MaxLen int
	// Pattern must match the whole value. XSD patterns are implicitly
	// anchored; write the Go equivalent with ^ and $.
	Pattern *regexp.Regexp
	Enum    []string
	// MinInclusive and MaxInclusive bound numeric bases when HasRange.
	MinInclusive, MaxInclusive int64
	HasRange                   bool
}

var (
	languagePattern = regexp.MustCompile(`^[a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*$`)
	unsignedPattern = regexp.MustCompile(`^\+?[0-9]+$`)
)

// validate checks a lexical value against the type.
func (t *Simple) validate(raw string) error {
	v := t.whitespace(raw)
	switch t.Base {
	case Language:
		if !languagePattern.MatchString(v) {
			return errors.New("not a language tag")
		}
	case Boolean:
		if v != "true" && v != "false" && v != "1" && v != "0" {
			return errors.New("not a boolean")
		}
	case UnsignedShort:
		if !unsignedPattern.MatchString(v) {
			return errors.New("not an unsigned integer")
		}
		n, err := strconv.ParseUint(strings.TrimPrefix(v, "+"), 10, 16)
		if err != nil {
			return errors.New("out of range for an unsignedShort")
		}
		if t.HasRange && (int64(n) < t.MinInclusive || int64(n) > t.MaxInclusive) {
			return fmt.Errorf("must be from %d to %d", t.MinInclusive, t.MaxInclusive)
		}
	}
	if n := utf8.RuneCountInString(v); (t.MinLen > 0 && n < t.MinLen) || (t.MaxLen > 0 && n > t.MaxLen) {
		switch {
		case t.MaxLen == 0:
			return fmt.Errorf("must be at least %d characters", t.MinLen)
		case t.MinLen == 0:
			return fmt.Errorf("must be at most %d characters", t.MaxLen)
		}
		return fmt.Errorf("must be %d to %d characters", t.MinLen, t.MaxLen)
	}
	if t.Pattern != nil && !t.Pattern.MatchString(v) {
		return errors.New("does not match the required pattern")
	}
	if len(t.Enum) > 0 && !slices.Contains(t.Enum, v) {
		return fmt.Errorf("must be one of %s", strings.Join(t.Enum, ", "))
	}
	return nil
}

// whitespace applies the base type's whitespace facet: string preserves,
// normalizedString replaces, and every other base collapses.
func (t *Simple) whitespace(v string) string {
	switch t.Base {
	case String:
		return v
	case NormalizedString:
		return strings.Map(func(r rune) rune {
			if r == '\t' || r == '\n' || r == '\r' {
				return ' '
			}
			return r
		}, v)
	default:
		return strings.Join(strings.FieldsFunc(v, func(r rune) bool {
			return r == ' ' || r == '\t' || r == '\n' || r == '\r'
		}), " ")
	}
}
