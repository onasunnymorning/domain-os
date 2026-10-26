package schema

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
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
	UnsignedByte
	Int
	// HexBinary and Base64Binary count length facets in octets.
	HexBinary
	Base64Binary
	// Date is xs:date: a Gregorian calendar date, with an optional time
	// zone.
	Date
	// DateTime is xs:dateTime: a date and a time of day, with an optional
	// time zone.
	DateTime
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
	// Unsigned types inherit xs:nonNegativeInteger's lexical space: an
	// optional "+", or a "-" only on a zero ("-0"). Older libxml2 (2.9)
	// refused any sign; 2.15 follows XSD, and so do the models (ADR 0016).
	unsignedPattern = regexp.MustCompile(`^(\+?[0-9]+|-0+)$`)
	schemePattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)
	intPattern      = regexp.MustCompile(`^[+-]?[0-9]+$`)
	hexPattern      = regexp.MustCompile(`^[0-9A-Fa-f]*$`)
	datePattern     = regexp.MustCompile(`^(-?)([0-9]{4,})-([0-9]{2})-([0-9]{2})(Z|[+-][0-9]{2}:[0-9]{2})?$`)
	// dateTimePattern: the date, "T", the time, an optional time zone and,
	// after a time zone only, trailing whitespace (libxml2's reading).
	dateTimePattern = regexp.MustCompile(`^(-?[0-9]{4,}-[0-9]{2}-[0-9]{2})T([0-9]{2}):([0-9]{2}):([0-9]{2})(\.[0-9]+)?(?:(Z|[+-][0-9]{2}:[0-9]{2})[ \t\r\n]*)?$`)
)

// inRange applies MinInclusive and MaxInclusive, when set.
func (t *Simple) inRange(n int64) error {
	if t.HasRange && (n < t.MinInclusive || n > t.MaxInclusive) {
		return fmt.Errorf("must be from %d to %d", t.MinInclusive, t.MaxInclusive)
	}
	return nil
}

// length is the value's length as XSD's length facets count it: octets
// for binary types, characters for everything else.
func (t *Simple) length(v string) int {
	switch t.Base {
	case HexBinary:
		return len(v) / 2
	case Base64Binary:
		n, _ := base64Octets(v)
		return n
	default:
		return utf8.RuneCountInString(v)
	}
}

// base64Octets is the decoded length of an xs:base64Binary value, by
// libxml2's rule rather than XSD's stricter lexical grammar (ADR 0016): as
// RFC 2045 asks of decoders, every character outside the base64 alphabet is
// ignored; after the first "=" only "=" may follow; and the padding must
// fit the count of data characters, with the unused bits of the last one
// zero.
func base64Octets(v string) (int, error) {
	var data, pad int
	last := -1 // the value of the last data character
	for i := 0; i < len(v); i++ {
		d := base64Value(v[i])
		switch {
		case d < 0:
			continue // ignored
		case d == 64:
			pad++
		case pad > 0:
			return 0, errors.New("data after padding")
		default:
			data++
			last = d
		}
	}
	total := 3 * (data / 4)
	switch pad {
	case 0:
		if data%4 != 0 {
			return 0, errors.New("incomplete quantum")
		}
	case 1:
		if data%4 != 3 || last&^0x3c != 0 {
			return 0, errors.New("bad padding")
		}
		total += 2
	case 2:
		if data%4 != 2 || last&^0x30 != 0 {
			return 0, errors.New("bad padding")
		}
		total++
	default:
		return 0, errors.New("bad padding")
	}
	return total, nil
}

// base64Value is a base64 character's value, 64 for "=", or -1 for any
// other character.
func base64Value(c byte) int {
	switch {
	case c >= 'A' && c <= 'Z':
		return int(c - 'A')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 26
	case c >= '0' && c <= '9':
		return int(c-'0') + 52
	case c == '+':
		return 62
	case c == '/':
		return 63
	case c == '=':
		return 64
	}
	return -1
}

