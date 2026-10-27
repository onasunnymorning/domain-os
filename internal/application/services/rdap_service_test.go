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
	t *entities.TLD
}

func (f rdapTLDs) GetWithPhases(context.Context, string) (*entities.TLD, error) { return f.t, nil }

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

type rdapHosts struct {
	repositories.HostRepository
	hosts []*entities.Host
}

func (f rdapHosts) ListHostsByName(context.Context, string) ([]*entities.Host, error) {
	return f.hosts, nil
}

// rdapDomainsByName holds registered domains by name.
type rdapDomainsByName struct {
	repositories.DomainRepository
	byName map[string]*entities.Domain
}

func (f rdapDomainsByName) GetDomainByName(_ context.Context, name string, _ bool) (*entities.Domain, error) {
	if d, ok := f.byName[name]; ok {
		return d, nil
	}
	return nil, entities.ErrDomainNotFound
}

func TestRDAPNameserverResolution(t *testing.T) {
	older := &entities.Host{Name: "ns1.a.example", ClID: "other", CreatedAt: time.Unix(1, 0)}
	sponsors := &entities.Host{Name: "ns1.a.example", ClID: "owner", CreatedAt: time.Unix(2, 0)}

	for _, tc := range []struct {
		name      string
		hosts     []*entities.Host
		domains   map[string]*entities.Domain
		want      *entities.Host
		ambiguous bool
	}{
		{"one host", []*entities.Host{sponsors}, nil, sponsors, false},
		{"several: the superordinate domain's sponsor wins over the oldest", []*entities.Host{older, sponsors},
			map[string]*entities.Domain{"a.example": {Name: "a.example", ClID: "owner"}}, sponsors, true},
		{"several, external: the oldest", []*entities.Host{older, sponsors}, nil, older, true},
		{"several, sponsor holds none: the oldest", []*entities.Host{older, sponsors},
			map[string]*entities.Domain{"a.example": {Name: "a.example", ClID: "nobody"}}, older, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewRDAPService(RDAPServiceDeps{
				Hosts: rdapHosts{hosts: tc.hosts}, Domains: rdapDomainsByName{byName: tc.domains}, Registrars: rdapRegistrars{},
			})
			rec, err := svc.Nameserver(context.Background(), "ns1.a.example")
			require.NoError(t, err)
			assert.Same(t, tc.want, rec.Host)
			assert.Equal(t, tc.ambiguous, rec.Ambiguous)
		})
	}

	t.Run("no host is not found", func(t *testing.T) {
		svc := NewRDAPService(RDAPServiceDeps{Hosts: rdapHosts{}, Domains: rdapDomainsByName{}, Registrars: rdapRegistrars{}})
		_, err := svc.Nameserver(context.Background(), "ns1.a.example")
		assert.ErrorIs(t, err, entities.ErrHostNotFound)
	})
}
