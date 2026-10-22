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

// TestEPPDomainTransformsOverPostgres runs update, renew and delete through
// DomainService on the Postgres repositories, as cmd/epp wires them: the
// nameserver links and DS records replaced in the update's transaction,
// with the hosts' linked and in-bailiwick flags; a write over a domain
// changed since it was read refused; and the events.
func TestEPPDomainTransformsOverPostgres(t *testing.T) {
	f := newEPPFixture(t)
	ctx := appcontext.WithUserID(context.Background(), "epp:"+f.rar1)
	scope := entities.RegistrarClID(f.rar1)
	hosts := NewEPPHostService(EPPHostServiceDeps{Hosts: f.hostRepo, Domains: f.domainRepo, TLDs: f.tldRepo, Roids: f.roids, Events: f.events})
	name := "transforms." + f.tld
	_, err := f.domains.Create(ctx, scope, commands.EPPCreateDomainCommand{
		Name: name, Period: 2, PeriodUnit: "y", RegistrantID: "c" + f.rar1,
		Contacts: []commands.EPPDomainContact{{Type: "tech", ID: "c" + f.rar1}}, AuthInfo: "Domain-Auth-1!",
		HostNames: []string{"ns1." + f.rar1 + ".net"},
	})
	require.NoError(t, err)
	glueHost := "ns1." + name
	_, err = hosts.Create(ctx, scope, commands.EPPCreateHostCommand{Name: glueHost, Addresses: []commands.EPPHostAddress{{Version: "v4", Address: "208.77.190.196"}}})
	require.NoError(t, err)
	digest := "49FD46E6C4B45C55D4AC69CBD3CD34AC1AFE51DE3DC8B27C3F72E84BC2E1D3F4"

	t.Run("an update replaces the nameservers and DS records together", func(t *testing.T) {
		_, err := f.domains.Update(ctx, scope, commands.EPPUpdateDomainCommand{
			Name: name, AddHostNames: []string{glueHost}, RemoveHostNames: []string{"ns1." + f.rar1 + ".net"},
			AddStatuses: []string{"clientHold"},
			DNSSEC:      &commands.EPPDNSSECUpdate{AddDS: []commands.EPPDSData{{KeyTag: 2371, Algorithm: 13, DigestType: 2, Digest: digest}}},
		})
		require.NoError(t, err)
		dom, err := f.domainRepo.GetDomainByName(ctx, name, true)
		require.NoError(t, err)
		require.Len(t, dom.Hosts, 1)
		assert.Equal(t, glueHost, dom.Hosts[0].Name.String())
		require.Len(t, dom.DSRecords, 1)
		assert.True(t, dom.Status.ClientHold)
		assert.Equal(t, entities.ClIDType(f.rar1), dom.UpRr)
	})
	t.Run("the hosts' flags follow the links", func(t *testing.T) {
		glue, err := f.hostRepo.GetHostByNameAndClID(ctx, glueHost, f.rar1)
		require.NoError(t, err)
		assert.True(t, glue.Status.Linked)
		assert.True(t, glue.InBailiwick, "a nameserver of its parent domain")
		old, err := f.hostRepo.GetHostByNameAndClID(ctx, "ns1."+f.rar1+".net", f.rar1)
		require.NoError(t, err)
		assert.False(t, old.Status.Linked)
	})
	t.Run("a write over a domain changed since it was read is refused", func(t *testing.T) {
		read, err := f.domainRepo.GetDomainByName(ctx, name, true)
		require.NoError(t, err)
		_, err = f.domains.Update(ctx, scope, commands.EPPUpdateDomainCommand{Name: name, RemoveStatuses: []string{"clientHold"}})
		require.NoError(t, err)
		read.Status.ClientRenewProhibited = true
		_, err = f.domainRepo.UpdateDomainIfUnchanged(ctx, read)
		require.ErrorIs(t, err, entities.ErrDomainChanged)
		stored, err := f.domainRepo.GetDomainByName(ctx, name, false)
		require.NoError(t, err)
		assert.False(t, stored.Status.ClientHold, "the other update is kept")
		assert.False(t, stored.Status.ClientRenewProhibited)
	})
	t.Run("a renew moves the expiry date and is recorded", func(t *testing.T) {
		before, err := f.domainRepo.GetDomainByName(ctx, name, false)
		require.NoError(t, err)
		renewed, err := f.domains.Renew(ctx, scope, commands.EPPRenewDomainCommand{Name: name, CurExpDate: before.ExpiryDate.UTC().Format("2006-01-02"), Period: 1, PeriodUnit: "y"})
		require.NoError(t, err)
		assert.Equal(t, before.ExpiryDate.AddDate(1, 0, 0).UTC(), renewed.ExpiryDate.UTC())
		stored, err := f.domainRepo.GetDomainByName(ctx, name, true)
		require.NoError(t, err)
		assert.Len(t, stored.Hosts, 1, "the renew keeps the nameservers")
		assert.Len(t, stored.DSRecords, 1, "and the DS records")
	})
	t.Run("a delete is refused while a host is subordinate, then marks the domain pendingDelete", func(t *testing.T) {
		_, err := f.domains.Delete(ctx, scope, name)
		require.ErrorIs(t, err, entities.ErrEPPSubordinateHosts)
		_, err = f.domains.Update(ctx, scope, commands.EPPUpdateDomainCommand{Name: name, AddHostNames: []string{"ns1." + f.rar1 + ".net"}, RemoveHostNames: []string{glueHost}})
		require.NoError(t, err)
		require.NoError(t, hosts.Delete(ctx, scope, glueHost))
		dom, err := f.domains.Delete(ctx, scope, name)
		require.NoError(t, err)
		assert.True(t, dom.Status.PendingDelete)
	})
	t.Run("its events name the login", func(t *testing.T) {
		var types []string
		require.NoError(t, f.db.Table("domain_events").Where("subject = ? AND actor = ?", name, "epp:"+f.rar1).Pluck("type", &types).Error)
		assert.Subset(t, types, []string{"domain.registered", "domain.updated", "domain.renewed", "domain.marked_for_deletion"})
	})
}
