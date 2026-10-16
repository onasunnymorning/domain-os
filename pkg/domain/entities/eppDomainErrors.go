package entities

import "errors"

// Refusals of EPP domain commands, and the audit log's failure. The EPP
// adapter maps each refusal to a result code; the codes are listed in
// docs/epp/POLICY_REFERENCE.md. They are domain vocabulary (INV-09): the
// application layer wraps them with the detail.
var (
	// ErrEPPNotSponsor: the object is sponsored by another registrar
	// (ADR 0006: 2201).
	ErrEPPNotSponsor = errors.New("the object is sponsored by another registrar")
	// ErrEPPTLDNotServed: the name is under no TLD this registry serves.
	ErrEPPTLDNotServed = errors.New("the TLD is not served by this registry")
	// ErrEPPTLDNotOpen: the TLD has no general-availability phase in force.
	ErrEPPTLDNotOpen = errors.New("the TLD is not open for registration")
	// ErrEPPInvalidPeriod: the registration period is outside 1 year to the
	// phase's maximum horizon.
	ErrEPPInvalidPeriod = errors.New("the registration period is out of range")
	// ErrEPPPeriodNotWholeYears: a period in months that is not a whole
	// number of years.
	ErrEPPPeriodNotWholeYears = errors.New("the period must be whole years")
	// ErrEPPContactRequired: the phase's contact policy requires a contact
	// that was not given.
	ErrEPPContactRequired = errors.New("a contact the registry requires is missing")
	// ErrEPPContactProhibited: a contact was given that the phase's contact
	// policy does not accept.
	ErrEPPContactProhibited = errors.New("a contact the registry does not accept was given")
	// ErrEPPContactTypeMissing: a <domain:contact> without a type.
	ErrEPPContactTypeMissing = errors.New("every contact needs a type")
	// ErrEPPDuplicateContactType: two contacts of one type; a domain has at
	// most one of each.
	ErrEPPDuplicateContactType = errors.New("at most one contact of each type")
	// ErrEPPHostAttributes: nameservers given as host attributes; this
	// registry uses host objects.
	ErrEPPHostAttributes = errors.New("this registry uses host objects: give nameservers as <domain:hostObj>")
	// ErrEPPMaxSigLife: a DNSSEC maximum signature lifetime, which this
	// registry does not support (RFC 5910 §3.2: 2102).
	ErrEPPMaxSigLife = errors.New("maxSigLife is not supported")
	// ErrEPPKeyData: the DNSSEC key data interface; this registry takes DS
	// data only (RFC 5910 §4: 2306).
	ErrEPPKeyData = errors.New("this registry takes DS data, not key data: give <secDNS:dsData>")
	// ErrEPPDSAlgorithm: a DS record whose algorithm the registry does not
	// accept.
	ErrEPPDSAlgorithm = errors.New("DNSSEC algorithm not accepted")
	// ErrEPPDSDigestType: a DS record whose digest type the registry does
	// not accept.
	ErrEPPDSDigestType = errors.New("DS digest type not accepted")
	// ErrEPPTooManyDS: more DS records than the registry accepts on one
	// domain.
	ErrEPPTooManyDS = errors.New("too many DS records")
	// ErrEPPDomainContactNotSet: removing a contact the domain does not
	// have in that role.
	ErrEPPDomainContactNotSet = errors.New("the domain does not have that contact")
	// ErrEPPNameserverNotSet: removing a nameserver the domain does not
	// have.
	ErrEPPNameserverNotSet = errors.New("the domain does not have that nameserver")
	// ErrEPPDSNotSet: removing a DS record the domain does not have.
	ErrEPPDSNotSet = errors.New("the domain does not have that DS record")
	// ErrEPPDNSSECUrgent: a <secDNS:update urgent="true">, which this
	// registry does not support (RFC 5910 §5.2.5: 2102).
	ErrEPPDNSSECUrgent = errors.New("urgent DNSSEC updates are not supported")
	// ErrEPPAuthInfoRequired: removing a domain's authorization
	// information (<domain:null/>); the registry keeps one on every domain.
	ErrEPPAuthInfoRequired = errors.New("a domain keeps its authorization information: change it instead")
	// ErrEPPCurExpDate: a renew whose current expiry date is not the
	// domain's (RFC 5731 §3.2.3).
	ErrEPPCurExpDate = errors.New("the current expiry date given is not the domain's")
	// ErrEPPSubordinateHosts: a delete of a domain with host objects under
	// it (RFC 5731 §3.2.2).
	ErrEPPSubordinateHosts = errors.New("the domain has subordinate host objects: delete or rename them first")
	// ErrEPPAuditUnavailable wraps a failure to write the EPP audit log.
	ErrEPPAuditUnavailable = errors.New("EPP audit log unavailable")
)
