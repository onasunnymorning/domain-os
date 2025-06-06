package entities

import (
	"time"

	"errors"
)

// NNDNState is a custom type for representing the state of an NNDN object.
type NNDNState string

// Constants for NNDNState.
const (
	NNDNStateBlocked  NNDNState = "blocked"  // Indicates the NNDN is not available for registration.
	NNDNStateWithheld NNDNState = "withheld" // Potentially a future registrable domain.
	NNDNStateMirrored NNDNState = "mirrored" // A mirrored IDN variant of a domain name.
)

var (
	ErrNNDNNotFound  = errors.New("NNDN not found")
	ErrInvalidNNDN   = errors.New("invalid NNDN")
	ErrDuplicateNNDN = errors.New("duplicate NNDN")
)

// NNDN represents a non-standard domain Name object in a domain Name registry.
// It is used for domain names that are not persisted as standard domain objects,
// such as reserved names or IDN variants. For example, a domain Name like "example.com"
// might have an IDN variant "例子.com" (represented in ASCII as "xn--fsq.com").
// IDN stands for Internationalized Domain Name. It refers to domain names
// that contain characters beyond the traditional ASCII (American Standard Code for Information Interchange) set.
// While "example.com" would be a standard domain object, its IDN variant "例子.com"
// would be managed as an NNDN. This approach allows registries to manage these
// special domain names separately from their main domain Name objects. NNDNs are
// essential for handling cases where domain names are reserved (not available for public
// registration), variants of existing domain names (IDNs), or for other administrative reasons.
// Ref: https://www.rfc-editor.org/rfc/rfc9022#name-nndn-object
type NNDN struct {
	// Unique identifier for the NNDN object.
	// The ASCII compatible (Punycode) representation of the NNDN.
	// For an IDN variant "例子.com", this would be "xn--fsq.com".
	Name DomainName

	// The Unicode representation of the NNDN.
	// For the IDN variant "xn--fsq.com", this would be "例子.com".
	UName DomainName

	// Identifier for the Top-Level Domain (TLD) associated with this NNDN.
	// For "例子.com", the TLDName might correspond to the ".com"
	TLDName DomainName

	// Indicates the state of the NNDN: 'blocked', 'withheld', or 'mirrored'.
	NameState NNDNState

	// Reason for the NNDN being blocked. This can be set by the user to create a basic form of categorization. Unlike NameState this can be chosen freely.
	Reason ClIDType

	// Timestamp of NNDN object creation. Example: 2024-01-19T15:04:05Z
	CreatedAt time.Time

	// Timestamp of the last update to the NNDN object. Example: 2024-01-20T15:04:05Z
	UpdatedAt time.Time
}

// NewNNDN creates a new NNDN object
func NewNNDN(name string) (*NNDN, error) {
	domain, err := NewDomainName(name)
	if err != nil {
		return nil, err
	}

	tld, err := NewDomainName(domain.ParentDomain()) // Untestable Domain and all its labels are already validated
	if err != nil {
		return nil, err
	}

	nndn := &NNDN{
		Name:      *domain,
		TLDName:   *tld,
		NameState: NNDNStateBlocked,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// Only set the UName if the domain is an IDN otherwise leave it empty
	if isIDN, _ := nndn.Name.IsIDN(); isIDN {
		uName, err := domain.ToUnicode() // Untestable Domain and all its labels are already validated
		if err != nil {
			return nil, err
		}
		nndn.UName = DomainName(uName)
	}

	return nndn, nil
}
