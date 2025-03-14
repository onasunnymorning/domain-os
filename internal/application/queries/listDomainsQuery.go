package queries

import "time"

// ListDomainsQuery is the struct that contains the query for the list domains query
type ListDomainsFilter struct {
	// RoidEquals does an equals search on the Roid
	RoidEquals string
	// NameLike does a like search on the Name
	NameLike string
	// NameEquals does an equals search on the Name
	NameEquals string
	// TldEquals does an equals search on the Tld
	TldEquals string
	// ClIDEquals does an equals search on the ClID
	ClIDEquals string
	// ExpiresBefore does a less than search on the ExpiryDate
	ExpiresBefore time.Time
	// ExpiresAfter does a greater than search on the ExpiryDate
	ExpiresAfter time.Time
	// CreatedBefore does a less than search on the CreatedDate
	CreatedBefore time.Time
	// CreatedAfter does a greater than search on the CreatedDate
	CreatedAfter time.Time
}

type ListDomainsQuery struct {
	// PageSize is the number of items to return
	PageSize int
	// PageCursor is the cursor for the next page, based on the domain Roid so is a string
	PageCursor string
	// Filter is the filter for the list domain query
	Filter ListDomainsFilter
}
