package commands

// Commands of the launch application admin API (ADR 0024 §3): the operator's
// decisions about the applications filed in an application phase. The
// application is named in the path.

// ValidateApplicationCommand records the outcome of validating an
// application that is pendingValidation: valid moves it to
// pendingAllocation, invalid ends it with the reason.
type ValidateApplicationCommand struct {
	ApplicationID string `json:"-"`
	Valid         bool   `json:"valid"`
	// Reason says why an invalid application failed; required when Valid is
	// false.
	Reason string `json:"reason"`
}

// AllocateApplicationCommand registers the name to an application that is
// pendingAllocation, in its phase, and rejects the other applications for
// the name.
type AllocateApplicationCommand struct {
	ApplicationID string `json:"-"`
}

// RejectApplicationCommand ends a non-final application with the reason.
type RejectApplicationCommand struct {
	ApplicationID string `json:"-"`
	Reason        string `json:"reason" binding:"required"`
}
