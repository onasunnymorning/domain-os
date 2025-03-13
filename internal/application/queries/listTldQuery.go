package queries

// ListTldQuery is the struct that contains the query for the list tld query
type ListTldQuery struct {
	// PageSize is the number of items to return
	PageSize int
	// PageCursor is the cursor for the next page, based on the tld name so is a string
	PageCursor string
	// Filter is the filter for the list tld query
	Filter ListTldQueryFilter
}

// ListTldQueryFilter is the struct that contains the filter for the list tld query
type ListTldQueryFilter struct {
	// NameLike does a like search on the Name
	NameLike string
	// TypeEquals does an equals search on the Type
	TypeEquals string
	// RyIDEquals does an equals search on the RyID
	RyIDEquals string
}
