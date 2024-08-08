package response

// CountResult represents the response for a count operation
type CountResult struct {
	ObjectType string `json:"objectType"`
	Count      int    `json:"count"`
}
