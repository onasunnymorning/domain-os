package queries

// HostCheckResult is one name's answer to a <host:check>: whether the
// registrar can create a host by that name, and if not, why.
type HostCheckResult struct {
	Name      string
	Available bool
	Reason    string
}
