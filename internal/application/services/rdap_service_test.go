package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/rdappolicy"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

type rdapDomains struct {
	repositories.DomainRepository
	d *entities.Domain
}

func (f rdapDomains) GetDomainByName(context.Context, string, bool) (*entities.Domain, error) {
	return f.d, nil
}

type rdapRegistrars struct {
	repositories.RegistrarRepository
}

func (rdapRegistrars) GetByClID(context.Context, string, bool) (*entities.Registrar, error) {
	return nil, entities.ErrRegistrarNotFound
}

type rdapTLDs struct {
	repositories.TLDRepository
	t *entities.TLD
}

func (f rdapTLDs) GetByName(context.Context, string, bool) (*entities.TLD, error) { return f.t, nil }

// rdapContacts records every contact read.
type rdapContacts struct {
	repositories.ContactRepository
	read *[]string
}

func (f rdapContacts) GetContactByID(_ context.Context, id string) (*entities.Contact, error) {
	*f.read = append(*f.read, id)
	return &entities.Contact{ID: entities.ClIDType(id)}, nil
}

func rdapTLD(typ entities.TLDType, cdp entities.ContactDataPolicy) *entities.TLD {
	policy := entities.NewPhasePolicy()
	policy.ContactDataPolicy = cdp
	return &entities.TLD{Name: "example", Type: typ, Phases: []entities.Phase{{
		Name: "GA", Type: entities.PhaseTypeGA, Starts: time.Now().Add(-time.Hour), Policy: policy,
	}}}
}

func TestRDAPServiceReadsOnlyContactsItMayServe(t *testing.T) {
	domain := &entities.Domain{Name: "a.example", TLDName: "example", ClID: "rar", RegistrantID: "reg", AdminID: "adm", TechID: "tec", BillingID: "bil"}

	for _, tc := range []struct {
		name string
		tld  *entities.TLD
		want []string
	}{
		{"gTLD thick: the RDP publishes registrant and tech only", rdapTLD(entities.TLDTypeGTLD, entities.NewContactDataPolicy()), []string{"reg", "tec"}},
		{"gTLD after going thin: nothing", rdapTLD(entities.TLDTypeGTLD, entities.ContactDataPolicy{
			RegistrantContactDataPolicy: entities.ContactDataPolicyTypeProhibited, TechContactDataPolicy: entities.ContactDataPolicyTypeProhibited,
			AdminContactDataPolicy: entities.ContactDataPolicyTypeProhibited, BillingContactDataPolicy: entities.ContactDataPolicyTypeProhibited,
		}), nil},
		{"ccTLD minimal: registrant only", rdapTLD(entities.TLDTypeCCTLD, entities.NewContactDataPolicy()), []string{"reg"}},
		{"no current GA phase: nothing", &entities.TLD{Name: "example", Type: entities.TLDTypeGTLD}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var read []string
			svc := NewRDAPService(RDAPServiceDeps{
				Domains: rdapDomains{d: domain}, Registrars: rdapRegistrars{}, TLDs: rdapTLDs{t: tc.tld}, Contacts: rdapContacts{read: &read},
			})
			rec, err := svc.Domain(context.Background(), "a.example")
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.want, read, "contacts in other roles are never read")
			assert.Len(t, rec.Contacts, len(tc.want))
			assert.Nil(t, rec.Registrar, "a missing sponsor is left out, not an error")
		})
	}
}

func TestRDAPServicePublicationFollowsTLDType(t *testing.T) {
	var read []string
	svc := NewRDAPService(RDAPServiceDeps{
		Domains: rdapDomains{d: &entities.Domain{Name: "a.example", TLDName: "example"}}, Registrars: rdapRegistrars{},
		TLDs: rdapTLDs{t: rdapTLD(entities.TLDTypeCCTLD, entities.NewContactDataPolicy())}, Contacts: rdapContacts{read: &read},
	})
	rec, err := svc.Domain(context.Background(), "a.example")
	require.NoError(t, err)
	assert.Equal(t, rdappolicy.CCTLDMinimal().Name, rec.Publication.Name)
}
