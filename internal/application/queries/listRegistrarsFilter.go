package queries

import (
	"fmt"
)

// ListRegistrarsFilter is a filter for the ListRegistrars query
type ListRegistrarsFilter struct {
	ClidLike         string
	NameLike         string
	NickNameLike     string
	GuridEquals      int
	EmailLike        string
	StatusEquals     string
	IANAStatusEquals string
	AutorenewEquals  string
}

// ToQueryParams converts the Filter to a query string that can be appended to the URL
func (f ListRegistrarsFilter) ToQueryParams() string {
	queryString := ""
	if f.ClidLike != "" {
		queryString += "&clid_like=" + f.ClidLike
	}
	if f.NameLike != "" {
		queryString += "&name_like=" + f.NameLike
	}
	if f.NickNameLike != "" {
		queryString += "&nick_name_like=" + f.NickNameLike
	}
	if f.GuridEquals != 0 {
		queryString += fmt.Sprintf("&gur_id_equals=%d", f.GuridEquals)
	}
	if f.EmailLike != "" {
		queryString += "&email_like=" + f.EmailLike
	}
	if f.StatusEquals != "" {
		queryString += "&status_equals=" + f.StatusEquals
	}
	if f.IANAStatusEquals != "" {
		queryString += "&iana_status_equals=" + f.IANAStatusEquals
	}
	if f.AutorenewEquals != "" {
		queryString += "&autorenew_equals=" + f.AutorenewEquals
	}
	return queryString
}
