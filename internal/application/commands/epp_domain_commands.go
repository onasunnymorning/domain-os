package commands

// EPPCreateDomainCommand is a <domain:create> as the EPP server hands it to
// the registry: the sponsoring registrar is the session's, passed as a scope
// alongside, never part of the command (ADR 0006).
type EPPCreateDomainCommand struct {
	Name string
	// Years is the registration period. A period given in months has been
	// converted; one that is not whole years is refused before this.
	Years int
	// The contacts, by contact ID. Empty means not given.
	RegistrantID string
	AdminID      string
	TechID       string
	BillingID    string
	// HostNames are the nameservers, as host object names.
	HostNames []string
	AuthInfo  string
}
