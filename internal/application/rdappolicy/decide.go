package rdappolicy

import (
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Tier is what a requester may see. Only Public exists until the
// authenticated tier (ADR 0021 decision 6).
type Tier string

// Public is every anonymous requester.
const Public Tier = "public"

// Requester is who is asking. Today the server builds only the anonymous
// one; an authenticated tier will add its grants here, and Decide will
// weigh them, without the response builders changing.
type Requester struct {
	Tier Tier
}

// Anonymous is the requester every RDAP request is today.
func Anonymous() Requester { return Requester{Tier: Public} }

// FieldDecision is what a response does with one field.
type FieldDecision struct {
	// Publish is true when the field's value is shown.
	Publish bool
	// Method is how the field is redacted when it is not published.
	Method Method
	// Replacement is the value shown instead, for ReplacementValue.
	Replacement string
}

// ContactDecision is what a response does with the contact in one role.
type ContactDecision struct {
	// Serve is false when the role is not shown at all: the TLD's current
	// policy prohibits it, or the publication policy does not publish it.
	// Nothing about an unserved role is listed as redacted.
	Serve  bool
	Fields map[Field]FieldDecision
}

// RoleAllowed reports whether the TLD's current contact policy collects the
// role: mandatory or optional. cdp is nil when the TLD has no current GA
// phase, and then no role is allowed (ADR 0021 decision 1).
func RoleAllowed(cdp *entities.ContactDataPolicy, role Role) bool {
	if cdp == nil {
		return false
	}
	var t entities.ContactDataPolicyType
	switch role {
	case RoleRegistrant:
		t = cdp.RegistrantContactDataPolicy
	case RoleAdmin:
		t = cdp.AdminContactDataPolicy
	case RoleTech:
		t = cdp.TechContactDataPolicy
	case RoleBilling:
		t = cdp.BillingContactDataPolicy
	}
	return t == entities.ContactDataPolicyTypeMandatory || t == entities.ContactDataPolicyTypeOptional
}

// Decide is what req may see of the contact in role, under the TLD's
// current contact policy cdp and its publication policy p. form is the
// postal form the response shows ("int" or "loc"): its disclose flags are
// the ones that count for the name, organisation and address.
//
// A role cdp prohibits is never served, whatever is stored: a TLD that moved
// from thick to thin stops showing the contacts still linked from before.
// Within a served role, disclose flags only narrow what p publishes: a
// publish-if-disclosed field is shown only with its flag set, and a withheld
// field is never shown, whatever the flags say.
func Decide(req Requester, cdp *entities.ContactDataPolicy, p Policy, role Role, disclose entities.ContactDisclose, form entities.PostalInfoEnumType) ContactDecision {
	_ = req // every requester is public until the authenticated tier
	rules, published := p.Roles[role]
	if !RoleAllowed(cdp, role) || !published {
		return ContactDecision{}
	}
	out := ContactDecision{Serve: true, Fields: make(map[Field]FieldDecision, numberFields)}
	for _, f := range Fields {
		rule, ok := rules[f]
		if !ok {
			rule = withhold(Removal) // a field the policy does not name is withheld
		}
		switch {
		case rule.Rule == Publish,
			rule.Rule == PublishIfDisclosed && disclosed(disclose, f, form):
			out.Fields[f] = FieldDecision{Publish: true}
		default:
			out.Fields[f] = withheldAs(rule, p, f)
		}
	}
	return out
}

// withheldAs is how a withheld field is redacted. A publish-if-disclosed
// field that is not disclosed is removed.
func withheldAs(rule FieldRule, p Policy, f Field) FieldDecision {
	m := rule.Method
	if rule.Rule != Withhold || m == "" {
		m = Removal
	}
	if m == ReplacementValue {
		if f != FieldEmail || p.ContactFormURI == "" {
			return FieldDecision{Method: Removal}
		}
		return FieldDecision{Method: ReplacementValue, Replacement: p.ContactFormURI}
	}
	return FieldDecision{Method: m}
}

// disclosed reads the RFC 5733 flag that covers f. A contact can hold an
// internationalised and a localised form with different disclose flags, so
// the name, organisation and address are disclosed only if the flag of the
// form shown is set.
func disclosed(d entities.ContactDisclose, f Field, form entities.PostalInfoEnumType) bool {
	intForm := form == entities.PostalInfoEnumType("int")
	switch f {
	case FieldName:
		return (intForm && d.NameInt) || (!intForm && d.NameLoc)
	case FieldOrg:
		return (intForm && d.OrgInt) || (!intForm && d.OrgLoc)
	case FieldStreet, FieldCity, FieldSP, FieldPC, FieldCC:
		return (intForm && d.AddrInt) || (!intForm && d.AddrLoc)
	case FieldVoice:
		return d.Voice
	case FieldFax:
		return d.Fax
	case FieldEmail:
		return d.Email
	default:
		return false
	}
}
