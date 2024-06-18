package entities

import (
	"encoding/json"
	"slices"
	"time"
)

const (
	AppAdminAPI = "AdminAPI"

	ObjectTypeTLD           = "tld"
	ObjectTypeContact       = "contact"
	ObjectTypeNNDN          = "nndn"
	ObjectTypeAccreditation = "accreditation"
	ObjectTypeHost          = "host"
	ObjectTypeUnknown       = "unknown"

	ObjectIDUnknown = "unknown"

	EventTypeInfo   = "info"
	EventTypeCreate = "create"
	EventTypeUpdate = "update"
	EventTypeDelete = "delete"

	EventTypeAccreditation   = "Accreditation"
	EventTypeDeAccreditation = "DeAccreditation"
	EventTypeCreateContact   = "CreateContact"
	EventTypeUpdateContact   = "UpdateContact"
	EventTypeDeleteContact   = "DeleteContact"
	EventTypeUnknown         = "Unknown"

	EventResultSuccess = "Success"
	EventResultFailure = "Failure"
)

var (
	ValidEventResults = []EventResult{EventResultSuccess, EventResultFailure}
)

// EventResult struct defines the result of an event. This can be either Success or Failure
type EventResult string

// Validate checks if the event result is valid
func (e EventResult) Validate() bool {
	return slices.Contains(ValidEventResults, e)
}

// Event struct defines a generic event throughout the system
type Event struct {
	Source     string // The application that generated the event
	User       string // The user responsible for the event
	Action     string // METHOD? The action that was performed
	ObjectType string // The type of the object that was affected
	ObjectID   string // ROID? The ID of the object that was affected
	EndPoint   string // URL ? The endpoint that was called
	Details    EventDetails
	Timestamp  time.Time
}

// EventDetails struct describes the details of an event
type EventDetails struct {
	Result  EventResult
	Command interface{}
	Before  interface{}
	After   interface{}
	Error   error
}

// NewEvent creates a new event
func NewEvent(source, user, action, oType, oID, endPoint string) *Event {
	return &Event{
		Source:     source,
		User:       user,
		Action:     action,
		ObjectType: oType,
		ObjectID:   oID,
		EndPoint:   endPoint,
		Timestamp:  time.Now().UTC(),
	}
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

// IsError returns true if the event is resulted in an error
func (e *Event) IsError() bool {
	return e.Details.Result == EventResultFailure
}

// GetError returns the error message if available
func (e *Event) GetError() error {
	return e.Details.Error
}
