package rest

import (
	"fmt"
	"log"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func PublishEvent(p *kafka.Producer, topic string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO FIXME: parametrize
		c.Set("userid", "admin")
		c.Set("app", entities.AppAdminAPI)

		// Create event and add to context
		e := NewEventFromContext(c)
		c.Set("event", e)

		// before request

		c.Next()

		// after request

		// Set the Event.Details.Result based on the response status
		if c.Writer.Status() < 300 && c.Writer.Status() >= 200 {
			e.Details.Result = entities.EventResultSuccess
		} else {
			e.Details.Result = entities.EventResultFailure
		}
		// If we are pinging, don't log the event
		if e.EndPoint == "/ping" {
			return
		}
		// If there is no producer (e.g. in tests), just log the event
		if p == nil {
			log.Println(e)
			return
		}

		// Send the event to Kafka
		p.Produce(
			&kafka.Message{
				TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
				Value:          e.ToJSONBytes(),
			},
			nil,
		)

	}
}

// NewEventFromContext returns a new event based on the context
func NewEventFromContext(ctx *gin.Context) *entities.Event {
	return entities.NewEvent(ctx.GetString("app"), ctx.GetString("userid"), GetActionFromContext(ctx), GetObjectTypeFromContext(ctx), GetObjectIDFromContext(ctx), ctx.Request.URL.RequestURI())
}

// GetEventFromContext returns the event from the context
func GetEventFromContext(ctx *gin.Context) *entities.Event {
	e, ok := ctx.Get("event")
	if !ok {
		log.Println("Event not found in context")
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

	if slice[1] == "nndns" {
		return entities.ObjectTypeNNDN
	}
	if slice[1] == "contacts" {
		return entities.ObjectTypeContact
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
	}

	return entities.ObjectIDUnknown
}
