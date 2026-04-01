package entities

import "time"

const (
	RegistrarEventTypeCreate = "CREATE"
	RegistrarEventTypeUpdate = "UPDATE"
	RegistrarEventTypeDelete = "DELETE"
)

// RegistrarEventType is the type of event (e.g. CREATE, UPDATE, DELETE)
type RegistrarEventType string

// RegistrarLifecycleEvent struct defines an event that is generated each time a registrar is created, updated or deleted
type RegistrarLifecycleEvent struct {
	// ClientID is the unique identifier of the client Registrar.ClID
	ClientID string
	// Type is the type of event (e.g. CREATE, UPDATE, DELETE)
	Type RegistrarEventType
	// CorrelationID is the identifier allowing to group events together in a business context (e.g. registrar-sync-workflow-kdjsflkwr238fnelwkknk34ln5)
	CorrelationID string
	// TraceID is the unique identifier allowing tracing events across services (e.g. traceID set by activity or client, event gets processed by billing application, billing appliction logs can contain trace_id)
	TraceID string
	// TimeStamp is the time the transaction took place
	TimeStamp time.Time
}

// NewRegistrarLifecycleEvent creates a new RegistrarLifecycleEvent with the given parameters
func NewRegistrarLifecycleEvent(clid string, eventType RegistrarEventType) *RegistrarLifecycleEvent {
	return &RegistrarLifecycleEvent{
		ClientID:  clid,
		Type:      eventType,
		TimeStamp: time.Now().UTC(),
	}
}
