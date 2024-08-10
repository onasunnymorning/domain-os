package response

import "time"

// CountResult represents the response for a count operation
type CountResult struct {
	ObjectType string    `json:"objectType"`
	Count      int64     `json:"count"`
	Total      int64     `json:"total"`
	Timestamp  time.Time `json:"timestamp"`
}
