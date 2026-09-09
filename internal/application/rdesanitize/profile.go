package rdesanitize

import "github.com/onasunnymorning/domain-os/pkg/domain/entities"

// PolicyVersion identifies this profile. It is stamped on every derivative,
// every manifest and every run record, and the database refuses a second
// derivative of the same source under the same version — so changing the table
// below without changing this constant would make two different outputs
// indistinguishable. Bump it in the same commit as any classification change.
const PolicyVersion = "rde-baseline-v1"

// EPP namespace URIs the RDE mapping composes with. pkg/domain/entities
// declares the rde* ones; these are the object mappings underneath.
const (
	EPPURI     = "urn:ietf:params:xml:ns:epp-1.0"
	EPPComURI  = "urn:ietf:params:xml:ns:eppcom-1.0"
	DomainURI  = "urn:ietf:params:xml:ns:domain-1.0"
	HostURI    = "urn:ietf:params:xml:ns:host-1.0"
	ContactURI = "urn:ietf:params:xml:ns:contact-1.0"
	SecDNSURI  = "urn:ietf:params:xml:ns:secDNS-1.1"
	RGPURI     = "urn:ietf:params:xml:ns:rgp-1.0"
)

// aliases maps a namespace URI to the short name used in the table below. A
// namespace absent from this map is unclassified and quarantines the run: a
// vendor extension is exactly the thing that must not pass through by default.
var aliases = map[string]string{
	entities.RDE_URI:        "rde",
	entities.RDE_HEADER_URI: "rdeHeader",
	entities.DOMAIN_URI:     "rdeDomain",
	entities.HOST_URI:       "rdeHost",
	entities.CONTACT_URI:    "rdeContact",
	entities.REGISTRAR_URI:  "rdeRegistrar",
	entities.IDN_URI:        "rdeIDN",
	entities.NNDN_URI:       "rdeNNDN",
	entities.EPP_PARAMS_URI: "rdeEppParams",
	entities.RDE_POLICY_URI: "rdePolicy",
	EPPURI:                  "epp",
	EPPComURI:               "eppcom",
	DomainURI:               "domain",
	HostURI:                 "host",
	ContactURI:              "contact",
	SecDNSURI:               "secDNS",
	RGPURI:                  "rgp",
}

// ActionKind is what the rewriter does with an element.
type ActionKind int

const (
	// ActKeep copies the element and its text through unchanged.
	ActKeep ActionKind = iota
	// ActDropSubtree removes the element and everything inside it.
	ActDropSubtree
	// ActKeepSubtree copies the element and everything inside it without
	// classifying the descendants. Reserved for blocks that carry protocol
	// policy rather than object data.
	ActKeepSubtree
	// ActSynthesize replaces the text with a deterministic synthetic value.
	ActSynthesize
	// ActTokenize replaces the text with a deterministic HMAC token.
	ActTokenize
	// ActRewriteFQDN replaces the registry suffix in a fully-qualified name.
	ActRewriteFQDN
)

// ValueKind selects the shape of a synthesized or tokenized value. The shape
// matters: the derivative must still satisfy the RDE and EPP schemas.
type ValueKind string

const (
	ValNone        ValueKind = ""
	ValContactID   ValueKind = "contactId"   // eppcom:clIDType, 3-16 chars
	ValContactROID ValueKind = "contactRoid" // eppcom:roidType, \w{1,80}-\w{1,8}
	ValPersonName  ValueKind = "personName"
	ValEmail       ValueKind = "email"
)

// Action is one classification decision.
type Action struct {
	Kind  ActionKind
	Value ValueKind
}

var (
	keep        = Action{Kind: ActKeep}
	keepSubtree = Action{Kind: ActKeepSubtree}
	drop        = Action{Kind: ActDropSubtree}
	rewriteFQDN = Action{Kind: ActRewriteFQDN}
	tokenID     = Action{Kind: ActTokenize, Value: ValContactID}
	tokenROID   = Action{Kind: ActTokenize, Value: ValContactROID}
	synthName   = Action{Kind: ActSynthesize, Value: ValPersonName}
	synthEmail  = Action{Kind: ActSynthesize, Value: ValEmail}
)

