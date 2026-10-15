package commands

// EPPHostAddress is a <host:addr>: the address as given, and the IP version
// its ip attribute names ("v4" when the attribute is absent, as the schema
// defaults it).
type EPPHostAddress struct {
	Version string
	Address string
}

// EPPCreateHostCommand is a <host:create> (RFC 5732 §3.2.1), schema-valid
// but otherwise unchecked. As with domains, the sponsoring registrar is the
// session's, passed as a scope alongside (ADR 0006).
type EPPCreateHostCommand struct {
	Name      string
	Addresses []EPPHostAddress
}

// EPPUpdateHostCommand is a <host:update> (RFC 5732 §3.2.5): the addresses
// and statuses to add and remove, and a new name if the host is renamed.
type EPPUpdateHostCommand struct {
	Name            string
	AddAddresses    []EPPHostAddress
	RemoveAddresses []EPPHostAddress
	AddStatuses     []string
	RemoveStatuses  []string
	// NewName is the <host:chg> name, empty if the host keeps its name.
	NewName string
}
