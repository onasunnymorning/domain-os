package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestEPPHostCommandsOverPostgres runs the host commands on the Postgres
// repositories, as cmd/epp wires them: the host and its addresses written
// in one insert, an update replacing the addresses in one transaction, a
// rename, and the link counts behind linked, delete and rename.
func TestEPPHostCommandsOverPostgres(t *testing.T) {
	f := newEPPFixture(t)
	ctx := appcontext.WithUserID(context.Background(), "epp:"+f.rar1)
	svc := NewEPPHostService(EPPHostServiceDeps{
		Hosts: f.hostRepo, Domains: f.domainRepo, TLDs: f.tldRepo, Roids: f.roids, Events: f.events,
	})
	scope1, scope2 := entities.RegistrarClID(f.rar1), entities.RegistrarClID(f.rar2)
	addrs := func(h *entities.Host) []string {
		out := make([]string, len(h.Addresses))
		for i, a := range h.Addresses {
			out[i] = a.String()
		}
		return out
	}
	parent := "hosts." + f.tld
	_, err := f.domains.Create(ctx, scope1, commands.EPPCreateDomainCommand{
		Name: parent, Period: 1, PeriodUnit: "y", RegistrantID: "c" + f.rar1,
		Contacts: []commands.EPPDomainContact{{Type: "tech", ID: "c" + f.rar1}}, AuthInfo: "Domain-Auth-1!",
	})
	require.NoError(t, err)

	internal := "ns1." + parent
	host, err := svc.Create(ctx, scope1, commands.EPPCreateHostCommand{Name: internal, Addresses: []commands.EPPHostAddress{
		{Version: "v4", Address: "208.77.190.196"}, {Version: "v6", Address: "2602:800:900e:1257::5"},
	}})
	require.NoError(t, err)
	t.Run("created with its addresses and a minted ROID", func(t *testing.T) {
		assert.Regexp(t, `^\d+_HOST-EPPTEST$`, host.RoID.String())
		assert.Equal(t, entities.ClIDType(f.rar1), host.ClID)
		assert.ElementsMatch(t, []string{"208.77.190.196", "2602:800:900e:1257::5"}, addrs(host))
		assert.False(t, host.CreatedAt.IsZero())
	})
	t.Run("a second create is refused as existing", func(t *testing.T) {
		_, err := svc.Create(ctx, scope1, commands.EPPCreateHostCommand{Name: internal})
		require.ErrorIs(t, err, entities.ErrHostAlreadyExists)
	})
	t.Run("another registrar is refused it", func(t *testing.T) {
		_, err := svc.Info(ctx, scope2, internal)
		require.ErrorIs(t, err, entities.ErrEPPNotSponsor)
		_, err = svc.Create(ctx, scope2, commands.EPPCreateHostCommand{Name: "ns2." + parent})
		require.ErrorIs(t, err, entities.ErrEPPNotSponsor, "the superordinate domain is not theirs")
	})
	t.Run("an update replaces the addresses and statuses", func(t *testing.T) {
		updated, err := svc.Update(ctx, scope1, commands.EPPUpdateHostCommand{
			Name:            internal,
			AddAddresses:    []commands.EPPHostAddress{{Version: "v4", Address: "208.77.190.197"}},
			RemoveAddresses: []commands.EPPHostAddress{{Version: "v6", Address: "2602:800:900e:1257::5"}},
			AddStatuses:     []string{entities.HostStatusClientDeleteProhibited},
		})
		require.NoError(t, err)
		id, _ := updated.RoID.Int64()
		stored, err := f.hostRepo.GetHostByRoid(ctx, id)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"208.77.190.196", "208.77.190.197"}, addrs(stored))
		assert.True(t, stored.Status.ClientDeleteProhibited)
		assert.False(t, stored.Status.OK)
		assert.Equal(t, entities.ClIDType(f.rar1), stored.UpRr)
	})

	linked := "linked." + f.tld
	_, err = f.domains.Create(ctx, scope1, commands.EPPCreateDomainCommand{
		Name: linked, Period: 1, PeriodUnit: "y", RegistrantID: "c" + f.rar1,
		Contacts:  []commands.EPPDomainContact{{Type: "tech", ID: "c" + f.rar1}}, AuthInfo: "Domain-Auth-1!",
		HostNames: []string{internal},
	})
	require.NoError(t, err)
	t.Run("a nameserver is linked, and is not deleted", func(t *testing.T) {
		info, err := svc.Info(ctx, scope1, internal)
		require.NoError(t, err)
		assert.True(t, info.Status.Linked)
		_, err = svc.Update(ctx, scope1, commands.EPPUpdateHostCommand{Name: internal, RemoveStatuses: []string{entities.HostStatusClientDeleteProhibited}})
		require.NoError(t, err)
		require.ErrorIs(t, svc.Delete(ctx, scope1, internal), entities.ErrEPPHostLinked)
	})
	t.Run("a linked internal host keeps an address", func(t *testing.T) {
		_, err := svc.Update(ctx, scope1, commands.EPPUpdateHostCommand{Name: internal, RemoveAddresses: []commands.EPPHostAddress{
			{Version: "v4", Address: "208.77.190.196"}, {Version: "v4", Address: "208.77.190.197"},
		}})
		require.ErrorIs(t, err, entities.ErrEPPHostNeedsAddress)
	})
	renamed := "ns9." + parent
	t.Run("a rename renames the nameserver on its domains", func(t *testing.T) {
		_, err := svc.Update(ctx, scope1, commands.EPPUpdateHostCommand{Name: internal, NewName: renamed})
		require.NoError(t, err)
		dom, err := f.domainRepo.GetDomainByName(ctx, linked, true)
		require.NoError(t, err)
		require.Len(t, dom.Hosts, 1)
		assert.Equal(t, renamed, dom.Hosts[0].Name.String())
	})
	t.Run("a rename onto another of the registrar's hosts is refused", func(t *testing.T) {
		_, err := svc.Update(ctx, scope1, commands.EPPUpdateHostCommand{Name: renamed, NewName: "ns1." + f.rar1 + ".net"})
		require.ErrorIs(t, err, entities.ErrHostAlreadyExists)
	})
	t.Run("a rename out of the registry's TLDs drops the addresses", func(t *testing.T) {
		updated, err := svc.Update(ctx, scope1, commands.EPPUpdateHostCommand{Name: renamed, NewName: "ns9.example.net"})
		require.NoError(t, err)
		id, _ := updated.RoID.Int64()
		stored, err := f.hostRepo.GetHostByRoid(ctx, id)
		require.NoError(t, err)
		assert.Empty(t, stored.Addresses)
	})
	t.Run("a nameserver of another registrar's domain is not renamed", func(t *testing.T) {
		// As an escrow import from a registry with shared hosts can leave it.
		require.NoError(t, f.db.Exec("UPDATE domains SET cl_id = ? WHERE name = ?", f.rar2, linked).Error)
		_, err := svc.Update(ctx, scope1, commands.EPPUpdateHostCommand{Name: "ns9.example.net", NewName: "ns8.example.net"})
		require.ErrorIs(t, err, entities.ErrEPPHostLinkedByOthers)
	})
	t.Run("an unlinked host is deleted", func(t *testing.T) {
		external := "ns7.example.net"
		created, err := svc.Create(ctx, scope1, commands.EPPCreateHostCommand{Name: external})
		require.NoError(t, err)
		require.NoError(t, svc.Delete(ctx, scope1, external))
		id, _ := created.RoID.Int64()
		_, err = f.hostRepo.GetHostByRoid(ctx, id)
		require.ErrorIs(t, err, entities.ErrHostNotFound)
	})
	t.Run("its events name the session's login as actor", func(t *testing.T) {
		var types []string
		require.NoError(t, f.db.Table("domain_events").Where("subject = ? AND actor = ?", host.RoID.String(), "epp:"+f.rar1).
			Order("occurred_at").Pluck("type", &types).Error)
		assert.Contains(t, types, "host.created")
		assert.Contains(t, types, "host.updated")
	})
}
