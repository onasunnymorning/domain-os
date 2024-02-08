package entities

import "time"

// Spec5Label is a struct representing an label blocked by RA Specification 5
type Spec5Label struct {
	Label     string    `json:"label" extensions:"x-order=0"`
	Type      string    `json:"type" extensions:"x-order=1"`
	CreatedAt time.Time `json:"created_at" extensions:"x-order=2"`
}
