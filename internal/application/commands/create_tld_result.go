package commands

import "time"

// TLDResult is the result converting an entity TLD to a command TLDResult
type TLDResult struct {
	Name      string
	Type      string
	UName     string
	CreatedAt time.Time
	UpdatedAt time.Time
}
