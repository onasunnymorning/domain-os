// Package rdappolicy decides what an RDAP response may contain (ADR 0021).
// Decide is a pure function of who is asking, the TLD's current contact
// policy, its publication policy and the contact's disclose flags, so every
// rule is tested without a server. The RDAP response builders carry its
// decisions out; they never decide.
package rdappolicy

import (
	"strings"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Role is a contact's role on a domain, named as RDAP names entity roles
// (RFC 9083 §10.2.4).
type Role string

const (
	RoleRegistrant Role = "registrant"
	RoleAdmin      Role = "administrative"
	RoleTech       Role = "technical"
	RoleBilling    Role = "billing"
)

// Roles is every contact role, in the order a response lists them.
var Roles = []Role{RoleRegistrant, RoleAdmin, RoleTech, RoleBilling}

// Field is one published element of a contact.
type Field string

const (
	FieldHandle  Field = "handle"
	FieldName    Field = "name"
	FieldOrg     Field = "org"
	FieldStreet  Field = "street"
	FieldCity    Field = "city"
	FieldSP      Field = "sp"
	FieldPC      Field = "pc"
	FieldCC      Field = "cc"
	FieldVoice   Field = "voice"
	FieldFax     Field = "fax"
	FieldEmail   Field = "email"
	numberFields       = 11
)

// Fields is every contact field, in the order a response lists them.
var Fields = []Field{FieldHandle, FieldName, FieldOrg, FieldStreet, FieldCity, FieldSP, FieldPC, FieldCC, FieldVoice, FieldFax, FieldEmail}

// Rule is what a publication policy says about one field.
type Rule string

const (
	// Publish publishes the field.
	Publish Rule = "publish"
	// PublishIfDisclosed publishes the field only if the contact's RFC 5733
	// disclose flag for it is set.
	PublishIfDisclosed Rule = "publish-if-disclosed"
	// Withhold never publishes the field, and lists it as redacted.
	Withhold Rule = "withhold"
)

// Method is how a withheld field is redacted (RFC 9537 §3).
type Method string

const (
	// Removal leaves the field out of the response.
	Removal Method = "removal"
	// EmptyValue keeps the field with an empty value.
	EmptyValue Method = "emptyValue"
	// ReplacementValue shows another value instead: for an email, the
	// publication policy's contact form URI.
	ReplacementValue Method = "replacementValue"
)

// FieldRule is a field's rule and, when withheld, how.
type FieldRule struct {
	Rule   Rule
	Method Method
}

// Policy is a publication policy: the field-access matrix of
// docs/rdap/PUBLICATION_REFERENCE.md. Phase 3 stores policies and assigns
// them to TLDs; until then each TLD has the built-in for its type.
type Policy struct {
	Name string
	// Roles are the roles this policy publishes at all. A role it does
	// not list is treated as not collected: no entity, nothing redacted.
	Roles map[Role]map[Field]FieldRule
	// ContactFormURI replaces a withheld email where the rule's method is
	// ReplacementValue. Empty falls back to Removal.
	ContactFormURI string
}

func withhold(m Method) FieldRule { return FieldRule{Rule: Withhold, Method: m} }

var (
	publish            = FieldRule{Rule: Publish}
	publishIfDisclosed = FieldRule{Rule: PublishIfDisclosed}
)

// GTLDRegistrationDataPolicy2024 is the built-in gtld-rdp-2024: ICANN's
// Registration Data Policy baseline. It awaits the policy owner's sign-off
// (docs/rdap/PUBLICATION_REFERENCE.md).
func GTLDRegistrationDataPolicy2024() Policy {
	return Policy{
		Name: "gtld-rdp-2024",
		Roles: map[Role]map[Field]FieldRule{
			RoleRegistrant: {
				FieldHandle: withhold(Removal),
				FieldName:   withhold(EmptyValue),
				FieldOrg:    publishIfDisclosed,
				FieldStreet: withhold(EmptyValue),
				FieldCity:   withhold(EmptyValue),
				FieldSP:     publish,
				FieldPC:     withhold(EmptyValue),
				FieldCC:     publish,
				FieldVoice:  withhold(EmptyValue),
				FieldFax:    withhold(Removal),
				FieldEmail:  withhold(ReplacementValue),
			},
			RoleTech: {
				FieldHandle: withhold(Removal),
				FieldName:   withhold(EmptyValue),
				FieldOrg:    withhold(Removal),
				FieldStreet: withhold(Removal),
				FieldCity:   withhold(Removal),
				FieldSP:     withhold(Removal),
				FieldPC:     withhold(Removal),
				FieldCC:     withhold(Removal),
				FieldVoice:  withhold(EmptyValue),
				FieldFax:    withhold(Removal),
				FieldEmail:  withhold(ReplacementValue),
			},
		},
	}
}

// CCTLDMinimal is the built-in cctld-minimal: only the registrant's country.
// The country is published in the jCard's address, and a jCard always has a
// name, so the name and the other address parts are kept empty
// (emptyValue), not removed.
func CCTLDMinimal() Policy {
	registrant := map[Field]FieldRule{}
	for _, f := range Fields {
		registrant[f] = withhold(Removal)
	}
	for _, f := range []Field{FieldName, FieldStreet, FieldCity, FieldSP, FieldPC} {
		registrant[f] = withhold(EmptyValue)
	}
	registrant[FieldCC] = publish
	return Policy{Name: "cctld-minimal", Roles: map[Role]map[Field]FieldRule{RoleRegistrant: registrant}}
}

// BuiltInFor is the publication policy a TLD has until Phase 3 lets one be
// assigned: gtld-rdp-2024 for a generic TLD, cctld-minimal for any other.
//
// The TLD type is inferred from the name (two letters is a ccTLD), which
// files an IDN ccTLD such as xn--p1ai as generic. When the type is uncertain
// the less revealing policy is the safe one, so an IDN TLD has cctld-minimal
// unless a policy is assigned to it: an IDN gTLD is assigned gtld-rdp-2024.
func BuiltInFor(name entities.DomainName, t entities.TLDType) Policy {
	if t == entities.TLDTypeGTLD && !strings.HasPrefix(strings.ToLower(string(name)), "xn--") {
		return GTLDRegistrationDataPolicy2024()
	}
	return CCTLDMinimal()
}
