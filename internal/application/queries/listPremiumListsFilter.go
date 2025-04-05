package queries

// ListPremiumListsFilter is the struct that contains the filter for the list premium lists query
type ListPremiumListsFilter struct {
	NameLike      string
	RyIDEquals    string
	CreatedBefore string
	CreatedAfter  string
}

// ToQueryParams converts the Filter to a query string that can be appended to the URL
func (f ListPremiumListsFilter) ToQueryParams() string {
	queryString := ""
	if f.NameLike != "" {
		queryString += "&name_like=" + f.NameLike
	}
	if f.RyIDEquals != "" {
		queryString += "&ry_id_equals=" + f.RyIDEquals
	}
	if f.CreatedBefore != "" {
		queryString += "&created_before=" + f.CreatedBefore
	}
	if f.CreatedAfter != "" {
		queryString += "&created_after=" + f.CreatedAfter
	}

	return queryString
}
