package entities

import "time"

type RegistrarLifecycleEvent struct {
	ClientID      string    // ClientID is the unique identifier of the client Registrar.ClID
	CorrelationID string    // CorrelationID is the identifier allowing to group events together in a business context (e.g. auto-renew-workflow-kdjsflkwr238fnelwkknk34ln5)
	TraceID       string    // TraceID is the unique identifier allowing tracing events across services (e.g. traceID set by activity or client, event gets processed by billing application, billing appliction logs can contain trace_id)
	TimeStamp     time.Time // TimeStamp is the time the transaction took place
}
