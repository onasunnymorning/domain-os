package entities

import "errors"

// Refusals of EPP host commands (RFC 5732). Like the domain refusals, they
// are domain vocabulary (INV-09) that the application layer wraps with the
// detail, and the EPP adapter maps to result codes listed in
// docs/epp/POLICY_REFERENCE.md.
var (
	// ErrEPPInvalidHostName: a host name that is not a hostname of at
	// least two labels.
	ErrEPPInvalidHostName = errors.New("invalid host name")
	// ErrEPPSuperordinateDomainNotFound: a host name under a TLD this
	// registry serves, whose superordinate domain is not registered
	// (RFC 5732 §3.2.1).
	ErrEPPSuperordinateDomainNotFound = errors.New("the host's superordinate domain is not registered")
	// ErrEPPExternalHostAddress: an address on a host outside this
	// registry's TLDs. Only a host within them needs glue.
	ErrEPPExternalHostAddress = errors.New("a host outside this registry's TLDs takes no addresses")
	// ErrEPPHostAddressVersion: an address that is not of the IP version its
	// ip attribute names.
	ErrEPPHostAddressVersion = errors.New("the address is not of the IP version given")
	// ErrEPPHostAddressNotPublic: an address that is not public unicast:
	// loopback, unspecified, private, link-local, multicast or an IPv4
	// address written as IPv6.
	ErrEPPHostAddressNotPublic = errors.New("the address is not a public unicast address")
	// ErrEPPHostAddressNotSet: removing an address the host does not have.
	ErrEPPHostAddressNotSet = errors.New("the host does not have the address")
	// ErrEPPHostStatusNotClient: adding or removing a status a client does
	// not control. A client sets only clientDeleteProhibited and
	// clientUpdateProhibited.
	ErrEPPHostStatusNotClient = errors.New("a client can set only clientDeleteProhibited and clientUpdateProhibited")
	// ErrEPPHostStatusAlreadySet: adding a status the host already has.
	ErrEPPHostStatusAlreadySet = errors.New("the host already has the status")
	// ErrEPPHostStatusNotSet: removing a status the host does not have.
	ErrEPPHostStatusNotSet = errors.New("the host does not have the status")
	// ErrEPPHostDeleteProhibited: a delete of a host with a delete
	// prohibition (RFC 5732 §2.3).
	ErrEPPHostDeleteProhibited = errors.New("the host's status prohibits deleting it")
	// ErrEPPHostPending: a transform of a host with a pending action.
	ErrEPPHostPending = errors.New("the host has a pending action")
	// ErrEPPHostLinked: a delete of a host that is a nameserver of a domain
	// (RFC 5732 §3.2.2).
	ErrEPPHostLinked = errors.New("the host is a nameserver of a domain")
	// ErrEPPHostLinkedByOthers: a rename of a host that is a nameserver of a
	// domain another registrar sponsors (RST epp-23).
	ErrEPPHostLinkedByOthers = errors.New("the host is a nameserver of a domain another registrar sponsors")
	// ErrEPPHostNeedsAddress: a host within this registry's TLDs that is a
	// nameserver would be left without an address, and so without glue.
	ErrEPPHostNeedsAddress = errors.New("a nameserver within this registry's TLDs needs an address")
)
