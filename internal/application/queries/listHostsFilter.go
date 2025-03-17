package queries

// ListHostsFilter is a filter for the ListHosts query
type ListHostsFilter struct {
	RoidGreaterThan string
	RoidLessThan    string
	ClidEquals      string
	NameLike        string
}

func (f ListHostsFilter) ToQueryParams() string {
	var queryParams string

	if f.RoidGreaterThan != "" {
		queryParams += "&roid_greater_than=" + f.RoidGreaterThan
	}

	if f.RoidLessThan != "" {
		queryParams += "&roid_less_than=" + f.RoidLessThan
	}

	if f.ClidEquals != "" {
		queryParams += "&clid_equals=" + f.ClidEquals
	}

	if f.NameLike != "" {
		queryParams += "&name_like=" + f.NameLike
	}

	return queryParams
}
