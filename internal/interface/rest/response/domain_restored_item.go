package response

// DomainExpiryItem is the response structure for domain expiry list
type DomainRestoredItem struct {
	RoID string `json:"ro_id"`
	Name string `json:"name"`
	ClID string `json:"cl_id"`
}
