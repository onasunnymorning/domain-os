package queries

// ListContactsFilter is a filter for the ListContacts query
type ListContactsFilter struct {
	RoidGreaterThan string
	RoidLessThan    string
	IdLike          string
	EmailLike       string
	ClidEquals      string
}

// ToQueryParams converts the filter to query parameters
func (f ListContactsFilter) ToQueryParams() string {
	var queryParams string

	if f.RoidGreaterThan != "" {
		queryParams += "&roid_greater_than=" + f.RoidGreaterThan
	}

	if f.RoidLessThan != "" {
		queryParams += "&roid_less_than=" + f.RoidLessThan
	}

	if f.IdLike != "" {
		queryParams += "&id_like=" + f.IdLike
	}

	if f.EmailLike != "" {
		queryParams += "&email_like=" + f.EmailLike
	}

	if f.ClidEquals != "" {
		queryParams += "&clid_equals=" + f.ClidEquals
	}

	return queryParams
}
