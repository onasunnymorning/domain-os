package rdappolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func thick() *entities.ContactDataPolicy {
	p := entities.NewContactDataPolicy() // registrant and tech mandatory, admin and billing optional
	return &p
}

func thin() *entities.ContactDataPolicy {
	return &entities.ContactDataPolicy{
		RegistrantContactDataPolicy: entities.ContactDataPolicyTypeProhibited,
		TechContactDataPolicy:       entities.ContactDataPolicyTypeProhibited,
		AdminContactDataPolicy:      entities.ContactDataPolicyTypeProhibited,
		BillingContactDataPolicy:    entities.ContactDataPolicyTypeProhibited,
	}
}

func TestThickToThinNeverServesAProhibitedRole(t *testing.T) {
	everything := *entities.NewDiscloseStruct(true)
	allPublic := Policy{Roles: map[Role]map[Field]FieldRule{}}
	for _, r := range Roles {
		allPublic.Roles[r] = map[Field]FieldRule{}
		for _, f := range Fields {
			allPublic.Roles[r][f] = FieldRule{Rule: Publish}
		}
	}
	for _, role := range Roles {
		// Even a policy publishing every field, and a contact disclosing
		// every field, show nothing once the TLD prohibits the role.
		got := Decide(Anonymous(), thin(), allPublic, role, everything)
		assert.False(t, got.Serve, role)
		assert.Empty(t, got.Fields, role)
	}
}

func TestNoCurrentGAPhaseServesNoContacts(t *testing.T) {
	for _, role := range Roles {
		assert.False(t, Decide(Anonymous(), nil, GTLDRegistrationDataPolicy2024(), role, entities.ContactDisclose{}).Serve, role)
	}
}

func TestRolesThePolicyDoesNotPublishAreNotServed(t *testing.T) {
	p := GTLDRegistrationDataPolicy2024()
	assert.True(t, Decide(Anonymous(), thick(), p, RoleRegistrant, entities.ContactDisclose{}).Serve)
	assert.True(t, Decide(Anonymous(), thick(), p, RoleTech, entities.ContactDisclose{}).Serve)
	assert.False(t, Decide(Anonymous(), thick(), p, RoleAdmin, entities.ContactDisclose{}).Serve, "the RDP does not publish admin contacts")
	assert.False(t, Decide(Anonymous(), thick(), p, RoleBilling, entities.ContactDisclose{}).Serve)
}

func TestGTLDRegistrantFields(t *testing.T) {
	p := GTLDRegistrationDataPolicy2024()

	t.Run("nothing disclosed", func(t *testing.T) {
		d := Decide(Anonymous(), thick(), p, RoleRegistrant, entities.ContactDisclose{})
		assert.Equal(t, FieldDecision{Publish: true}, d.Fields[FieldCC])
		assert.Equal(t, FieldDecision{Publish: true}, d.Fields[FieldSP])
		assert.Equal(t, FieldDecision{Method: EmptyValue}, d.Fields[FieldName])
		assert.Equal(t, FieldDecision{Method: Removal}, d.Fields[FieldOrg], "publish-if-disclosed, not disclosed")
		assert.Equal(t, FieldDecision{Method: Removal}, d.Fields[FieldHandle])
		assert.Equal(t, FieldDecision{Method: Removal}, d.Fields[FieldEmail], "no contact form URI configured")
		assert.Len(t, d.Fields, len(Fields))
	})

	t.Run("organisation disclosed", func(t *testing.T) {
		d := Decide(Anonymous(), thick(), p, RoleRegistrant, entities.ContactDisclose{OrgLoc: true})
		assert.True(t, d.Fields[FieldOrg].Publish)
	})

	t.Run("disclose flags cannot publish a withheld field", func(t *testing.T) {
		d := Decide(Anonymous(), thick(), p, RoleRegistrant, *entities.NewDiscloseStruct(true))
		assert.False(t, d.Fields[FieldName].Publish)
		assert.False(t, d.Fields[FieldEmail].Publish)
		assert.False(t, d.Fields[FieldVoice].Publish)
	})

	t.Run("email replaced by the contact form when one is set", func(t *testing.T) {
		p := p
		p.ContactFormURI = "https://registry.example/contact"
		d := Decide(Anonymous(), thick(), p, RoleRegistrant, entities.ContactDisclose{})
		assert.Equal(t, FieldDecision{Method: ReplacementValue, Replacement: "https://registry.example/contact"}, d.Fields[FieldEmail])
	})
}

func TestCCTLDMinimalPublishesOnlyTheCountry(t *testing.T) {
	d := Decide(Anonymous(), thick(), CCTLDMinimal(), RoleRegistrant, *entities.NewDiscloseStruct(true))
	for _, f := range Fields {
		assert.Equal(t, f == FieldCC, d.Fields[f].Publish, f)
	}
	assert.False(t, Decide(Anonymous(), thick(), CCTLDMinimal(), RoleTech, entities.ContactDisclose{}).Serve)
}

func TestBuiltInFor(t *testing.T) {
	assert.Equal(t, "gtld-rdp-2024", BuiltInFor(entities.TLDTypeGTLD).Name)
	assert.Equal(t, "cctld-minimal", BuiltInFor(entities.TLDTypeCCTLD).Name)
}

func TestEveryBuiltInNamesEveryField(t *testing.T) {
	for _, p := range []Policy{GTLDRegistrationDataPolicy2024(), CCTLDMinimal()} {
		for role, rules := range p.Roles {
			for _, f := range Fields {
				_, ok := rules[f]
				assert.True(t, ok, "%s %s has no rule for %s", p.Name, role, f)
			}
		}
	}
}
