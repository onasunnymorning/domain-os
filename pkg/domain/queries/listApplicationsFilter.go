package queries

import "net/url"

// ListApplicationsFilter narrows a page of launch applications to one
// status (ADR 0024 §3). Empty means every status.
type ListApplicationsFilter struct {
	Status string
}

// ToQueryParams implements ListItemsFilter, so a next-page link keeps the
// status it was asked with.
func (f ListApplicationsFilter) ToQueryParams() string {
	if f.Status == "" {
		return ""
	}
	return "&status=" + url.QueryEscape(f.Status)
}
