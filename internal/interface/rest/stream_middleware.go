package rest

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func ContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userid", "admin")
		c.Set("app", entities.AppAdminAPI)

		// 1. Correlation ID: query parameter first, fallback to header
		correlationID := c.Query("correlation_id")
		if correlationID == "" {
			correlationID = c.GetHeader("X-Correlation-ID")
		}
		c.Set("correlation_id", correlationID)

		// 2. Trace ID: query parameter first, fallback to header, fallback to generating new UUID
		traceID := c.Query("trace_id")
		if traceID == "" {
			traceID = c.GetHeader("X-Trace-ID")
		}
		if traceID == "" {
			traceID = uuid.New().String()
		}
		c.Set("trace_id", traceID)

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
