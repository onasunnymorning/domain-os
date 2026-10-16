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

// EPPUpdateDomainCommand is a <domain:update> (RFC 5731 §3.2.5), with its
// <secDNS:update> (RFC 5910 §5.2.5), schema-valid but otherwise unchecked.
type EPPUpdateDomainCommand struct {
	Name string
	// The <domain:add> and <domain:rem>: nameservers given as host objects,
	// contacts and statuses. HostAttributes is set when either gave
	// nameservers as host attributes.
	AddHostNames, RemoveHostNames []string
	HostAttributes                bool
	AddContacts, RemoveContacts   []EPPDomainContact
	AddStatuses, RemoveStatuses   []string
	// Registrant is the <domain:chg> registrant, nil if not changed; an
	// empty one removes it.
	Registrant *string
	// AuthInfo is the <domain:chg> password, nil if not changed.
	// AuthInfoNull is a <domain:null/>, which removes it.
	AuthInfo     *string
	AuthInfoNull bool
	// DNSSEC is the <secDNS:update>, nil if none was given.
	DNSSEC *EPPDNSSECUpdate
}

// EPPDNSSECUpdate is a <secDNS:update> (RFC 5910 §5.2.5).
type EPPDNSSECUpdate struct {
	Urgent bool
	// RemoveAll is <secDNS:rem><secDNS:all>true</secDNS:all>.
	RemoveAll bool
	RemoveDS  []EPPDSData
	AddDS     []EPPDSData
	// KeyData is set when the key data interface was used to add or
	// remove; MaxSigLife when one was given, in <secDNS:add> or
	// <secDNS:chg>.
	KeyData    bool
	MaxSigLife int
}

// EPPRenewDomainCommand is a <domain:renew> (RFC 5731 §3.2.3).
type EPPRenewDomainCommand struct {
	Name string
	// CurExpDate is the <domain:curExpDate>: a date, YYYY-MM-DD, with any
	// time zone dropped.
	CurExpDate string
	// Period and PeriodUnit are the <domain:period>; a zero Period means
	// none was given.
	Period     int
	PeriodUnit string
}
