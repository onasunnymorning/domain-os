package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func ContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userid", "admin")
		c.Set("app", entities.AppAdminAPI)
		c.Set("correlation_id", c.Query("correlation_id"))
		c.Set("trace_id", c.Query("trace_id"))
		c.Next()
	}
}
