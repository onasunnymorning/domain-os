package helpers

import (
	"context"

	"github.com/google/uuid"
)

// getCorrelationID returns the correlation ID from the context if it exists, otherwise it generates a new one (UUID prefixed with "newCorrelationID-")
func getCorrelationID(ctx context.Context) string {
	// We don't want a panic if the value is not found or not a string
	if correlationID, ok := ctx.Value("correlationID").(string); ok {
		return correlationID
	}
	return "newCorrelationID-" + uuid.New().String()
}
