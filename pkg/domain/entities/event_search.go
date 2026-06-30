package entities

import "time"

// EventSearchFilter defines the criteria for searching events.
type EventSearchFilter struct {
	Subject string     // exact match on subject (domain name, contact ID, etc.)
	Type    string     // exact match or prefix match with '*' suffix (e.g. "domain.*")
	Source  string     // exact match on source
	Actor   string     // exact match on actor
	RoID    string     // exact match on ROID
	After   *time.Time // inclusive lower bound on occurred_at
	Before  *time.Time // exclusive upper bound on occurred_at
	Limit   int        // max results (default 50, max 200)
	Cursor  string     // opaque cursor for pagination
}

// EventSearchResult contains the results of an event search.
type EventSearchResult struct {
	Events     []DomainEvent `json:"data"`
	NextCursor string        `json:"nextCursor,omitempty"`
	TotalCount int64         `json:"totalCount"`
	Tier       string        `json:"tier"` // "hot", "warm", "mixed"
}
