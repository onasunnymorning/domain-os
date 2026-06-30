package rest

import (
	"context"

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

// ContextPropagationMiddleware copies specific keys from the Gin context into the standard Go context of the HTTP request.
func ContextPropagationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if uid, ok := c.Get("userid"); ok {
			if uidStr, ok := uid.(string); ok && uidStr != "" {
				ctx = context.WithValue(ctx, "userid", uidStr)
			}
		}
		if tid, ok := c.Get("trace_id"); ok {
			if tidStr, ok := tid.(string); ok && tidStr != "" {
				ctx = context.WithValue(ctx, "trace_id", tidStr)
			}
		}
		if cid, ok := c.Get("correlation_id"); ok {
			if cidStr, ok := cid.(string); ok && cidStr != "" {
				ctx = context.WithValue(ctx, "correlation_id", cidStr)
			}
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
