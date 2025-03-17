package queries

// ListRegistryOperatorsFilter represents the filter for the ListRegistryOperatores query
type ListRegistryOperatorsFilter struct {
	RyidLike  string
	NameLike  string
	EmailLike string
}

// ToQueryParams converts the ListRegistryOperatoresFilter to a query string that can be appended to the URL
func (rf ListRegistryOperatorsFilter) ToQueryParams() string {
	queryString := ""
	if rf.RyidLike != "" {
		queryString += "&ryid_like=" + rf.RyidLike
	}
	if rf.NameLike != "" {
		queryString += "&name_like=" + rf.NameLike
	}
	if rf.EmailLike != "" {
		queryString += "&email_like=" + rf.EmailLike
	}
	return queryString
}
