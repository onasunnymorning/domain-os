package response

import "time"

// DomainExpiryItem is the response structure for domain expiry list
type DomainExpiryItem struct {
	RoID       string    `json:"ro_id"`
	Name       string    `json:"name"`
	ExpiryDate time.Time `json:"expiry_date"`
}