// Profile is the versioned classification table.
//
// Lookup is by "parentAlias:parentLocal>alias:local" first and "alias:local"
// second. The two-segment form is not decoration: rdeContact:org means a
// registrant's organisation under postalInfo and a disclosure preference under
// disclose, and those must not share a decision.
//
// Reading of the field-policy baseline, recorded here so a reviewer can
// overturn it in one place:
//
//   - Mandatory direct identifiers are replaced with deterministic synthetic
//     values (a schema-valid derivative still needs a contact name and email);
//     optional ones are removed outright (org, street, voice, fax). Removal is
//     preferred wherever the schema permits it.
//   - Country, city, state/province and postal code are retained: the ticket
//     lists them as analytical metadata required for the approved analysis.
//     They are also why the output is pseudonymized and not anonymous.
//   - Registrar records are retained in full. A registrar is an organisation
//     whose contact details ICANN already publishes, and the ticket lists the
//     agreed registrar identifier as operational metadata to retain. To treat
//     registrar postal data as personal instead, change the rdeRegistrar
//     entries below and bump PolicyVersion.
//   - Domain and host ROIDs are retained while contact ROIDs are tokenised.
//     A contact ROID is a second handle for the same person and would defeat
//     tokenising the contact id; a domain ROID identifies an object whose name
//     the derivative keeps by design, so tokenising it would buy nothing.
//   - secDNS keyData is retained. DNSKEY public-key material is published in
//     DNS; the private key material RFC 8909 warns about never appears in a
//     deposit and has no element here to carry it.
type Profile struct {
	elements   map[string]Action
	attributes map[string][]string
}

// BaselineProfile returns the shipped profile. It is built fresh so a caller
// cannot mutate the shared table.
func BaselineProfile() *Profile {
	return &Profile{elements: baselineElements(), attributes: baselineAttributes()}
}

// Version returns the policy version this profile implements.
func (p *Profile) Version() string { return PolicyVersion }

// Alias returns the short name for a namespace URI and whether it is known.
func Alias(uri string) (string, bool) {
	a, ok := aliases[uri]
	return a, ok
}

// Action returns the classification for an element, given its parent. The
// boolean is false when the profile does not classify it, which quarantines
// the run.
func (p *Profile) Action(parentKey, key string) (Action, bool) {
	if parentKey != "" {
		if a, ok := p.elements[parentKey+">"+key]; ok {
			return a, true
		}
	}
	a, ok := p.elements[key]
	return a, ok
}