// validate checks a lexical value against the type.
func (t *Simple) validate(raw string) error {
	v := t.whitespace(raw)
	switch t.Base {
	case AnyURI:
		if err := checkURIReference(v); err != nil {
			return err
		}
	case Language:
		if !languagePattern.MatchString(v) {
			return errors.New("not a language tag")
		}
	case Boolean:
		if v != "true" && v != "false" && v != "1" && v != "0" {
			return errors.New("not a boolean")
		}
	case UnsignedShort, UnsignedByte:
		if !unsignedPattern.MatchString(v) {
			return errors.New("not an unsigned integer")
		}
		bits := 16
		if t.Base == UnsignedByte {
			bits = 8
		}
		n, err := strconv.ParseUint(strings.TrimLeft(v, "+-"), 10, bits)
		if err != nil {
			return fmt.Errorf("out of range for %d bits", bits)
		}
		if err := t.inRange(int64(n)); err != nil { // #nosec G115 -- n fits 16 bits, parsed with that bit size
			return err
		}
	case Int:
		if !intPattern.MatchString(v) {
			return errors.New("not an integer")
		}
		n, err := strconv.ParseInt(strings.TrimPrefix(v, "+"), 10, 32)
		if err != nil {
			return errors.New("out of range for an int")
		}
		if err := t.inRange(n); err != nil {
			return err
		}
	case HexBinary:
		if len(v)%2 != 0 || !hexPattern.MatchString(v) {
			return errors.New("not hex")
		}
	case Base64Binary:
		if _, err := base64Octets(v); err != nil {
			return errors.New("not base64")
		}
	case Date:
		// libxml2 refuses whitespace around a date, though xs:date
		// collapses it; the models follow libxml2 (ADR 0016).
		if !isDate(raw) {
			return errors.New("not a date")
		}
	case DateTime:
		if !isDateTime(raw) {
			return errors.New("not a date and time")
		}
	}
	if n := t.length(v); (t.MinLen > 0 && n < t.MinLen) || (t.MaxLen > 0 && n > t.MaxLen) {
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

// checkURIReference is the part of RFC 3986 URI-reference syntax that
// libxml2 enforces on xs:anyURI, after it has escaped characters that are
// not allowed in a URI: a colon before any "/", "?" or "#" must end a valid
// scheme, an authority's port must be one or more digits, and every "%"
// must start an escape of two hex digits. The differential test pins each
// of these against xmllint.
func checkURIReference(v string) error {
	rest := v
	if i := strings.IndexAny(v, ":/?#"); i >= 0 && v[i] == ':' {
		if !schemePattern.MatchString(v[:i]) {
			return errors.New("not a URI")
		}
		rest = v[i+1:]
	}
	if auth, ok := strings.CutPrefix(rest, "//"); ok {
		if end := strings.IndexAny(auth, "/?#"); end >= 0 {
			auth = auth[:end]
		}
		if at := strings.LastIndex(auth, "@"); at >= 0 {
			auth = auth[at+1:]
		}
		host := auth
		if strings.HasPrefix(auth, "[") {
			if end := strings.Index(auth, "]"); end >= 0 {
				host = auth[end+1:]
			}
		}
		// After an unbracketed host, at most one colon, then a port of at
		// least one digit (libxml2 refuses an empty port).
		if c := strings.Index(host, ":"); c >= 0 {
			if port := host[c+1:]; port == "" || strings.Trim(port, "0123456789") != "" {
				return errors.New("not a URI")
			}
		}
	}
	for i := 0; i < len(v); i++ {
		if v[i] == '%' && (i+2 >= len(v) || !isHex(v[i+1]) || !isHex(v[i+2])) {
			return errors.New("not a URI")
		}
	}
	return nil
}

// isDate reports whether v is an xs:date as libxml2 reads one: a year of
// four or more digits (no leading zero beyond four, and not 0000), a month
// and a day that exist in it, and an optional time zone of at most 14
// hours.
func isDate(v string) bool {
	m := datePattern.FindStringSubmatch(v)
	if m == nil {
		return false
	}
	year, month, day, tz := m[2], m[3], m[4], m[5]
	if (len(year) > 4 && year[0] == '0') || strings.Trim(year, "0") == "" {
		return false
	}
	y, err := strconv.Atoi(year)
	if err != nil {
		return false
	}
	if m[1] == "-" {
		// libxml2 applies the leap year rule to a negative year as it is:
		// -0004 is a leap year.
		y = -y
	}
	mo, _ := strconv.Atoi(month)
	d, _ := strconv.Atoi(day)
	if mo < 1 || mo > 12 || d < 1 || d > daysIn(y, mo) {
		return false
	}
	if tz != "" && tz != "Z" {
		hh, _ := strconv.Atoi(tz[1:3])
		mm, _ := strconv.Atoi(tz[4:6])
		if hh > 14 || mm > 59 || (hh == 14 && mm != 0) {
			return false
		}
	}
	return true
}

// isDateTime reports whether v is an xs:dateTime as libxml2 reads one: a
// date as isDate reads it, "T", hours, minutes and seconds of two digits
// (24:00:00 allowed, as the end of the day), optional fractional seconds,
// and an optional time zone. libxml2 refuses leading whitespace, and
// trailing whitespace too unless a time zone precedes it.
func isDateTime(v string) bool {
	m := dateTimePattern.FindStringSubmatch(v)
	if len(m) != 7 || !isDate(m[1]+m[6]) {
		return false
	}
	hh, _ := strconv.Atoi(m[2])
	mm, _ := strconv.Atoi(m[3])
	ss, _ := strconv.Atoi(m[4])
	if hh == 24 {
		return mm == 0 && ss == 0 && strings.Trim(m[5], ".0") == ""
	}
	return hh < 24 && mm < 60 && ss < 60
}

// DateTimeValue is the time an xs:dateTime the schema has accepted names,
// in UTC. One without a time zone is read as UTC; 24:00:00 is the start of
// the next day. A time whose UTC year is outside 1 to 9999, which RFC 3339
// cannot write (nor Go's JSON), is an error.
func DateTimeValue(raw string) (time.Time, error) {
	m := dateTimePattern.FindStringSubmatch(raw)
	if len(m) != 7 {
		return time.Time{}, errors.New("not a date and time")
	}
	date := datePattern.FindStringSubmatch(m[1])
	if len(date) != 6 {
		return time.Time{}, errors.New("not a date and time")
	}
	year, err := strconv.Atoi(date[1] + date[2])
	if err != nil || year < -9999 || year > 9999 {
		return time.Time{}, errors.New("year out of range")
	}
	num := func(s string) int { n, _ := strconv.Atoi(s); return n }
	nanos := 0
	if frac := strings.TrimPrefix(m[5], "."); frac != "" {
		frac = (frac + "000000000")[:9]
		nanos = num(frac)
	}
	loc := time.UTC
	if tz := m[6]; tz != "" && tz != "Z" {
		offset := (num(tz[1:3])*60 + num(tz[4:6])) * 60
		if tz[0] == '-' {
			offset = -offset
		}
		loc = time.FixedZone(tz, offset)
	}
	t := time.Date(year, time.Month(num(date[3])), num(date[4]), num(m[2]), num(m[3]), num(m[4]), nanos, loc).UTC()
	if t.Year() < 1 || t.Year() > 9999 {
		return time.Time{}, errors.New("year out of range")
	}
	return t, nil
}

// daysIn is the number of days in month of the proleptic Gregorian year y.
func daysIn(y, month int) int {
	switch month {
	case 2:
		if (y%4 == 0 && y%100 != 0) || y%400 == 0 {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// TokenValue is the value of an xs:token (and every type derived from it)
// as the schema defines it: whitespace collapsed. Validation checks this
// value, so a handler must read this value too.
func TokenValue(raw string) string { return (&Simple{Base: Token}).whitespace(raw) }

// NormalizedStringValue is the value of an xs:normalizedString: tabs, line
// feeds and carriage returns replaced by spaces, nothing else changed.
func NormalizedStringValue(raw string) string {
	return (&Simple{Base: NormalizedString}).whitespace(raw)
}
