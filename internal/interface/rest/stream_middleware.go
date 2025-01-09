package rest

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func StreamMiddleWare(es *services.EventService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO FIXME: parametrize
		c.Set("userid", "admin")
		c.Set("app", entities.AppAdminAPI)
		c.Set("correlation_id", c.Query("correlation_id"))

		// Create event and add to context
		e := NewEventFromContext(c)
		c.Set("event", e)

		// before request

		c.Next()
		if len(c.Errors) > 0 {
			// If there's an unhandled error, set event details accordingly.
			e.Details.Result = entities.EventResultFailure
			e.Details.Error = c.Errors.ByType(gin.ErrorTypeAny).String()
		}

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
		if es == nil {
			log.Println(e)
			return
		}

		// Omit info commands for admin API
		if e.Action == entities.EventTypeInfo && e.Source == entities.AppAdminAPI {
			return
		}

		// Send the event to Kafka
		err := es.SendStream(e)
		if err != nil {
			log.Println(err)
		}

	}
}
