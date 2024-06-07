package entities

import (
	"encoding/json"
	"time"
)

const (
	AppAdminAPI = "AdminAPI"

	ObjectTypeTLD = "TLD"

	EventTypeAccreditation = "Accreditation"

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

// ToBytes converts the event to a byte array
func (e *Event) ToBytes() []byte {
	jsonBytes, err := json.Marshal(e)
	if err != nil {
		return nil
	}
	return jsonBytes
}

// ToJSONString converts the event to a JSON string
func (e *Event) ToJSONString() string {
	jsonBytes, err := json.Marshal(e)
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

// EventDetails struct describes the details of an event
type EventDetails struct {
	Result string
	Before interface{}
	After  interface{}
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
