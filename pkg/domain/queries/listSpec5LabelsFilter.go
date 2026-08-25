package queries

// ListSpec5LabelsFilter is the struct that contains the query for the list spec5 labels query
type ListSpec5LabelsFilter struct {
	LabelLike  string
	TypeEquals string
}

// ToQueryParams converts the Filter to a query string that can be appended to the URL
func (sf ListSpec5LabelsFilter) ToQueryParams() string {
	queryString := ""
	if sf.LabelLike != "" {
		queryString += "&label_like=" + sf.LabelLike
	}
	if sf.TypeEquals != "" {
		queryString += "&type_equals=" + sf.TypeEquals
	}
	return queryString
}
