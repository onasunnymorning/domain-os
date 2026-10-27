package rdappolicy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	assert.Equal(t, "gtld-rdp-2024", BuiltInFor("example", entities.TLDTypeGTLD).Name)
	assert.Equal(t, "cctld-minimal", BuiltInFor("cc", entities.TLDTypeCCTLD).Name)

	// NewTLD files an IDN ccTLD as generic; it must not get the gTLD policy.
	idn, err := entities.NewTLD("xn--p1ai", "ry-test")
	require.NoError(t, err)
	assert.Equal(t, entities.TLDType(entities.TLDTypeGTLD), idn.Type, "the misclassification this guards against")
	assert.Equal(t, "cctld-minimal", BuiltInFor(idn.Name, idn.Type).Name)
	assert.Equal(t, "cctld-minimal", BuiltInFor("XN--P1AI", entities.TLDTypeGTLD).Name)
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

// flagFor is, independently of Decide, which disclose flag covers a field
// in each postal form.
var flagFor = map[Field]func(d entities.ContactDisclose, intForm bool) bool{
	FieldName: func(d entities.ContactDisclose, i bool) bool { return (i && d.NameInt) || (!i && d.NameLoc) },
	FieldOrg:  func(d entities.ContactDisclose, i bool) bool { return (i && d.OrgInt) || (!i && d.OrgLoc) },
	FieldStreet: func(d entities.ContactDisclose, i bool) bool {
		return (i && d.AddrInt) || (!i && d.AddrLoc)
	},
	FieldCity:   func(d entities.ContactDisclose, i bool) bool { return (i && d.AddrInt) || (!i && d.AddrLoc) },
	FieldSP:     func(d entities.ContactDisclose, i bool) bool { return (i && d.AddrInt) || (!i && d.AddrLoc) },
	FieldPC:     func(d entities.ContactDisclose, i bool) bool { return (i && d.AddrInt) || (!i && d.AddrLoc) },
	FieldCC:     func(d entities.ContactDisclose, i bool) bool { return (i && d.AddrInt) || (!i && d.AddrLoc) },
	FieldVoice:  func(d entities.ContactDisclose, _ bool) bool { return d.Voice },
	FieldFax:    func(d entities.ContactDisclose, _ bool) bool { return d.Fax },
	FieldEmail:  func(d entities.ContactDisclose, _ bool) bool { return d.Email },
	FieldHandle: func(entities.ContactDisclose, bool) bool { return false },
}

// TestDecisionTable checks every role, collection state, rule, disclose set
// and postal form, on every field (ADR 0021 decision 3).
func TestDecisionTable(t *testing.T) {
	collections := map[string]*entities.ContactDataPolicy{"no current GA phase": nil}
	for _, typ := range []entities.ContactDataPolicyType{entities.ContactDataPolicyTypeMandatory, entities.ContactDataPolicyTypeOptional, entities.ContactDataPolicyTypeProhibited} {
		collections[string(typ)] = &entities.ContactDataPolicy{
			RegistrantContactDataPolicy: typ, AdminContactDataPolicy: typ, TechContactDataPolicy: typ, BillingContactDataPolicy: typ,
		}
	}
	rules := map[string]FieldRule{
		"publish":              {Rule: Publish},
		"publish-if-disclosed": {Rule: PublishIfDisclosed},
		"withhold emptied":     {Rule: Withhold, Method: EmptyValue},
		"withhold removed":     {Rule: Withhold, Method: Removal},
		"withhold replaced":    {Rule: Withhold, Method: ReplacementValue},
	}
	discloses := map[string]entities.ContactDisclose{
		"nothing":    {},
		"int only":   {NameInt: true, OrgInt: true, AddrInt: true},
		"loc only":   {NameLoc: true, OrgLoc: true, AddrLoc: true},
		"everything": *entities.NewDiscloseStruct(true),
	}
	for _, uri := range []string{"", "https://registry.example/contact"} {
		for cname, cdp := range collections {
			for rname, rule := range rules {
				p := Policy{Roles: map[Role]map[Field]FieldRule{}, ContactFormURI: uri}
				for _, role := range Roles {
					p.Roles[role] = map[Field]FieldRule{}
					for _, f := range Fields {
						p.Roles[role][f] = rule
					}
				}
				for dname, d := range discloses {
					for _, form := range []entities.PostalInfoEnumType{"int", "loc"} {
						for _, role := range Roles {
							got := Decide(Anonymous(), cdp, p, role, d, form)
							serve := cdp != nil && (cname == "mandatory" || cname == "optional")
							name := fmt.Sprintf("%s/%s/%s/%s/%s/uri=%t", role, cname, rname, dname, form, uri != "")
							require.Equal(t, serve, got.Serve, name)
							if !serve {
								assert.Empty(t, got.Fields, name)
								continue
							}
							for _, f := range Fields {
								fd := got.Fields[f]
								want := rule.Rule == Publish || (rule.Rule == PublishIfDisclosed && flagFor[f](d, form == "int"))
								assert.Equal(t, want, fd.Publish, "%s %s", name, f)
								if want {
									continue
								}
								switch {
								case rule.Rule == PublishIfDisclosed, rule.Method == Removal:
									assert.Equal(t, Removal, fd.Method, "%s %s", name, f)
								case rule.Method == EmptyValue:
									assert.Equal(t, EmptyValue, fd.Method, "%s %s", name, f)
								case f == FieldEmail && uri != "":
									assert.Equal(t, FieldDecision{Method: ReplacementValue, Replacement: uri}, fd, "%s %s", name, f)
								default: // only an email can be replaced, and only with a URI
									assert.Equal(t, Removal, fd.Method, "%s %s", name, f)
								}
							}
						}
					}
				}
			}
		}
	}
}
