package rest

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func PublishEvent() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create event and add to context
		e := entities.NewEvent("AdminAPI", "admin", GetActionFromContext(c), GetObjectTypeFromContext(c), c.Param("id"), c.Request.URL.RequestURI())
		c.Set("event", e)

		// before request

		c.Next()

		// after request

		log.Println(e)
		// access the status we are sending
		status := c.Writer.Status()
		log.Println(status)
	}
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
