package queries

import "fmt"

// ListTombstonesFilter contains filter criteria for listing domain tombstones.
type ListTombstonesFilter struct {
	NameLike      string // ILIKE match on tombstone name
	NameEquals    string // exact match on tombstone name
	TLDEquals     string // exact match on TLD
	RegistrarClID string // exact match on registrar client ID
	PurgeReason   string // exact match on purge reason
}

// ToQueryParams converts the filter to a query string that can be appended to a URL.
func (f ListTombstonesFilter) ToQueryParams() string {
	q := ""
	if f.NameLike != "" {
		q += fmt.Sprintf("&name_like=%s", f.NameLike)
	}
	if f.NameEquals != "" {
		q += fmt.Sprintf("&name=%s", f.NameEquals)
	}
	if f.TLDEquals != "" {
		q += fmt.Sprintf("&tld=%s", f.TLDEquals)
	}
	if f.RegistrarClID != "" {
		q += fmt.Sprintf("&registrar=%s", f.RegistrarClID)
	}
	if f.PurgeReason != "" {
		q += fmt.Sprintf("&purge_reason=%s", f.PurgeReason)
	}
	return q
}
