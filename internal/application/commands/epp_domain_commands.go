package commands

// EPPCreateDomainCommand is a <domain:create> as the EPP server hands it to
// the registry: what the frame said, schema-valid but otherwise unchecked,
// so that every rule is applied in one order by the service. The sponsoring
// registrar is the session's, passed as a scope alongside, never part of
// the command (ADR 0006).
type EPPCreateDomainCommand struct {
	Name string
	// Period and PeriodUnit ("y" or "m") are the <domain:period>; a zero
	// Period means none was given.
	Period     int
	PeriodUnit string
	// RegistrantID is the <domain:registrant>, empty if none was given.
	RegistrantID string
	// Contacts are the <domain:contact> elements in order. Type is empty
	// when the element had no type attribute.
	Contacts []EPPDomainContact
	// HostNames are the nameservers given as host objects.
	HostNames []string
	// HostAttributes is set when nameservers were given as host
	// attributes (<domain:hostAttr>).
	HostAttributes bool
	AuthInfo       string
	// DNSSEC (RFC 5910), from a <secDNS:create> extension: the DS records
	// given; MaxSigLife if one was given (0 if not); and whether the key
	// data interface was used instead of DS data.
	DSData     []EPPDSData
	MaxSigLife int
	KeyData    bool
}

// EPPDSData is one <secDNS:dsData>.
type EPPDSData struct {
	KeyTag     int
	Algorithm  int
	DigestType int
	Digest     string
}

// EPPDomainContact is one <domain:contact>.
type EPPDomainContact struct {
	Type string // admin, billing or tech
	ID   string
}
