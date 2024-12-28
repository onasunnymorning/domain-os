package rest

import (
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// NewEventFromContext returns a new event based on the context
func NewEventFromContext(ctx *gin.Context) *entities.Event {
	return entities.NewEvent(ctx.GetString("app"), ctx.GetString("userid"), GetActionFromContext(ctx), GetObjectTypeFromContext(ctx), GetObjectIDFromContext(ctx), ctx.Request.URL.RequestURI())
}

// GetEventFromContext returns the event from the context
func GetEventFromContext(ctx *gin.Context) *entities.Event {
	e, ok := ctx.Get("event")
	if !ok {
		log.Println("Event not found in context")
		return nil
	}
	return e.(*entities.Event)
}

// GetActionFromContext returns the action based on the HTTP method
func GetActionFromContext(ctx *gin.Context) string {
	switch ctx.Request.Method {
	case "GET":
		return entities.EventTypeInfo
	case "POST":
		return entities.EventTypeCreate
	case "PUT":
		return entities.EventTypeUpdate
	case "DELETE":
		return entities.EventTypeDelete
	default:
		return entities.EventTypeUnknown
	}
}

// GetObjectTypeFromContext returns the object type based on the URL
func GetObjectTypeFromContext(ctx *gin.Context) string {
	url := ctx.FullPath()

	slice := strings.Split(url, "/")

	if len(slice) < 2 {
		return entities.ObjectTypeUnknown
	}

	switch strings.ToLower(slice[1]) {
	case "nndns":
		return entities.ObjectTypeNNDN
	case "contacts":
		return entities.ObjectTypeContact
	case "tlds":
		// TLD CRUD functions
		if len(slice) == 2 {
			return entities.ObjectTypeTLD
		}
		// Phase CRUD functions
		if len(slice) >= 4 && slice[3] == "phases" {
			return entities.ObjectTypePhase
		}
	case "phases":
		return entities.ObjectTypePhase
	case "domains":
		return entities.ObjectTypeDomain
	case "accreditations":
		return entities.ObjectTypeAccreditation
	case "hosts":
		return entities.ObjectTypeHost
	}

	return entities.ObjectTypeUnknown
}

// GetObjectIDFromContext returns the object ID based on the URL
func GetObjectIDFromContext(ctx *gin.Context) string {
	objecttype := GetObjectTypeFromContext(ctx)

	switch objecttype {
	case entities.ObjectTypeContact:
		fmt.Println(ctx.Param("id"))
		return ctx.Param("id")
	case entities.ObjectTypeNNDN:
		return ctx.Param("name")
	case entities.ObjectTypeTLD:
		return ctx.Param("tldName")
	case entities.ObjectTypePhase:
		return ctx.Param("phaseName")
	case entities.ObjectTypeAccreditation:
		return ctx.Param("tldName")
	case entities.ObjectTypeHost:
		return ctx.Param("roid")
	}

	return entities.ObjectIDUnknown
}

// SetEventDetailsFromRequest sets the event details from the request
func SetEventDetailsFromRequest(c *gin.Context, action, objectType, objectID string) {
	event := GetEventFromContext(c)
	if event == nil {
		return
	}
	event.Action = action
	event.ObjectType = objectType
	event.ObjectID = objectID
	// Set the command details
	event.Details.Command = map[string]interface{}{
		"pathParams": c.Params,
		"query":      c.Request.URL.Query(),
	}
}

// The following allows type safe access to the event in the context
type ContextKey string

const eventCtxKey ContextKey = "event"

// SetEvent sets the event in the context
func SetEvent(c *gin.Context, e *entities.Event) {
	c.Set(string(eventCtxKey), e)
}

// GetEvent gets the event from the context
func GetEvent(c *gin.Context) *entities.Event {
	val, exists := c.Get(string(eventCtxKey))
	if !exists {
		return nil
	}
	evt, ok := val.(*entities.Event)
	if !ok {
		return nil
	}
	return evt
}
