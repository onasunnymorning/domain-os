package services

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// racingContacts runs race once, just before the first delete is written:
// a command that changes the contact between Delete's checks and its
// write.
type racingContacts struct {
	repositories.ContactRepository
	once sync.Once
	race func()
}

func (r *racingContacts) DeleteContactIfUnchanged(ctx context.Context, c *entities.Contact) error {
	r.once.Do(r.race)
	return r.ContactRepository.DeleteContactIfUnchanged(ctx, c)
}

func newRacingContacts(t *testing.T) (*memstore.Registry, *racingContacts, *EPPContactService, *EPPContactService) {
	t.Helper()
	reg := memstore.NewRegistry("TEST")
	reg.AddContact("shared-1", "registrar1")
	racing := &racingContacts{ContactRepository: reg.Contacts()}
	svc := NewEPPContactService(EPPContactServiceDeps{Contacts: racing, Roids: NewRoidService(reg, "TEST")})
	other := NewEPPContactService(EPPContactServiceDeps{Contacts: reg.Contacts(), Roids: NewRoidService(reg, "TEST")})
	return reg, racing, svc, other
}

func TestEPPContactDelete_DoesNotDeleteAReplacement(t *testing.T) {
	ctx := context.Background()
	reg, racing, svc, other := newRacingContacts(t)
	name, city := "Someone Else", "Paris"
	racing.race = func() {
		// registrar1's own delete of the contact, then registrar2 takes the
		// ID.
		require.NoError(t, other.Delete(ctx, "registrar1", "shared-1"))
		_, err := other.Create(ctx, "registrar2", commands.EPPCreateContactCommand{
			ID:         "shared-1",
			PostalInfo: []commands.EPPPostalInfo{{Type: "int", Name: &name, Addr: &commands.EPPContactAddress{City: city, CC: "FR"}}},
			Email:      "else@example.com", AuthInfo: "Contact-Auth-1!",
		})
		require.NoError(t, err)
	}

	err := svc.Delete(ctx, "registrar1", "shared-1")
	require.ErrorIs(t, err, entities.ErrEPPNotSponsor, "checked again, the ID is another registrar's")
	c := reg.Contact("shared-1")
	require.NotNil(t, c, "the replacement is not deleted")
	assert.Equal(t, entities.ClIDType("registrar2"), c.ClID)
}

func TestEPPContactDelete_HonoursAConcurrentDeleteProhibition(t *testing.T) {
	ctx := context.Background()
	reg, racing, svc, other := newRacingContacts(t)
	racing.race = func() {
		_, err := other.Update(ctx, "registrar1", commands.EPPUpdateContactCommand{ID: "shared-1", AddStatuses: []string{"clientDeleteProhibited"}})
		require.NoError(t, err)
	}

	err := svc.Delete(ctx, "registrar1", "shared-1")
	require.ErrorIs(t, err, entities.ErrEPPDeleteProhibited)
	assert.NotNil(t, reg.Contact("shared-1"))
}

// racingHostDeletes is racingHosts for deletes.
type racingHostDeletes struct {
	repositories.HostRepository
	once sync.Once
	race func()
}

func (r *racingHostDeletes) DeleteHostIfUnchanged(ctx context.Context, h *entities.Host) error {
	r.once.Do(r.race)
	return r.HostRepository.DeleteHostIfUnchanged(ctx, h)
}

func TestEPPHostDelete_HonoursAConcurrentDeleteProhibition(t *testing.T) {
	ctx := context.Background()
	reg := memstore.NewRegistry("TEST")
	reg.AddHost("ns1.example.net", "registrar1")
	hosts := &racingHostDeletes{HostRepository: reg.Hosts()}
	svc := NewEPPHostService(EPPHostServiceDeps{Hosts: hosts, Domains: reg.Domains(), TLDs: reg.TLDs()})
	other := NewEPPHostService(EPPHostServiceDeps{Hosts: reg.Hosts(), Domains: reg.Domains(), TLDs: reg.TLDs()})
	hosts.race = func() {
		_, err := other.Update(ctx, "registrar1", commands.EPPUpdateHostCommand{Name: "ns1.example.net", AddStatuses: []string{"clientDeleteProhibited"}})
		require.NoError(t, err)
	}

	err := svc.Delete(ctx, "registrar1", "ns1.example.net")
	require.ErrorIs(t, err, entities.ErrEPPDeleteProhibited)
	assert.NotNil(t, reg.Host("ns1.example.net", "registrar1"))
}
