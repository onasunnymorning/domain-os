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

// racingHosts runs race once, just before the first update is written: a
// command that changes the host between Update's reading of it and its
// write.
type racingHosts struct {
	repositories.HostRepository
	once sync.Once
	race func()
}

func (r *racingHosts) UpdateHostAndAddresses(ctx context.Context, h *entities.Host) (*entities.Host, error) {
	r.once.Do(r.race)
	return r.HostRepository.UpdateHostAndAddresses(ctx, h)
}

func TestEPPHostUpdate_DoesNotOverwriteAConcurrentChange(t *testing.T) {
	ctx := context.Background()
	reg := memstore.NewRegistry("TEST")
	reg.AddTLD("test", true)
	reg.AddDomain(&entities.Domain{Name: "mine.test", ClID: "registrar1", RoID: "1_DOM-TEST"})
	reg.AddHost("ns1.mine.test", "registrar1", "208.77.190.196")

	hosts := &racingHosts{HostRepository: reg.Hosts()}
	svc := NewEPPHostService(EPPHostServiceDeps{Hosts: hosts, Domains: reg.Domains(), TLDs: reg.TLDs(), Roids: NewRoidService(reg, "TEST")})
	other := NewEPPHostService(EPPHostServiceDeps{Hosts: reg.Hosts(), Domains: reg.Domains(), TLDs: reg.TLDs(), Roids: NewRoidService(reg, "TEST")})
	scope := entities.RegistrarClID("registrar1")
	add := func(addr string) commands.EPPUpdateHostCommand {
		return commands.EPPUpdateHostCommand{Name: "ns1.mine.test", AddAddresses: []commands.EPPHostAddress{{Version: "v4", Address: addr}}}
	}
	hosts.race = func() {
		_, err := other.Update(ctx, scope, add("208.77.190.197"))
		require.NoError(t, err)
	}

	_, err := svc.Update(ctx, scope, add("208.77.190.198"))
	require.NoError(t, err)
	var got []string
	for _, a := range reg.Host("ns1.mine.test", "registrar1").Addresses {
		got = append(got, a.String())
	}
	assert.ElementsMatch(t, []string{"208.77.190.196", "208.77.190.197", "208.77.190.198"}, got,
		"the update was read and checked again after the other command's change, and kept it")
}

func TestEPPHostUpdate_ReportsAHostThatKeepsChanging(t *testing.T) {
	ctx := context.Background()
	reg := memstore.NewRegistry("TEST")
	reg.AddTLD("test", true)
	reg.AddDomain(&entities.Domain{Name: "mine.test", ClID: "registrar1", RoID: "1_DOM-TEST"})
	reg.AddHost("ns1.mine.test", "registrar1", "208.77.190.196")
	svc := NewEPPHostService(EPPHostServiceDeps{Hosts: alwaysChanged{reg.Hosts()}, Domains: reg.Domains(), TLDs: reg.TLDs()})

	_, err := svc.Update(ctx, "registrar1", commands.EPPUpdateHostCommand{
		Name: "ns1.mine.test", AddStatuses: []string{entities.HostStatusClientDeleteProhibited},
	})
	require.ErrorIs(t, err, entities.ErrHostChanged)
}

// alwaysChanged refuses every write as stale.
type alwaysChanged struct{ repositories.HostRepository }

func (alwaysChanged) UpdateHostAndAddresses(context.Context, *entities.Host) (*entities.Host, error) {
	return nil, entities.ErrHostChanged
}
