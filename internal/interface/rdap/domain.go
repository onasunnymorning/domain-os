package rdap

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/rdappolicy"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// domainConformance is a domain response's rdapConformance: a domain
// response can redact (RFC 9537).
var domainConformance = append(slices.Clone(conformance), "redacted")

// domainResponse is an RFC 9083 §5.3 domain object, as the gTLD RDAP
// Profile shapes it.
type domainResponse struct {
	RDAPConformance []string        `json:"rdapConformance"`
	ObjectClassName string          `json:"objectClassName"`
	Handle          string          `json:"handle"`
	LDHName         string          `json:"ldhName"`
	UnicodeName     string          `json:"unicodeName,omitempty"`
	Status          []string        `json:"status"`
	Events          []event         `json:"events"`
	Entities        []entity        `json:"entities,omitempty"`
	Nameservers     []nameserverRef `json:"nameservers,omitempty"`
	SecureDNS       secureDNS       `json:"secureDNS"`
	Links           []link          `json:"links"`
	Notices         []notice        `json:"notices"`
	Redacted        []redaction     `json:"redacted,omitempty"`
}

// event is an RFC 9083 §4.5 event.
type event struct {
	EventAction string `json:"eventAction"`
	EventDate   string `json:"eventDate"`
}

// entity is an RFC 9083 §5.1 entity.
type entity struct {
	ObjectClassName string     `json:"objectClassName"`
	Handle          string     `json:"handle,omitempty"`
	Roles           []string   `json:"roles"`
	PublicIDs       []publicID `json:"publicIds,omitempty"`
	VCardArray      []any      `json:"vcardArray,omitempty"`
	Entities        []entity   `json:"entities,omitempty"`
	Links           []link     `json:"links,omitempty"`
}

type publicID struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
}

// nameserverRef is a nameserver inside a domain response.
type nameserverRef struct {
	ObjectClassName string       `json:"objectClassName"`
	Handle          string       `json:"handle,omitempty"`
	LDHName         string       `json:"ldhName"`
	IPAddresses     *ipAddresses `json:"ipAddresses,omitempty"`
	Links           []link       `json:"links,omitempty"`
}

type ipAddresses struct {
	V4 []string `json:"v4,omitempty"`
	V6 []string `json:"v6,omitempty"`
}

// secureDNS is RFC 9083 §5.3's DNSSEC member.
type secureDNS struct {
	DelegationSigned bool     `json:"delegationSigned"`
	DSData           []dsData `json:"dsData,omitempty"`
}

type dsData struct {
	KeyTag     uint16 `json:"keyTag"`
	Algorithm  uint8  `json:"algorithm"`
	DigestType uint8  `json:"digestType"`
	Digest     string `json:"digest"`
}

