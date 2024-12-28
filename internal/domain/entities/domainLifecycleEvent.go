package entities

import (
	"time"

	"github.com/google/uuid"
)

// DomainLifecycleEvent struct defines a domain lifecycle event. This is a specific type of event that is used to track domain lifecycle operations as well as being used for ICANN reporting or billing purposes if applicable.
type DomainLifecycleEvent struct {
	ID            uuid.UUID // The unique identifier of the event
	Timestamp     time.Time // The time that the event occured
	CorrelationID string    // The unique identifier of the event that this event is related to
	DomainName    string    // The domain name that the event is related to
	TLD           string    // The TLD that the event belongs to
	Quote         Quote     // The quote that contains all relavant pricing information. This also includes registrar, phase, domainname
}
