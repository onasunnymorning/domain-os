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
			allPublic.Roles[r][f] = publish
		}
	}
	for _, role := range Roles {
		// Even a policy publishing every field, and a contact disclosing
		// every field, show nothing once the TLD prohibits the role.
		got := Decide(Anonymous(), thin(), allPublic, role, everything, "int")
		assert.False(t, got.Serve, role)
		assert.Empty(t, got.Fields, role)
	}
}

func TestNoCurrentGAPhaseServesNoContacts(t *testing.T) {
	for _, role := range Roles {
		assert.False(t, Decide(Anonymous(), nil, GTLDRegistrationDataPolicy2024(), role, entities.ContactDisclose{}, "int").Serve, role)
	}
}

func TestRolesThePolicyDoesNotPublishAreNotServed(t *testing.T) {
	p := GTLDRegistrationDataPolicy2024()
	assert.True(t, Decide(Anonymous(), thick(), p, RoleRegistrant, entities.ContactDisclose{}, "int").Serve)
	assert.True(t, Decide(Anonymous(), thick(), p, RoleTech, entities.ContactDisclose{}, "int").Serve)
	assert.False(t, Decide(Anonymous(), thick(), p, RoleAdmin, entities.ContactDisclose{}, "int").Serve, "the RDP does not publish admin contacts")
	assert.False(t, Decide(Anonymous(), thick(), p, RoleBilling, entities.ContactDisclose{}, "int").Serve)
}

func TestGTLDRegistrantFields(t *testing.T) {
	p := GTLDRegistrationDataPolicy2024()

	t.Run("nothing disclosed", func(t *testing.T) {
		d := Decide(Anonymous(), thick(), p, RoleRegistrant, entities.ContactDisclose{}, "int")
		assert.Equal(t, FieldDecision{Publish: true}, d.Fields[FieldCC])
		assert.Equal(t, FieldDecision{Publish: true}, d.Fields[FieldSP])
		assert.Equal(t, FieldDecision{Method: EmptyValue}, d.Fields[FieldName])
		assert.Equal(t, FieldDecision{Method: Removal}, d.Fields[FieldOrg], "publish-if-disclosed, not disclosed")
		assert.Equal(t, FieldDecision{Method: Removal}, d.Fields[FieldHandle])
		assert.Equal(t, FieldDecision{Method: Removal}, d.Fields[FieldEmail], "no contact form URI configured")
		assert.Len(t, d.Fields, len(Fields))
	})

	t.Run("only the shown form's disclose flag counts", func(t *testing.T) {
		locOnly := entities.ContactDisclose{OrgLoc: true, NameLoc: true, AddrLoc: true}
		shownInt := Decide(Anonymous(), thick(), p, RoleRegistrant, locOnly, "int")
		assert.False(t, shownInt.Fields[FieldOrg].Publish, "the int form is shown and its organisation is not disclosed")
		shownLoc := Decide(Anonymous(), thick(), p, RoleRegistrant, locOnly, "loc")
		assert.True(t, shownLoc.Fields[FieldOrg].Publish)
	})

	t.Run("organisation disclosed", func(t *testing.T) {
		d := Decide(Anonymous(), thick(), p, RoleRegistrant, entities.ContactDisclose{OrgLoc: true}, "loc")
		assert.True(t, d.Fields[FieldOrg].Publish)
	})

	t.Run("disclose flags cannot publish a withheld field", func(t *testing.T) {
		d := Decide(Anonymous(), thick(), p, RoleRegistrant, *entities.NewDiscloseStruct(true), "int")
		assert.False(t, d.Fields[FieldName].Publish)
		assert.False(t, d.Fields[FieldEmail].Publish)
		assert.False(t, d.Fields[FieldVoice].Publish)
	})

	t.Run("email replaced by the contact form when one is set", func(t *testing.T) {
		p := p
		p.ContactFormURI = "https://registry.example/contact"
		d := Decide(Anonymous(), thick(), p, RoleRegistrant, entities.ContactDisclose{}, "int")
		assert.Equal(t, FieldDecision{Method: ReplacementValue, Replacement: "https://registry.example/contact"}, d.Fields[FieldEmail])
	})
}

func TestCCTLDMinimalKeepsTheJCardShape(t *testing.T) {
	d := Decide(Anonymous(), thick(), CCTLDMinimal(), RoleRegistrant, entities.ContactDisclose{}, "int")
	for _, f := range []Field{FieldName, FieldStreet, FieldCity, FieldSP, FieldPC} {
		assert.Equal(t, EmptyValue, d.Fields[f].Method, "%s stays as an empty slot beside the published country", f)
	}
}

func TestCCTLDMinimalPublishesOnlyTheCountry(t *testing.T) {
	d := Decide(Anonymous(), thick(), CCTLDMinimal(), RoleRegistrant, *entities.NewDiscloseStruct(true), "int")
	for _, f := range Fields {
		assert.Equal(t, f == FieldCC, d.Fields[f].Publish, f)
	}
	assert.False(t, Decide(Anonymous(), thick(), CCTLDMinimal(), RoleTech, entities.ContactDisclose{}, "int").Serve)
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
