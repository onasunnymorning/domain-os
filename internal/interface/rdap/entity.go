package rdap

import (
	"net/http"
	"strconv"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// entityResponse is a registrar answering an entity lookup (RFC 9083 §5.1).
type entityResponse struct {
	RDAPConformance []string   `json:"rdapConformance"`
	ObjectClassName string     `json:"objectClassName"`
	Handle          string     `json:"handle"`
	Roles           []string   `json:"roles"`
	PublicIDs       []publicID `json:"publicIds,omitempty"`
	VCardArray      []any      `json:"vcardArray"`
	Entities        []entity   `json:"entities,omitempty"`
	Events          []event    `json:"events"`
	Links           []link     `json:"links"`
	Notices         []notice   `json:"notices"`
}

// buildRegistrar is the entity response for a registrar, requested as r.
func (s *Server) buildRegistrar(r *http.Request, rr *entities.Registrar) entityResponse {
	e := registrarEntity(rr, true)
	resp := entityResponse{
		RDAPConformance: conformance,
		ObjectClassName: "entity",
		Handle:          e.Handle,
		Roles:           e.Roles,
		PublicIDs:       e.PublicIDs,
		VCardArray:      e.VCardArray,
		Entities:        e.Entities,
		Events: []event{
			{"registration", rdapDate(rr.CreatedAt)},
			{"last changed", rdapDate(rr.UpdatedAt)},
			{"last update of RDAP database", rdapDate(s.now())},
		},
		Links:   append([]link{{Value: s.requestURL(r), Rel: "self", Href: s.requestURL(r), Type: contentType}}, e.Links...),
		Notices: s.notices(r),
	}
	if resp.Notices == nil {
		resp.Notices = []notice{}
	}
	return resp
}

// registrarEntity is a registrar as an entity, with its abuse contact
// (gTLD RDAP Profile). full adds the registrar's own address, phone and
// email, which an entity lookup shows and a domain or nameserver response
// leaves to the registrar's own lookup.
func registrarEntity(rr *entities.Registrar, full bool) entity {
	props := []any{vcardProperty("fn", nil, "text", rr.Name)}
	if full {
		if addr := registrarAddress(rr); addr != nil {
			props = append(props, vcardProperty("adr", map[string]any{"cc": string(addr.CountryCode)}, "text",
				[]any{"", "", joinStreet(*addr), string(addr.City), string(addr.StateProvince), string(addr.PostalCode), ""}))
		}
		if rr.Voice != "" {
			props = append(props, vcardProperty("tel", map[string]any{"type": "voice"}, "uri", telURI(rr.Voice.String())))
		}
		if rr.Email != "" {
			props = append(props, vcardProperty("email", nil, "text", rr.Email))
		}
	}
	e := entity{
		ObjectClassName: "entity",
		Handle:          string(rr.ClID),
		Roles:           []string{"registrar"},
		VCardArray:      vcard(props...),
	}
	if rr.GurID > 0 {
		id := strconv.Itoa(rr.GurID)
		e.Handle = id
		e.PublicIDs = []publicID{{Type: "IANA Registrar ID", Identifier: id}}
	}
	if rr.URL != "" {
		e.Links = []link{{Rel: "about", Href: string(rr.URL), Type: "text/html"}}
	}
	abuse := []any{vcardProperty("fn", nil, "text", "")}
	if rr.Voice != "" {
		abuse = append(abuse, vcardProperty("tel", map[string]any{"type": "voice"}, "uri", telURI(rr.Voice.String())))
	}
	if rr.Email != "" {
		abuse = append(abuse, vcardProperty("email", nil, "text", rr.Email))
	}
	e.Entities = []entity{{ObjectClassName: "entity", Roles: []string{"abuse"}, VCardArray: vcard(abuse...)}}
	return e
}

// registrarAddress is the registrar's address, the internationalised form
// first.
func registrarAddress(rr *entities.Registrar) *entities.Address {
	var loc *entities.Address
	for _, pi := range rr.PostalInfo {
		if pi == nil || pi.Address == nil {
			continue
		}
		if pi.Type == entities.PostalInfoEnumType("int") {
			return pi.Address
		}
		loc = pi.Address
	}
	return loc
}
