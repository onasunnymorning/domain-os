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

// We define a roid as such: {SnowFlakeID int64}_{objectIdentifier string}-{repositoryID string}
//
// The repository ID is the registry's EPP repository identifier, registered
// with IANA (https://www.iana.org/assignments/epp-repository-ids). It is
// configuration, not code: see RepositoryID and ADR 0017.
const (
	ROID_REGEX = `^(\w|_){1,80}-\w{1,8}$`

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
	ErrInvalidRepositoryID     = errors.New("invalid repository ID: 1 to 8 ASCII letters or digits")
	// ErrRepositoryIDChanged is a configured repository ID that differs from
	// the one the installation's ROIDs were issued with. Only the number of a
	// ROID is stored, so running with it would rename every object.
	ErrRepositoryIDChanged = errors.New("repository ID differs from the one this installation's ROIDs were issued with")
)

// RepositoryID is the EPP repository identifier this registry mints ROIDs
// with: the part after the "-". One installation has one, registered with
// IANA (ADR 0017). The IANA registry allows 1 to 8 letters or digits; this
// accepts the ASCII ones, which every registered ID uses, and so also fits
// RFC 5730's roidType pattern.
type RepositoryID string

var repositoryIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{1,8}$`)

// NewRepositoryID returns s as a RepositoryID, or ErrInvalidRepositoryID.
func NewRepositoryID(s string) (RepositoryID, error) {
	r := RepositoryID(s)
	if err := r.Validate(); err != nil {
		return "", err
	}
	return r, nil
}

// Validate checks r is 1 to 8 ASCII letters or digits.
func (r RepositoryID) Validate() error {
	if !repositoryIDPattern.MatchString(string(r)) {
		return fmt.Errorf("%w: %q", ErrInvalidRepositoryID, string(r))
	}
	return nil
}

// String implements the Stringer interface
func (r RepositoryID) String() string {
	return string(r)
}

// RoidTypeInt is a type for the Roid
type RoidType string

// NewRoidType creates a new instance of RoidType based on a snowflake ID +
// object identifier + repository ID. Storage keeps only the number, and the
// installation's repository ID is appended again on every read (ADR 0017).
func NewRoidType(uniqueID int64, objectIdentifier string, repository RepositoryID) (RoidType, error) {
	var object string
	switch objectIdentifier {
	case RoidTypeContact:
		object = CONTACT_ROID_ID
	case RoidTypeHost:
		object = HOST_ROID_ID
	case RoidTypeDomain:
		object = DOMAIN_ROID_ID
	default:
		return RoidType(""), ErrInvalidObjectIdentifier
	}
	if err := repository.Validate(); err != nil {
		return RoidType(""), err
	}
	return RoidType(fmt.Sprintf("%d_%s-%s", uniqueID, object, repository)), nil
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
// registry uses. It does not look at the repository ID. It is false for any conformant roid from another registry,
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
