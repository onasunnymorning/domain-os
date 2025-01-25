package commands

// ToggleDomainStatusCommand is a command to toggle (set/unset) the status of a domain
type ToggleDomainStatusCommand struct {
	DomainName    string
	Status        string
	CorrelationID string
	TraceID       string
}
