package response

// DomainExpiryItem is the response structure for domain expiry list
type DomainRestoredItem struct {
	RoID string `json:"RoID"`
	Name string `json:"name"`
	ClID string `json:"ClID"`
}
