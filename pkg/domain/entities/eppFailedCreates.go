package entities

import "time"

// The failed-create budget (docs/epp/ARCHITECTURE.md §5, anti-dropcatch):
// domain creates answered 2302 because the name is registered are counted
// per registrar and per family over a sliding window, and a registrar over
// its budget, or in a family over its, is escalated: its creates are held
// (a tarpit), then refused with its session closed, then its login is
// suspended.

// EPPFailedCreateWindow is how far back creates are counted: the current
// minute and the four before it.
const EPPFailedCreateWindow = 5 * time.Minute

// EPPFailedCreateScope is one count of creates, a registrar's or a
// family's, with its budget.
type EPPFailedCreateScope struct {
	Key    string
	Budget EPPFailedCreateBudget
}

// EPPFailedCreateCheck is what is asked before a domain create.
type EPPFailedCreateCheck struct {
	// Login is the login sending the create, whose suspension is checked.
	Login string
	// Strikes names the strike count of the registrar sending it.
	Strikes string
	// Scopes are the counts to hold the create to: the registrar's, and
	// its family's if it is in one.
	Scopes []EPPFailedCreateScope
	// SuspendAfter is how many strikes suspend the login, for SuspendFor.
	SuspendAfter int
	SuspendFor   time.Duration
}

// EPPFailedCreateStanding is the answer: whether the login is suspended,
// and if not, how many strikes the registrar has, a strike being one
// create sent while over budget (this one included).
type EPPFailedCreateStanding struct {
	// Suspended is how much longer the login is suspended; 0 if it is not.
	Suspended time.Duration
	// NewlySuspended is set when this create's strike suspended it.
	NewlySuspended bool
	// Strikes is 0 when every scope is within its budget.
	Strikes int
	// Over is the index of the first scope over its budget.
	Over int
}

// EPPCreateOutcomes are a registrar's domain creates for one UTC day that
// created a domain and that were refused because the name is registered:
// the record a fee for excessive failed creates would be computed from.
type EPPCreateOutcomes struct {
	ClID      ClIDType  `json:"clid"`
	Day       time.Time `json:"day"`
	Succeeded int64     `json:"succeeded"`
	Failed    int64     `json:"failed"`
}
