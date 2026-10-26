// Package rdappolicy decides what an RDAP response may contain (ADR 0021).
// Decide is a pure function of who is asking, the TLD's current contact
// policy, its publication policy and the contact's disclose flags, so every
// rule is tested without a server. The RDAP response builders carry its
// decisions out; they never decide.
//
// The vocabulary (roles, fields, rules, methods) and the publication policy
// itself are domain types (entities.RDAPPublicationPolicy); this package
// names them for the decision.
package rdappolicy

import (
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type (
	// Role is a contact's role on a domain.
	Role = entities.RDAPRole
	// Field is one published element of a contact.
	Field = entities.RDAPField
	// Rule is what a publication policy says about one field.
	Rule = entities.RDAPRule
	// Method is how a withheld field is redacted (RFC 9537 §3).
	Method = entities.RDAPRedactionMethod
	// FieldRule is a field's rule and, when withheld, how.
	FieldRule = entities.RDAPFieldRule
	// Policy is a publication policy.
	Policy = entities.RDAPPublicationPolicy
)

const (
	RoleRegistrant = entities.RDAPRoleRegistrant
	RoleAdmin      = entities.RDAPRoleAdmin
	RoleTech       = entities.RDAPRoleTech
	RoleBilling    = entities.RDAPRoleBilling

	FieldHandle = entities.RDAPFieldHandle
	FieldName   = entities.RDAPFieldName
	FieldOrg    = entities.RDAPFieldOrg
	FieldStreet = entities.RDAPFieldStreet
	FieldCity   = entities.RDAPFieldCity
	FieldSP     = entities.RDAPFieldSP
	FieldPC     = entities.RDAPFieldPC
	FieldCC     = entities.RDAPFieldCC
	FieldVoice  = entities.RDAPFieldVoice
	FieldFax    = entities.RDAPFieldFax
	FieldEmail  = entities.RDAPFieldEmail

	Publish            = entities.RDAPPublish
	PublishIfDisclosed = entities.RDAPPublishIfDisclosed
	Withhold           = entities.RDAPWithhold

	Removal          = entities.RDAPRemoval
	EmptyValue       = entities.RDAPEmptyValue
	ReplacementValue = entities.RDAPReplacementValue
)

var (
	// Roles is every contact role, in the order a response lists them.
	Roles = entities.RDAPRoles
	// Fields is every contact field, in the order a response lists them.
	Fields = entities.RDAPFields
)

func builtIn(name string) Policy {
	for _, p := range entities.BuiltInRDAPPublicationPolicies() {
		if p.Name == name {
			return *p
		}
	}
	panic("rdappolicy: no built-in policy " + name)
}

// GTLDRegistrationDataPolicy2024 is the built-in gtld-rdp-2024.
func GTLDRegistrationDataPolicy2024() Policy { return builtIn(entities.RDAPPolicyGTLDRDP2024) }

// CCTLDMinimal is the built-in cctld-minimal.
func CCTLDMinimal() Policy { return builtIn(entities.RDAPPolicyCCTLDMinimal) }

// BuiltInFor is the built-in a TLD of type t has when assigned none.
func BuiltInFor(t entities.TLDType) Policy {
	return builtIn(entities.BuiltInRDAPPublicationPolicyFor(t))
}
