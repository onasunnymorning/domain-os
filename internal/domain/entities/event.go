package entities

import (
	"encoding/json"
	"time"
)

const (
	AppAdminAPI = "AdminAPI"

	ObjectTypeTLD = "TLD"

	EventTypeAccreditation = "Accreditation"
	EventTypeUnknown       = "Unknown"

	EventResultSuccess = "Success"
	EventResultFailure = "Failure"
)

// Event struct defines a generic event throughout the system
type Event struct {
	App        string
	Actor      string
	Action     string
	ObjectType string
	ObjectID   string
	EndPoint   string
	Details    EventDetails
	Timestamp  time.Time
}

// ToJSONBytes converts the event to a JSON byte array
func (e *Event) ToJSONBytes() []byte {
	jsonBytes, err := json.Marshal(e)
	if err != nil {
		return nil
	}
	return jsonBytes
}

// ToJSONString converts the event to a JSON string
func (e *Event) ToJSONString() string {
	return string(e.ToJSONBytes())
}

// EventDetails struct describes the details of an event
type EventDetails struct {
	Result string
	Before interface{}
	After  interface{}
	Error  string
}

// NewEvent creates a new event
func NewEvent(app, actor, action, oType, oID, endPoint string) *Event {
	return &Event{
		App:        app,
		Actor:      actor,
		Action:     action,
		ObjectType: oType,
		ObjectID:   oID,
		EndPoint:   endPoint,
		Timestamp:  time.Now().UTC(),
	}
}

// IsError returns true if the event is an error
func (e *Event) IsError() bool {
	return e.Details.Result == EventResultFailure
}

// GetError returns the error message
func (e *Event) GetError() string {
	return e.Details.Error
}
