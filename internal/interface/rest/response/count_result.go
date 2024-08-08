package response

import "time"

// CountResult represents the response for a count operation
type CountResult struct {
	ObjectType string    `json:"objectType"`
	Count      int       `json:"count"`
	Timestamp  time.Time `json:"timestamp"`
}
