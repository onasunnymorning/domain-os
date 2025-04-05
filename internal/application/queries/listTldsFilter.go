package queries

// ListTldsFilter is the struct that contains the filter for the list tld query
type ListTldsFilter struct {
	// NameLike does a like search on the Name
	NameLike string
	// TypeEquals does an equals search on the Type
	TypeEquals string
	// RyIDEquals does an equals search on the RyID
	RyIDEquals string
}

// ToQueryParams converts the Filter to a query string that can be appended to the URL
func (f ListTldsFilter) ToQueryParams() string {
	queryString := ""
	if f.NameLike != "" {
		queryString += "name_like=" + f.NameLike + "&"
	}
	if f.TypeEquals != "" {
		queryString += "type_equals=" + f.TypeEquals + "&"
	}
	if f.RyIDEquals != "" {
		queryString += "ry_id_equals=" + f.RyIDEquals + "&"
	}
	return queryString
}
