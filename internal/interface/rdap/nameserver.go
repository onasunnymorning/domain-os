package rdap

import (
	"net/http"
	"strings"

	"golang.org/x/net/idna"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// nameserverResponse is an RFC 9083 §5.2 nameserver object.
type nameserverResponse struct {
	RDAPConformance []string     `json:"rdapConformance"`
	ObjectClassName string       `json:"objectClassName"`
	Handle          string       `json:"handle"`
	LDHName         string       `json:"ldhName"`
	UnicodeName     string       `json:"unicodeName,omitempty"`
	Status          []string     `json:"status"`
	IPAddresses     *ipAddresses `json:"ipAddresses,omitempty"`
	Entities        []entity     `json:"entities,omitempty"`
	Events          []event      `json:"events"`
	Links           []link       `json:"links"`
	Notices         []notice     `json:"notices"`
}

// hostStatus maps RFC 5732 host statuses to RDAP's (RFC 8056 §2).
var hostStatus = map[string]string{
	entities.HostStatusOK:                     "active",
	entities.HostStatusLinked:                 "associated",
	entities.HostStatusPendingCreate:          "pending create",
	entities.HostStatusPendingDelete:          "pending delete",
	entities.HostStatusPendingUpdate:          "pending update",
	entities.HostStatusPendingTransfer:        "pending transfer",
	entities.HostStatusClientDeleteProhibited: "client delete prohibited",
	entities.HostStatusClientUpdateProhibited: "client update prohibited",
	entities.HostStatusServerDeleteProhibited: "server delete prohibited",
	entities.HostStatusServerUpdateProhibited: "server update prohibited",
}

// buildNameserver is the nameserver response for rec, requested as r.
func (s *Server) buildNameserver(r *http.Request, rec *interfaces.RDAPNameserver) nameserverResponse {
	h := rec.Host
	name := string(h.Name)
	resp := nameserverResponse{
		RDAPConformance: conformance,
		ObjectClassName: "nameserver",
		Handle:          h.RoID.String(),
		LDHName:         name,
		Status:          []string{},
		Links:           []link{{Value: s.requestURL(r), Rel: "self", Href: s.requestURL(r), Type: contentType}},
		Notices:         s.notices(r),
	}
	if resp.Notices == nil {
		resp.Notices = []notice{}
	}
	if strings.Contains(name, "xn--") {
		if u, err := idna.Lookup.ToUnicode(name); err == nil && u != name {
			resp.UnicodeName = u
		}
	}
	for _, st := range h.Status.StringSlice() {
		if mapped, ok := hostStatus[st]; ok {
			resp.Status = append(resp.Status, mapped)
		}
	}
	if len(resp.Status) == 0 {
		resp.Status = []string{"active"}
	}
	if len(h.Addresses) > 0 {
		resp.IPAddresses = &ipAddresses{}
		for _, a := range h.Addresses {
			if a.Is4() {
				resp.IPAddresses.V4 = append(resp.IPAddresses.V4, a.String())
			} else {
				resp.IPAddresses.V6 = append(resp.IPAddresses.V6, a.String())
			}
		}
	}
	if rec.Registrar != nil {
		resp.Entities = []entity{registrarEntity(rec.Registrar, false)}
	}
	resp.Events = []event{
		{"registration", rdapDate(h.CreatedAt)},
		{"last changed", rdapDate(h.UpdatedAt)},
		{"last update of RDAP database", rdapDate(s.now())},
	}
	if rec.Ambiguous {
		s.metrics.nameserversAmbiguous.Inc()
	}
	return resp
}
