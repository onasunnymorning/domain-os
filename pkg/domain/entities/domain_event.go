package entities

import (
	"time"

	"github.com/google/uuid"
)

type DomainEvent struct {
	ID            string      `json:"id"`              // UUID v4
	Source        string      `json:"source"`          // "domain-os/api", "domain-os/worker", "domain-os/epp"
	Type          string      `json:"type"`            // "domain.registered", "domain.renewed", "registrar.created"
	Subject       string      `json:"subject"`         // "example.com", "REG-001"
	Description   string      `json:"description,omitempty"` // Human-readable summary, e.g. "Host ns1.dns.com added to domain foo.bar"
	Time          time.Time   `json:"time"`
	TraceID       string      `json:"trace_id,omitempty"`
	CorrelationID string      `json:"correlation_id,omitempty"`
	Data          interface{} `json:"data"`            // DomainLifeCycleEvent, RegistrarLifecycleEvent, etc.
	Command       interface{} `json:"command,omitempty"`
	BeforeState   interface{} `json:"before_state,omitempty"`
	AfterState    interface{} `json:"after_state,omitempty"`
	Actor         string      `json:"actor,omitempty"`
}

func NewDomainEvent(source, eventType, subject, description string, data interface{}) DomainEvent {
	return DomainEvent{
		ID:          uuid.NewString(),
		Source:      source,
		Type:        eventType,
		Subject:     subject,
		Description: description,
		Time:        time.Now().UTC(),
		Data:        data,
	}
}
