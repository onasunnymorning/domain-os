package services

import (
	"context"
	"strings"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// TestEPPDomainWritesOverPostgres_HoldAndRaces covers, on the Postgres
// repositories: a held domain leaving the TLD's NS, DS and glue exports and
// coming back; a lifecycle write refused over a domain renewed since it was
// read; a nameserver changed since it was checked not linked; a domain not
// written pendingDelete while a host is under it; and a host not placed
// under a domain being deleted.
func TestEPPDomainWritesOverPostgres_HoldAndRaces(t *testing.T) {
	f := newEPPFixture(t)
	ctx := context.Background()
	scope := entities.RegistrarClID(f.rar1)
	hosts := NewEPPHostService(EPPHostServiceDeps{Hosts: f.hostRepo, Domains: f.domainRepo, TLDs: f.tldRepo, Roids: f.roids})
	name := "races." + f.tld
	_, err := f.domains.Create(ctx, scope, commands.EPPCreateDomainCommand{
		Name: name, Period: 1, PeriodUnit: "y", RegistrantID: "c" + f.rar1,
		Contacts: []commands.EPPDomainContact{{Type: "tech", ID: "c" + f.rar1}}, AuthInfo: "Tr4nsf3r-C0de-Xq9#Lm2!",
		HostNames: []string{"ns1." + f.rar1 + ".net"},
	})
	require.NoError(t, err)
	glue := "ns1." + name
	_, err = hosts.Create(ctx, scope, commands.EPPCreateHostCommand{Name: glue, Addresses: []commands.EPPHostAddress{{Version: "v4", Address: "208.77.190.196"}}})
	require.NoError(t, err)
	_, err = f.domains.Update(ctx, scope, commands.EPPUpdateDomainCommand{
		Name: name, AddHostNames: []string{glue},
		DNSSEC: &commands.EPPDNSSECUpdate{AddDS: []commands.EPPDSData{{KeyTag: 2371, Algorithm: 13, DigestType: 2, Digest: "49FD46E6C4B45C55D4AC69CBD3CD34AC1AFE51DE3DC8B27C3F72E84BC2E1D3F4"}}},
	})
	require.NoError(t, err)

	mentions := func(rrs []dns.RR, err error) bool {
		require.NoError(t, err)
		for _, rr := range rrs {
			if strings.Contains(rr.String(), name+".") {
				return true
			}
		}
		return false
	}
	exports := func() (ns, ds, glue bool) {
		return mentions(f.domainRepo.GetActiveDomainsWithHosts(ctx, queries.ActiveDomainsWithHostsQuery{TldName: f.tld})),
			mentions(f.domainRepo.GetActiveDomainDS(ctx, f.tld)),
			mentions(f.domainRepo.GetActiveDomainGlue(ctx, f.tld))
	}
	t.Run("clientHold takes the domain out of the NS, DS and glue exports, and back", func(t *testing.T) {
		ns, ds, gl := exports()
		require.True(t, ns && ds && gl, "published before the hold: ns=%v ds=%v glue=%v", ns, ds, gl)
		_, err := f.domains.Update(ctx, scope, commands.EPPUpdateDomainCommand{Name: name, AddStatuses: []string{"clientHold"}})
		require.NoError(t, err)
		ns, ds, gl = exports()
		assert.False(t, ns || ds || gl, "held: ns=%v ds=%v glue=%v", ns, ds, gl)
		stored, err := f.domainRepo.GetDomainByName(ctx, name, true)
		require.NoError(t, err)
		assert.Len(t, stored.Hosts, 2, "the nameservers are kept")
		_, err = f.domains.Update(ctx, scope, commands.EPPUpdateDomainCommand{Name: name, RemoveStatuses: []string{"clientHold"}})
		require.NoError(t, err)
		ns, ds, gl = exports()
		assert.True(t, ns && ds && gl, "released: ns=%v ds=%v glue=%v", ns, ds, gl)
	})
	t.Run("a lifecycle write over a domain renewed since it was read is refused", func(t *testing.T) {
		stale, err := f.domainRepo.GetDomainByName(ctx, name, false)
		require.NoError(t, err)
		_, err = f.domains.Renew(ctx, scope, commands.EPPRenewDomainCommand{Name: name, CurExpDate: stale.ExpiryDate.UTC().Format("2006-01-02")})
		require.NoError(t, err)
		stale.Status.PendingDelete = true
		_, err = f.domainRepo.UpdateDomainColumnsIfUnchanged(ctx, stale)
		require.ErrorIs(t, err, entities.ErrDomainChanged)
		stored, err := f.domainRepo.GetDomainByName(ctx, name, false)
		require.NoError(t, err)
		assert.False(t, stored.Status.PendingDelete)
		assert.Equal(t, stale.ExpiryDate.AddDate(1, 0, 0).UTC(), stored.ExpiryDate.UTC(), "the renew is kept")
	})
	t.Run("a nameserver changed since it was checked is not linked", func(t *testing.T) {
		other := "ns2." + f.rar1 + ".net"
		_, err := hosts.Create(ctx, scope, commands.EPPCreateHostCommand{Name: other})
		require.NoError(t, err)
		dom, err := f.domainRepo.GetDomainByName(ctx, name, true)
		require.NoError(t, err)
		checked, err := f.hostRepo.GetHostByNameAndClID(ctx, other, f.rar1)
		require.NoError(t, err)
		_, err = hosts.Update(ctx, scope, commands.EPPUpdateHostCommand{Name: other, AddStatuses: []string{"clientDeleteProhibited"}})
		require.NoError(t, err)
		dom.Hosts = append(dom.Hosts, checked)
		_, err = f.domainRepo.UpdateDomainIfUnchanged(ctx, dom)
		require.ErrorIs(t, err, entities.ErrDomainChanged)
		stored, err := f.domainRepo.GetDomainByName(ctx, name, true)
		require.NoError(t, err)
		assert.Len(t, stored.Hosts, 2)
	})
	t.Run("a domain is not written pendingDelete while a host is under it", func(t *testing.T) {
		dom, err := f.domainRepo.GetDomainByName(ctx, name, true)
		require.NoError(t, err)
		dom.Status.PendingDelete, dom.Status.OK = true, false
		_, err = f.domainRepo.UpdateDomainIfUnchanged(ctx, dom)
		require.ErrorIs(t, err, entities.ErrEPPSubordinateHosts)
	})
	t.Run("a host is not placed under a domain being deleted", func(t *testing.T) {
		_, err := f.domains.Update(ctx, scope, commands.EPPUpdateDomainCommand{Name: name, RemoveHostNames: []string{glue}})
		require.NoError(t, err)
		require.NoError(t, hosts.Delete(ctx, scope, glue))
		_, err = f.domains.Delete(ctx, scope, name)
		require.NoError(t, err)
		roid, err := f.roids.GenerateRoid("host")
		require.NoError(t, err)
		h, err := entities.NewHost("ns3."+name, roid.String(), f.rar1)
		require.NoError(t, err)
		_, err = f.hostRepo.CreateHostUnder(ctx, h, name)
		require.ErrorIs(t, err, entities.ErrEPPSuperordinatePendingDelete)
	})
}
