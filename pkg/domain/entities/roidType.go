package entities

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"errors"
)

// <simpleType name="roidType">
//
//		<restriction base="token">
//	  	<pattern value="(\w|_){1,80}-\w{1,8}"/>
//		</restriction>
//
// </simpleType>

// We define a roid as such: {SnowFlakeID int64}_{objectIdentifier string}-{systemIdentifier string}
const (
	ROID_REGEX = `^(\w|_){1,80}-\w{1,8}$`

	EPP_REPOSITORY_ID = "APEX" // aka EPP REPOSITORY ID (IANA) TODO: make this an ENVAR and/or make available at tld level - ref: https://www.iana.org/assignments/epp-repository-ids/epp-repository-ids.xhtml

	CONTACT_ROID_ID = "CONT"
	HOST_ROID_ID    = "HOST"
	DOMAIN_ROID_ID  = "DOM"

	RoidTypeContact = "contact"
	RoidTypeHost    = "host"
	RoidTypeDomain  = "domain"
)

var (
	ErrInvalidRoid             = errors.New("invalid roid")
	ErrInvalidObjectIdentifier = errors.New("invalid object identifier: accepts ('contact', 'host', 'domain') only")
)

// RoidTypeInt is a type for the Roid
type RoidType string

// NewRoidType creates a new instance of RoidType based on a snowflake ID + object identifier + system identifier
func NewRoidType(uniqueID int64, objectIdentifier string) (RoidType, error) {
	switch objectIdentifier {
	case RoidTypeContact:
		return RoidType(fmt.Sprintf("%d_%s-%s", uniqueID, CONTACT_ROID_ID, EPP_REPOSITORY_ID)), nil
	case RoidTypeHost:
		return RoidType(fmt.Sprintf("%d_%s-%s", uniqueID, HOST_ROID_ID, EPP_REPOSITORY_ID)), nil
	case RoidTypeDomain:
		return RoidType(fmt.Sprintf("%d_%s-%s", uniqueID, DOMAIN_ROID_ID, EPP_REPOSITORY_ID)), nil
	default:
		return RoidType(""), ErrInvalidObjectIdentifier
	}
}

// Validate checks the roid against the EPP roidType pattern from RFC 5730,
// and nothing else.
//
// The {id}_{object}-{system} shape above is what this registry issues, not
// what the standard requires: RFC 5730 constrains a roid to the pattern and
// says nothing about its structure. An escrow deposit from another registry
// carries that registry's roids — ".radio" writes "Dztys40879-RADIO" — and
// they are valid. Requiring the underscore here made every foreign object
// fail validation and import; the shape is enforced where it belongs, in
// NewRoidType, which is the only thing that mints one.
func (r RoidType) Validate() error {
	if !roidPattern.MatchString(string(r)) {
		return ErrInvalidRoid
	}
	return nil
}

var roidPattern = regexp.MustCompile(ROID_REGEX)

// IsIssuedHere reports whether the roid has the shape this registry mints:
// {id}_{objectIdentifier}-{systemIdentifier}, with an object identifier this
// registry uses. It is false for any conformant roid from another registry,
// so a caller can hold its own objects to the local convention without
// holding a deposit's objects to it.
func (r RoidType) IsIssuedHere() bool {
	if !strings.Contains(string(r), "_") {
		return false
	}
	switch r.ObjectIdentifier() {
	case CONTACT_ROID_ID, HOST_ROID_ID, DOMAIN_ROID_ID:
		return true
	default:
		return false
	}
}

// String implements the Stringer interface
func (r RoidType) String() string {
	return string(r)
}

// Int64 returns the Unique ID part of the RoidType, which only a roid this
// registry minted has. A foreign roid yields an error rather than a number.
func (r RoidType) Int64() (int64, error) {
	id, _, _ := r.parts()
	return strconv.ParseInt(id, 10, 64)
}

// ObjectIdentifier returns the object identifier part of the RoidType, or ""
// for a roid that does not carry one. It never panics: a roid from another
// registry has no underscore, and splitting on one and indexing [1] used to
// take down whatever was holding it.
func (r RoidType) ObjectIdentifier() string {
	_, object, _ := r.parts()
	return object
}

// SystemIdentifier returns the system identifier part of the RoidType, or ""
// if the roid carries no "-".
func (r RoidType) SystemIdentifier() string {
	_, _, system := r.parts()
	return system
}

// parts splits {id}_{object}-{system}, returning "" for any part the roid does
// not have.
func (r RoidType) parts() (id, object, system string) {
	s := string(r)
	if i := strings.LastIndex(s, "-"); i >= 0 {
		s, system = s[:i], s[i+1:]
	}
	if i := strings.Index(s, "_"); i >= 0 {
		return s[:i], s[i+1:], system
	}
	return s, "", system
}
