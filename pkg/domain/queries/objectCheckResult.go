package queries

// HostCheckResult is one name's answer to a <host:check>: whether the
// registrar can create a host by that name, and if not, why.
type HostCheckResult struct {
	Name      string
	Available bool
	Reason    string
}

// ContactCheckResult is one ID's answer to a <contact:check>: whether a
// contact can be created with that ID, and if not, why.
type ContactCheckResult struct {
	ID        string
	Available bool
	Reason    string
}
