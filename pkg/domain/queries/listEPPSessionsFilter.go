package queries

import (
	"net/url"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// ListEPPSessionsFilter selects live EPP sessions. Empty fields match
// everything.
type ListEPPSessionsFilter struct {
	ClID         string
	LoginID      string
	ClientIP     string
	CredentialID uuid.UUID
}

// IsEmpty reports whether the filter selects every session.
func (f ListEPPSessionsFilter) IsEmpty() bool {
	return f.ClID == "" && f.LoginID == "" && f.ClientIP == "" && f.CredentialID == uuid.Nil
}

// Matches reports whether s is selected.
func (f ListEPPSessionsFilter) Matches(s *entities.EPPSession) bool {
	return (f.ClID == "" || string(s.ClID) == f.ClID) &&
		(f.LoginID == "" || string(s.LoginID) == f.LoginID) &&
		(f.ClientIP == "" || s.ClientIP.String() == f.ClientIP) &&
		(f.CredentialID == uuid.Nil || s.UsedCredential(f.CredentialID))
}

// ToQueryParams converts the filter to query parameters for a next link.
func (f ListEPPSessionsFilter) ToQueryParams() string {
	q := ""
	add := func(k, v string) {
		if v != "" {
			q += "&" + k + "=" + url.QueryEscape(v)
		}
	}
	add("clid", f.ClID)
	add("login", f.LoginID)
	add("client_ip", f.ClientIP)
	if f.CredentialID != uuid.Nil {
		add("credential_id", f.CredentialID.String())
	}
	return q
}
