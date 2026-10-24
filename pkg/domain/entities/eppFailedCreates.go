package entities

import (
	"errors"
	"time"
)

// ErrInvalidCreateOutcomeRange is a request for daily create counts whose
// days are out of order or too far apart.
var ErrInvalidCreateOutcomeRange = errors.New("invalid range of days")

// ErrEPPSuspensionNotFound is a login that is not suspended.
var ErrEPPSuspensionNotFound = errors.New("the login is not suspended")

// EventEPPLoginSuspended records a login suspended for failed creates, by
// the EPP node that suspended it.
const EventEPPLoginSuspended = "epp.login.suspended"

// EPPLoginSuspendedData is an epp.login.suspended event's data: the
// suspension, the strike that caused it, and where the create came from.
type EPPLoginSuspendedData struct {
	EPPLoginSuspension
	Strikes  int    `json:"strikes"`
	ClientIP string `json:"client_ip"`
}

// EventEPPLoginSuspensionLifted records an operator ending a login's
// suspension for failed creates early.
const EventEPPLoginSuspensionLifted = "epp.login.suspension_lifted"

// EPPLoginSuspension is a login suspended for failed creates: by its
// registrar's strikes (Scope "registrar") or its family's ("family").
type EPPLoginSuspension struct {
	ClID   ClIDType  `json:"clid"`
	Login  string    `json:"login"`
	Scope  string    `json:"scope"`
	Family string    `json:"family,omitempty"`
	EndsAt time.Time `json:"ends_at"`
}

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
	// Login is the login sending the create, whose suspension is checked,
	// and ClID its registrar.
	Login string
	ClID  ClIDType
	// Scopes are the counts to hold the create to: the registrar's, and
	// its family's if it is in one. The first over its budget takes the
	// strike, so a family over budget is escalated as one, however many
	// of its members send the creates.
	Scopes []EPPFailedCreateScope
	// SuspendAfter is how many strikes suspend the login, for SuspendFor.
	SuspendAfter int
	SuspendFor   time.Duration
}

// EPPFailedCreateStanding is the answer: whether the login is suspended,
// and if not, how many strikes the scope over budget has, a strike being
// one create sent while it is over (this one included). A scope's strikes
// are shared by every login sending its creates: a login that carries on a
// run past SuspendAfter is suspended at its first strike.
type EPPFailedCreateStanding struct {
	// Suspended is how much longer the login is suspended; 0 if it is not.
	Suspended time.Duration
	// NewlySuspended is set when this create's strike suspended it.
	NewlySuspended bool
	// Strikes is 0 when every scope is within its budget.
	Strikes int
	// Over is the index of the first scope over its budget, or of the
	// scope whose strikes suspended the login.
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