// rdapDate is the time format of every RDAP date: RFC 3339 in UTC.
func rdapDate(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// domainStatus maps EPP statuses (RFC 5731) and RGP statuses (RFC 3915) to
// RDAP's (RFC 8056 §2).
var domainStatus = map[string]string{
	entities.DomainStatusOK:                       "active",
	entities.DomainStatusInactive:                 "inactive",
	entities.DomainStatusClientTransferProhibited: "client transfer prohibited",
	entities.DomainStatusClientUpdateProhibited:   "client update prohibited",
	entities.DomainStatusClientDeleteProhibited:   "client delete prohibited",
	entities.DomainStatusClientRenewProhibited:    "client renew prohibited",
	entities.DomainStatusClientHold:               "client hold",
	entities.DomainStatusServerTransferProhibited: "server transfer prohibited",
	entities.DomainStatusServerUpdateProhibited:   "server update prohibited",
	entities.DomainStatusServerDeleteProhibited:   "server delete prohibited",
	entities.DomainStatusServerRenewProhibited:    "server renew prohibited",
	entities.DomainStatusServerHold:               "server hold",
	entities.DomainStatusPendingCreate:            "pending create",
	entities.DomainStatusPendingRenew:             "pending renew",
	entities.DomainStatusPendingTransfer:          "pending transfer",
	entities.DomainStatusPendingUpdate:            "pending update",
	entities.DomainStatusPendingRestore:           "pending restore",
	entities.DomainStatusPendingDelete:            "pending delete",
	entities.RGPStatusAddPeriod:                   "add period",
	entities.RGPStatusRenewPeriod:                 "renew period",
	entities.RGPStatusAutoRenewPeriod:             "auto renew period",
	entities.RGPStatusTransferPeriod:              "transfer period",
	entities.RGPStatusRedemptionPeriod:            "redemption period",
}

// statuses are d's RDAP statuses at now, each once.
func statuses(d *entities.Domain, now time.Time) []string {
	var out []string
	for _, s := range append(d.Status.StringSlice(), d.RGPStatuses(now)...) {
		if r, ok := domainStatus[s]; ok && !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		out = []string{"active"}
	}
	return out
}

// buildDomain is the domain response for rec, requested as r.
func (s *Server) buildDomain(r *http.Request, rec *interfaces.RDAPDomain, req rdappolicy.Requester) domainResponse {
	d := rec.Domain
	now := s.now()
	name := string(d.Name)
	resp := domainResponse{
		RDAPConformance: domainConformance,
		ObjectClassName: "domain",
		Handle:          d.RoID.String(),
		LDHName:         name,
		Status:          statuses(d, now),
		SecureDNS:       secureDNS{DelegationSigned: len(d.DSRecords) > 0},
		Links: []link{{
			Value: s.requestURL(r), Rel: "self", Type: contentType,
			Href: s.baseURL + string(d.TLDName) + "/domain/" + name,
		}},
		Notices: s.domainNotices(r, rec.TLDType),
	}
	if d.UName != "" && d.UName != d.Name {
		resp.UnicodeName = string(d.UName)
	}

	resp.Events = append(resp.Events, event{"registration", rdapDate(d.CreatedAt)})
	resp.Events = append(resp.Events, event{"expiration", rdapDate(d.ExpiryDate)})
	if !d.TransferredAt.IsZero() {
		resp.Events = append(resp.Events, event{"transfer", rdapDate(d.TransferredAt)})
	}
	resp.Events = append(resp.Events, event{"last changed", rdapDate(d.UpdatedAt)})
	resp.Events = append(resp.Events, event{"last update of RDAP database", rdapDate(now)})

	for _, ds := range d.DSRecords {
		resp.SecureDNS.DSData = append(resp.SecureDNS.DSData, dsData{KeyTag: ds.KeyTag, Algorithm: ds.Algorithm, DigestType: ds.DigestType, Digest: ds.Digest})
	}

	for _, h := range d.Hosts {
		ns := nameserverRef{
			ObjectClassName: "nameserver",
			Handle:          h.RoID.String(),
			LDHName:         string(h.Name),
			Links: []link{{
				Value: s.requestURL(r), Rel: "related", Type: contentType,
				Href: s.baseURL + string(d.TLDName) + "/nameserver/" + string(h.Name),
			}},
		}
		// Glue: the addresses of a host under this domain. Decided by name:
		// imported hosts can have addresses without InBailiwick set.
		if underDomain(string(h.Name), name) && len(h.Addresses) > 0 {
			ns.IPAddresses = &ipAddresses{}
			for _, a := range h.Addresses {
				if a.Is4() {
					ns.IPAddresses.V4 = append(ns.IPAddresses.V4, a.String())
				} else {
					ns.IPAddresses.V6 = append(ns.IPAddresses.V6, a.String())
				}
			}
		}
		resp.Nameservers = append(resp.Nameservers, ns)
	}

	if rec.Registrar != nil {
		resp.Entities = append(resp.Entities, registrarEntity(rec.Registrar, false))
		if base := string(rec.Registrar.RdapBaseURL); base != "" {
			if base[len(base)-1] != '/' {
				base += "/"
			}
			resp.Links = append(resp.Links, link{
				Value: s.requestURL(r), Rel: "related", Type: contentType,
				Href: base + "domain/" + name,
			})
		}
	}

	for _, role := range rdappolicy.Roles {
		c, ok := rec.Contacts[role]
		if !ok {
			continue
		}
		var form entities.PostalInfoEnumType
		if pi := postalInfo(c); pi != nil {
			form = pi.Type
		}
		dec := rdappolicy.Decide(req, rec.ContactPolicy, rec.Publication, role, c.Disclose, form)
		if !dec.Serve {
			continue
		}
		e, redacted := contactEntity(role, c, dec)
		resp.Entities = append(resp.Entities, e)
		resp.Redacted = append(resp.Redacted, redacted...)
		for _, rd := range redacted {
			s.metrics.redactions.WithLabelValues(string(d.TLDName), rd.field, rd.Method).Inc()
		}
	}
	return resp
}

// domainNotices are the notices a domain response carries: the terms of
// use, and for a gTLD the profile's Status Codes and RDDS Inaccuracy
// Complaint Form notices.
func (s *Server) domainNotices(r *http.Request, t entities.TLDType) []notice {
	out := s.notices(r)
	if t == entities.TLDTypeGTLD {
		out = append(out,
			notice{
				Title:       "Status Codes",
				Description: []string{"For more information on domain status codes, please visit https://icann.org/epp"},
				Links:       []link{{Value: s.requestURL(r), Rel: "glossary", Href: "https://icann.org/epp", Type: "text/html"}},
			},
			notice{
				Title:       "RDDS Inaccuracy Complaint Form",
				Description: []string{"URL of the ICANN RDDS Inaccuracy Complaint Form: https://icann.org/wicf"},
				Links:       []link{{Value: s.requestURL(r), Rel: "help", Href: "https://icann.org/wicf", Type: "text/html"}},
			},
		)
	}
	if out == nil {
		out = []notice{}
	}
	return out
}

// underDomain reports whether host is the domain or a name under it.
func underDomain(host, domain string) bool {
	host, domain = strings.ToLower(strings.TrimSuffix(host, ".")), strings.ToLower(strings.TrimSuffix(domain, "."))
	return host == domain || strings.HasSuffix(host, "."+domain)
}