// AllowsAttribute reports whether an attribute may appear on an element.
// Namespace declarations are handled by the rewriter and never reach here.
func (p *Profile) AllowsAttribute(parentKey, key, attr string) bool {
	if parentKey != "" {
		if allowed, ok := p.attributes[parentKey+">"+key]; ok {
			return contains(allowed, attr)
		}
	}
	if allowed, ok := p.attributes[key]; ok {
		return contains(allowed, attr)
	}
	return false
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// objectRoots are the elements counted as RDE objects in the manifest.
var objectRoots = map[string]string{
	"rdeDomain:domain":       entities.DOMAIN_URI,
	"rdeHost:host":           entities.HOST_URI,
	"rdeContact:contact":     entities.CONTACT_URI,
	"rdeRegistrar:registrar": entities.REGISTRAR_URI,
	"rdeIDN:idnTableRef":     entities.IDN_URI,
	"rdeNNDN:NNDN":           entities.NNDN_URI,
}

func baselineElements() map[string]Action {
	m := map[string]Action{
		// ---- deposit envelope (RFC 8909) ----
		"rde:deposit":   keep,
		"rde:watermark": keep,
		"rde:rdeMenu":   keep,
		"rde:version":   keep,
		"rde:objURI":    keep,
		"rde:contents":  keep,
		"rde:deletes":   keep,

		// ---- header (RFC 9022) ----
		"rdeHeader:header": keep,
		"rdeHeader:tld":    rewriteFQDN,
		"rdeHeader:count":  keep,

		// ---- domain ----
		"rdeDomain:domain":       keep,
		"rdeDomain:name":         rewriteFQDN,
		"rdeDomain:uName":        rewriteFQDN,
		"rdeDomain:originalName": rewriteFQDN,
		"rdeDomain:roid":         keep,
		"rdeDomain:idnTableId":   keep,
		"rdeDomain:status":       keep,
		"rdeDomain:rgpStatus":    keep,
		"rdeDomain:registrant":   tokenID,
		"rdeDomain:contact":      tokenID,
		"rdeDomain:ns":           keep,
		"rdeDomain:clID":         keep,
		"rdeDomain:crRr":         keep,
		"rdeDomain:crDate":       keep,
		"rdeDomain:exDate":       keep,
		"rdeDomain:upRr":         keep,
		"rdeDomain:upDate":       keep,
		"rdeDomain:trDate":       keep,
		"rdeDomain:secDNS":       keep,
		"rdeDomain:trnData":      keep,
		"rdeDomain:trStatus":     keep,
		"rdeDomain:reRr":         keep,
		"rdeDomain:reDate":       keep,
		"rdeDomain:acRr":         keep,
		"rdeDomain:acDate":       keep,
		"rdeDomain:authInfo":     drop,

		// nameservers and glue (domain-1.0)
		"domain:hostObj":  rewriteFQDN,
		"domain:hostAttr": keep,
		"domain:hostName": rewriteFQDN,
		"domain:hostAddr": keep,
		// credentials, wherever they appear
		"domain:authInfo":  drop,
		"contact:authInfo": drop,
		"epp:authInfo":     drop,

		// ---- DNSSEC (secDNS-1.1): published in DNS, retained ----
		"secDNS:dsData":     keep,
		"secDNS:keyTag":     keep,
		"secDNS:alg":        keep,
		"secDNS:digestType": keep,
		"secDNS:digest":     keep,
		"secDNS:maxSigLife": keep,
		"secDNS:keyData":    keep,
		"secDNS:flags":      keep,
		"secDNS:protocol":   keep,
		"secDNS:pubKey":     keep,

		// ---- RGP ----
		"rgp:rgpStatus": keep,

		// ---- host ----
		"rdeHost:host":   keep,
		"rdeHost:name":   rewriteFQDN,
		"rdeHost:roid":   keep,
		"rdeHost:status": keep,
		"rdeHost:addr":   keep,
		"rdeHost:clID":   keep,
		"rdeHost:crRr":   keep,
		"rdeHost:crDate": keep,
		"rdeHost:upRr":   keep,
		"rdeHost:upDate": keep,
		"rdeHost:trDate": keep,

		// ---- contact ----
		"rdeContact:contact":    keep,
		"rdeContact:id":         tokenID,
		"rdeContact:roid":       tokenROID,
		"rdeContact:status":     keep,
		"rdeContact:postalInfo": keep,
		"rdeContact:clID":       keep,
		"rdeContact:crRr":       keep,
		"rdeContact:crDate":     keep,
		"rdeContact:upRr":       keep,
		"rdeContact:upDate":     keep,
		"rdeContact:trDate":     keep,
		"rdeContact:authInfo":   drop,
		"rdeContact:disclose":   keep,
		// mandatory identifiers are replaced, optional ones removed
		"rdeContact:postalInfo>rdeContact:name": synthName,
		"rdeContact:postalInfo>rdeContact:org":  drop,
		"rdeContact:postalInfo>rdeContact:addr": keep,
		"rdeContact:addr>rdeContact:street":     drop,
		"rdeContact:addr>rdeContact:city":       keep,
		"rdeContact:addr>rdeContact:sp":         keep,
		"rdeContact:addr>rdeContact:pc":         keep,
		"rdeContact:addr>rdeContact:cc":         keep,
		"rdeContact:contact>rdeContact:voice":   drop,
		"rdeContact:contact>rdeContact:fax":     drop,
		"rdeContact:contact>rdeContact:email":   synthEmail,
		// disclosure preferences are empty flag elements, not values
		"rdeContact:disclose>rdeContact:name":  keep,
		"rdeContact:disclose>rdeContact:org":   keep,
		"rdeContact:disclose>rdeContact:addr":  keep,
		"rdeContact:disclose>rdeContact:voice": keep,
		"rdeContact:disclose>rdeContact:fax":   keep,
		"rdeContact:disclose>rdeContact:email": keep,

		// ---- registrar: business data, retained (see the type comment) ----
		"rdeRegistrar:registrar":  keep,
		"rdeRegistrar:id":         keep,
		"rdeRegistrar:name":       keep,
		"rdeRegistrar:gurid":      keep,
		"rdeRegistrar:status":     keep,
		"rdeRegistrar:postalInfo": keep,
		"rdeRegistrar:addr":       keep,
		"rdeRegistrar:street":     keep,
		"rdeRegistrar:city":       keep,
		"rdeRegistrar:sp":         keep,
		"rdeRegistrar:pc":         keep,
		"rdeRegistrar:cc":         keep,
		"rdeRegistrar:org":        keep,
		"rdeRegistrar:voice":      keep,
		"rdeRegistrar:fax":        keep,
		"rdeRegistrar:email":      keep,
		"rdeRegistrar:url":        keep,
		"rdeRegistrar:whoisInfo":  keep,
		"rdeRegistrar:crDate":     keep,
		"rdeRegistrar:upDate":     keep,

		// ---- IDN table references ----
		"rdeIDN:idnTableRef": keep,
		"rdeIDN:url":         keep,
		"rdeIDN:urlPolicy":   keep,
		"rdeIDN:id":          keep,

		// ---- NNDN ----
		"rdeNNDN:NNDN":         keep,
		"rdeNNDN:aName":        rewriteFQDN,
		"rdeNNDN:uName":        rewriteFQDN,
		"rdeNNDN:originalName": rewriteFQDN,
		"rdeNNDN:idnTableId":   keep,
		"rdeNNDN:nameState":    keep,
		"rdeNNDN:crDate":       keep,
		"rdeNNDN:upDate":       keep,

		// ---- EPP parameters and policy: protocol metadata, no object data ----
		"rdeEppParams:eppParams":    keep,
		"rdeEppParams:version":      keep,
		"rdeEppParams:lang":         keep,
		"rdeEppParams:objURI":       keep,
		"rdeEppParams:svcExtension": keep,
		"rdeEppParams:extURI":       keep,
		"rdeEppParams:dcp":          keepSubtree,
		"rdePolicy:policy":          keep,
	}
	return m
}

func baselineAttributes() map[string][]string {
	return map[string][]string{
		"rde:deposit":     {"type", "id", "prevId", "resend"},
		"rdeHeader:count": {"uri"},

		"rdeDomain:status":    {"s", "lang"},
		"rdeDomain:rgpStatus": {"s", "lang"},
		"rdeDomain:contact":   {"type"},
		"rdeDomain:crRr":      {"client"},
		"rdeDomain:upRr":      {"client"},
		"rdeDomain:reRr":      {"client"},
		"rdeDomain:acRr":      {"client"},
		"rgp:rgpStatus":       {"s", "lang"},
		"domain:hostAddr":     {"ip"},

		"rdeHost:status": {"s", "lang"},
		"rdeHost:addr":   {"ip"},
		"rdeHost:crRr":   {"client"},
		"rdeHost:upRr":   {"client"},

		"rdeContact:status":                   {"s", "lang"},
		"rdeContact:postalInfo":               {"type"},
		"rdeContact:disclose":                 {"flag"},
		"rdeContact:contact>rdeContact:voice": {"x"},
		"rdeContact:contact>rdeContact:fax":   {"x"},
		"rdeContact:disclose>rdeContact:name": {"type"},
		"rdeContact:disclose>rdeContact:org":  {"type"},
		"rdeContact:disclose>rdeContact:addr": {"type"},
		"rdeContact:crRr":                     {"client"},
		"rdeContact:upRr":                     {"client"},

		"rdeRegistrar:status":     {"s", "lang"},
		"rdeRegistrar:postalInfo": {"type"},
		"rdeRegistrar:voice":      {"x"},
		"rdeRegistrar:fax":        {"x"},

		"rdeIDN:idnTableRef": {"id"},
		"rdePolicy:policy":   {"scope", "element", "type"},

		"secDNS:dsData":  {},
		"secDNS:keyData": {},
	}
}
