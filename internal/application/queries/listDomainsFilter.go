package queries

import "time"

// ListDomainsQuery is the struct that contains the query for the list domains query
type ListDomainsFilter struct {
	// RoidGreaterThan eliminates all domains with a Roid less than the given value
	RoidGreaterThan string
	// RoidLessThan eliminates all domains with a Roid greater than the given value
	RoidLessThan string
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

// ToQueryParams converts the ListDomainsFilter to a query string that can be appended to the URL
func (df ListDomainsFilter) ToQueryParams() string {
	queryString := ""
	if df.RoidGreaterThan != "" {
		queryString += "&roid_greater_than=" + df.RoidGreaterThan
	}
	if df.RoidLessThan != "" {
		queryString += "&roid_less_than=" + df.RoidLessThan
	}
	if df.NameLike != "" {
		queryString += "&name_like=" + df.NameLike
	}
	if df.NameEquals != "" {
		queryString += "&name_equals=" + df.NameEquals
	}
	if df.TldEquals != "" {
		queryString += "&tld_equals=" + df.TldEquals
	}
	if df.ClIDEquals != "" {
		queryString += "&clid_equals=" + df.ClIDEquals
	}
	if !df.ExpiresBefore.IsZero() {
		queryString += "&expires_before=" + df.ExpiresBefore.Format(time.RFC3339)
	}
	if !df.ExpiresAfter.IsZero() {
		queryString += "&expires_after=" + df.ExpiresAfter.Format(time.RFC3339)
	}
	if !df.CreatedBefore.IsZero() {
		queryString += "&created_before=" + df.CreatedBefore.Format(time.RFC3339)
	}
	if !df.CreatedAfter.IsZero() {
		queryString += "&created_after=" + df.CreatedAfter.Format(time.RFC3339)
	}
	return queryString
}
