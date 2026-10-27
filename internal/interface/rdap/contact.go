package rdap

import (
	"github.com/onasunnymorning/domain-os/internal/application/rdappolicy"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// redaction is an RFC 9537 §4.2 redacted member.
type redaction struct {
	Name     redactionName `json:"name"`
	PrePath  string        `json:"prePath,omitempty"`
	PostPath string        `json:"postPath,omitempty"`
	// ReplacementPath locates the value shown instead (RFC 9537 §4.2).
	ReplacementPath string          `json:"replacementPath,omitempty"`
	PathLang        string          `json:"pathLang"`
	Method          string          `json:"method"`
	Reason          redactionReason `json:"reason"`

	field string // for metrics only
}

type redactionName struct {
	Type string `json:"type"`
}

type redactionReason struct {
	Type string `json:"type"`
}

// roleLabel names a role in the gTLD profile's redaction names
// ("Registrant Name", "Tech Email").
var roleLabel = map[rdappolicy.Role]string{
	rdappolicy.RoleRegistrant: "Registrant",
	rdappolicy.RoleAdmin:      "Admin",
	rdappolicy.RoleTech:       "Tech",
	rdappolicy.RoleBilling:    "Billing",
}

// fieldRedaction names a field's redaction and says where the value is, as
// a JSONPath within the role's entity. For a removed field the path is where
// it would have been (prePath); otherwise where it is (postPath).
var fieldRedaction = map[rdappolicy.Field]struct{ suffix, path string }{
	rdappolicy.FieldHandle: {"ID", ".handle"},
	rdappolicy.FieldName:   {"Name", ".vcardArray[1][?(@[0]=='fn')][3]"},
	rdappolicy.FieldOrg:    {"Organization", ".vcardArray[1][?(@[0]=='org')][3]"},
	rdappolicy.FieldStreet: {"Street", ".vcardArray[1][?(@[0]=='adr')][3][2]"},
	rdappolicy.FieldCity:   {"City", ".vcardArray[1][?(@[0]=='adr')][3][3]"},
	rdappolicy.FieldSP:     {"State/Province", ".vcardArray[1][?(@[0]=='adr')][3][4]"},
	rdappolicy.FieldPC:     {"Postal Code", ".vcardArray[1][?(@[0]=='adr')][3][5]"},
	rdappolicy.FieldCC:     {"Country", ".vcardArray[1][?(@[0]=='adr')][1].cc"},
	rdappolicy.FieldVoice:  {"Phone", ".vcardArray[1][?(@[1].type=='voice')][3]"},
	rdappolicy.FieldFax:    {"Fax", ".vcardArray[1][?(@[1].type=='fax')][3]"},
	rdappolicy.FieldEmail:  {"Email", ".vcardArray[1][?(@[0]=='email')][3]"},
}

// contactEntity is the contact c in role, as decided by dec: published
// fields with their values, withheld ones redacted, and the redacted members
// listing each withheld field that has a value. A field with no value is
// simply absent, and not listed (RFC 9537 separates the two).
func contactEntity(role rdappolicy.Role, c *entities.Contact, dec rdappolicy.ContactDecision) (entity, []redaction) {
	pi := postalInfo(c)
	var addr entities.Address
	if pi != nil && pi.Address != nil {
		addr = *pi.Address
	}
	value := map[rdappolicy.Field]string{
		rdappolicy.FieldHandle: c.RoID.String(),
		rdappolicy.FieldVoice:  c.Voice.String(),
		rdappolicy.FieldFax:    c.Fax.String(),
		rdappolicy.FieldEmail:  c.Email,
		rdappolicy.FieldStreet: joinStreet(addr),
		rdappolicy.FieldCity:   string(addr.City),
		rdappolicy.FieldSP:     string(addr.StateProvince),
		rdappolicy.FieldPC:     string(addr.PostalCode),
		rdappolicy.FieldCC:     string(addr.CountryCode),
	}
	if pi != nil {
		value[rdappolicy.FieldName] = string(pi.Name)
		value[rdappolicy.FieldOrg] = string(pi.Org)
	}

	dec = effective(dec, value)

	selector := "$.entities[?(@.roles[0]=='" + string(role) + "')]"
	var redacted []redaction
	// shown is the value a field is shown with, and whether it is shown.
	shown := func(f rdappolicy.Field) (string, bool) {
		v := value[f]
		if v == "" {
			return "", false
		}
		fd := dec.Fields[f]
		if fd.Publish {
			return v, true
		}
		rd := redaction{
			Name:     redactionName{Type: redactionType(role, f)},
			PathLang: "jsonpath",
			Method:   string(fd.Method),
			Reason:   redactionReason{Type: "Server policy"},
			field:    string(f),
		}
		path := selector + fieldRedaction[f].path
		switch fd.Method {
		case rdappolicy.Removal:
			rd.PrePath = path
			redacted = append(redacted, rd)
			return "", false
		case rdappolicy.ReplacementValue:
			// The email is replaced by a contact form (contact-uri below);
			// the email property itself is removed.
			rd.PrePath = path
			rd.ReplacementPath = selector + ".vcardArray[1][?(@[0]=='contact-uri')][3]"
			redacted = append(redacted, rd)
			return "", false
		default: // EmptyValue
			rd.PostPath = path
			redacted = append(redacted, rd)
			return "", true
		}
	}

	e := entity{ObjectClassName: "entity", Roles: []string{string(role)}}
	if v, ok := shown(rdappolicy.FieldHandle); ok {
		e.Handle = v
	}

	props := []any{}
	if v, ok := shown(rdappolicy.FieldName); ok {
		props = append(props, vcardProperty("fn", nil, "text", v))
	} else {
		// fn is required in a vCard; an absent name is shown empty.
		props = append(props, vcardProperty("fn", nil, "text", ""))
	}
	if v, ok := shown(rdappolicy.FieldOrg); ok {
		props = append(props, vcardProperty("org", nil, "text", v))
	}

	street, hasStreet := shown(rdappolicy.FieldStreet)
	city, hasCity := shown(rdappolicy.FieldCity)
	sp, hasSP := shown(rdappolicy.FieldSP)
	pc, hasPC := shown(rdappolicy.FieldPC)
	cc, hasCC := shown(rdappolicy.FieldCC)
	if hasStreet || hasCity || hasSP || hasPC || hasCC {
		params := map[string]any{}
		if hasCC {
			params["cc"] = cc // RFC 8605
		}
		props = append(props, vcardProperty("adr", params, "text", []any{"", "", street, city, sp, pc, ""}))
	}

	if v, ok := shown(rdappolicy.FieldVoice); ok {
		props = append(props, vcardProperty("tel", map[string]any{"type": "voice"}, "uri", telURI(v)))
	}
	if v, ok := shown(rdappolicy.FieldFax); ok {
		props = append(props, vcardProperty("tel", map[string]any{"type": "fax"}, "uri", telURI(v)))
	}
	if v, ok := shown(rdappolicy.FieldEmail); ok {
		props = append(props, vcardProperty("email", nil, "text", v))
	}
	if fd := dec.Fields[rdappolicy.FieldEmail]; fd.Method == rdappolicy.ReplacementValue && value[rdappolicy.FieldEmail] != "" {
		props = append(props, vcardProperty("contact-uri", nil, "uri", fd.Replacement))
	}

	e.VCardArray = vcard(props...)
	return e, redacted
}

// effective is dec as the jCard can carry it out, so the redacted members
// describe the response as it is. A jCard always has fn, so a withheld name
// is kept empty, never removed. The address is one property: once any part
// of it is shown, the withheld parts with a value are kept empty too.
func effective(dec rdappolicy.ContactDecision, value map[rdappolicy.Field]string) rdappolicy.ContactDecision {
	out := rdappolicy.ContactDecision{Serve: dec.Serve, Fields: make(map[rdappolicy.Field]rdappolicy.FieldDecision, len(dec.Fields))}
	for f, d := range dec.Fields {
		out.Fields[f] = d
	}
	empty := func(f rdappolicy.Field) {
		if d := out.Fields[f]; !d.Publish && d.Method == rdappolicy.Removal {
			out.Fields[f] = rdappolicy.FieldDecision{Method: rdappolicy.EmptyValue}
		}
	}
	empty(rdappolicy.FieldName)
	shown := false
	for _, f := range []rdappolicy.Field{rdappolicy.FieldStreet, rdappolicy.FieldCity, rdappolicy.FieldSP, rdappolicy.FieldPC, rdappolicy.FieldCC} {
		d := out.Fields[f]
		if value[f] != "" && (d.Publish || d.Method == rdappolicy.EmptyValue) {
			shown = true
		}
	}
	if shown {
		// The country lives in the address's parameters, so it can still
		// be left out: only the value slots are kept.
		for _, f := range []rdappolicy.Field{rdappolicy.FieldStreet, rdappolicy.FieldCity, rdappolicy.FieldSP, rdappolicy.FieldPC} {
			empty(f)
		}
	}
	return out
}

// redactionType is the gTLD profile's name for a redacted field: "Registry
// Registrant ID" for a handle, "Registrant Name" and so on for the rest.
func redactionType(role rdappolicy.Role, f rdappolicy.Field) string {
	if f == rdappolicy.FieldHandle {
		return "Registry " + roleLabel[role] + " ID"
	}
	return roleLabel[role] + " " + fieldRedaction[f].suffix
}

// postalInfo is the contact's postal information, the internationalised
// form first (RFC 5733 int is ASCII-only, so it is the form every client can
// show).
func postalInfo(c *entities.Contact) *entities.ContactPostalInfo {
	var loc *entities.ContactPostalInfo
	for _, pi := range c.PostalInfo {
		if pi == nil {
			continue
		}
		if pi.Type == entities.PostalInfoEnumType("int") {
			return pi
		}
		loc = pi
	}
	return loc
}

func joinStreet(a entities.Address) string {
	out := ""
	for _, s := range []entities.OptPostalLineType{a.Street1, a.Street2, a.Street3} {
		if s == "" {
			continue
		}
		if out != "" {
			out += ", "
		}
		out += string(s)
	}
	return out
}

// telURI is an EPP E.164 number (+1.5555551234) as a tel URI.
// An empty number (a redacted one) stays empty.
func telURI(e164 string) string {
	if e164 == "" {
		return ""
	}
	return "tel:" + e164
}

// vcard is a jCard (RFC 7095) with the given properties after the version.
func vcard(props ...any) []any {
	return []any{"vcard", append([]any{vcardProperty("version", nil, "text", "4.0")}, props...)}
}

// vcardProperty is one jCard property: name, parameters, type, value.
func vcardProperty(name string, params map[string]any, typ string, value any) []any {
	if params == nil {
		params = map[string]any{}
	}
	return []any{name, params, typ, value}
}
