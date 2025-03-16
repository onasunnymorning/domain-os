package queries

// ListNndnsFilter is the struct that contains the query for the list nndns query
type ListNndnsFilter struct {
	NameLike     string
	TldEquals    string
	ReasonEquals string
	ReasonLike   string
}

// ToQueryParams converts the ListNndnsFilter to a query string that can be appended to the URL
func (nf ListNndnsFilter) ToQueryParams() string {
	queryString := ""
	if nf.NameLike != "" {
		queryString += "&name_like=" + nf.NameLike
	}
	if nf.TldEquals != "" {
		queryString += "&tld_equals=" + nf.TldEquals
	}
	if nf.ReasonEquals != "" {
		queryString += "&reason_equals=" + nf.ReasonEquals
	}
	if nf.ReasonLike != "" {
		queryString += "&reason_like=" + nf.ReasonLike
	}
	return queryString
}
