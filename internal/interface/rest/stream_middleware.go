package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func ContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userid", "admin")
		c.Set("app", entities.AppAdminAPI)

		// 1. Correlation ID: query parameter first (either correlation_id or correlationID), fallback to header, fallback to generating new UUID
		correlationID := c.Query("correlation_id")
		if correlationID == "" {
			correlationID = c.Query("correlationID")
		}
		if correlationID == "" {
			correlationID = c.GetHeader("X-Correlation-ID")
		}
		if correlationID == "" {
			correlationID = uuid.New().String()
		}
		c.Set("correlation_id", correlationID)

		// 2. Trace ID: query parameter first (either trace_id or traceID), fallback to header, fallback to generating new UUID
		traceID := c.Query("trace_id")
		if traceID == "" {
			traceID = c.Query("traceID")
		}
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
				ctx = appcontext.WithUserID(ctx, uidStr)
			}
		}
		if tid, ok := c.Get("trace_id"); ok {
			if tidStr, ok := tid.(string); ok && tidStr != "" {
				ctx = appcontext.WithTraceID(ctx, tidStr)
			}
		}
		if cid, ok := c.Get("correlation_id"); ok {
			if cidStr, ok := cid.(string); ok && cidStr != "" {
				ctx = appcontext.WithCorrelationID(ctx, cidStr)
			}
		}
		c.Request = c.Request.WithContext(ctx)
	}
}
