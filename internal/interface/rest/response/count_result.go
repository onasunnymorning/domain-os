package response

import "time"

// CountResult represents the response for a count operation
type CountResult struct {
	// ObjectType is the type of object that was counted
	ObjectType string
	// Count is the number of objects that were counted
	Count int64
	// Timestamp is the time the count was taken
	Timestamp time.Time
	// Filter is the filter that was applied to the count
	Filter interface{}
}
