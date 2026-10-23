package services

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestEPPDomainTransferOverPostgres runs a transfer through DomainService
// and the Postgres repositories, as cmd/epp wires them: the request, the
// poll queue, and the approval's one transaction, which moves the domain,
// re-links its nameservers and queues the gaining registrar's message.
func TestEPPDomainTransferOverPostgres(t *testing.T) {
	f := newEPPFixture(t)
	ctx, db, tld, rar1, rar2 := context.Background(), f.db, f.tld, f.rar1, f.rar2
	scope1, scope2 := entities.RegistrarClID(rar1), entities.RegistrarClID(rar2)
	transfers := postgres.NewDomainTransferRepository(db, "EPPTEST")
	f.registry.SetTransferRepo(transfers)
	svc := NewEPPDomainService(EPPDomainServiceDeps{
		Domains: f.domainRepo, NNDNs: postgres.NewGormNNDNRepository(db), TLDs: f.tldRepo, Phases: postgres.NewGormPhaseRepository(db),
		Contacts: postgres.NewContactRepository(db, "EPPTEST"), Hosts: f.hostRepo, Registrars: postgres.NewGormRegistrarRepository(db),
		Transfers: transfers, Register: f.registry, CheckParallelism: 2,
	})
	hosts := NewEPPHostService(EPPHostServiceDeps{Hosts: f.hostRepo, Domains: f.domainRepo, TLDs: f.tldRepo, Roids: f.roids})
	polls := NewEPPPollService(transfers)
	require.NoError(t, NewAccreditationService(postgres.NewAccreditationRepository(db), postgres.NewGormRegistrarRepository(db), f.tldRepo, f.events).
		CreateAccreditation(ctx, tld, rar2))
	// No transfer lock, as for an RST TLD: the domain is transferable at once.
	require.NoError(t, db.Exec("UPDATE phases SET transfer_lock_period = 0 WHERE tld_name = ?", tld).Error)

	const code = "Tr4nsf3r-C0de-Xq9#Lm2!" // #nosec G101 -- a test fixture's authInfo, not a credential
	name, other := "move."+tld, "other."+tld
	for _, n := range []string{name, other} {
		_, err := svc.Create(ctx, scope1, commands.EPPCreateDomainCommand{
			Name: n, RegistrantID: "c" + rar1, Contacts: []commands.EPPDomainContact{{Type: "tech", ID: "c" + rar1}},
			HostNames: []string{"ns1." + rar1 + ".net"}, AuthInfo: code,
		})
		require.NoError(t, err)
	}
	// name itself is a host under the domain too (RFC 5732: its
	// superordinate domain is the domain): it moves with it.
	for _, h := range []string{"ns1." + name, "ns." + other, name} {
		_, err := hosts.Create(ctx, scope1, commands.EPPCreateHostCommand{Name: h, Addresses: []commands.EPPHostAddress{{Address: "208.77.190.196"}}})
		require.NoError(t, err)
	}
	_, err := svc.Update(ctx, scope1, commands.EPPUpdateDomainCommand{Name: name, AddHostNames: []string{"ns1." + name, "ns." + other}})
	require.NoError(t, err)

	hostRow := func(name, clid string) *postgres.Host {
		var h postgres.Host
		if err := db.Where("name = ? AND cl_id = ?", name, clid).First(&h).Error; err != nil {
			return nil
		}
		return &h
	}

	t.Run("a request over a stale read is refused, to be read again", func(t *testing.T) {
		stale, err := f.domainRepo.GetDomainByName(ctx, name, true)
		require.NoError(t, err)
		_, err = svc.Update(ctx, scope1, commands.EPPUpdateDomainCommand{Name: name, AddStatuses: []string{entities.DomainStatusClientHold}})
		require.NoError(t, err)
		_, _, err = f.registry.RequestTransferAsRead(ctx, stale, entities.ClIDType(rar2), code, 1)
		require.ErrorIs(t, err, entities.ErrDomainChanged)
	})

	tr, err := svc.TransferRequest(ctx, scope2, commands.EPPTransferDomainCommand{Name: name, AuthInfo: ptr(code)})
	require.NoError(t, err)
	require.Equal(t, entities.TransferStatusPending, tr.Status)
	require.NotZero(t, tr.ID)

	t.Run("the request: pendingTransfer, recorded, and the losing registrar told", func(t *testing.T) {
		dom, err := f.domainRepo.GetDomainByName(ctx, name, false)
		require.NoError(t, err)
		assert.True(t, dom.Status.PendingTransfer)
		latest, err := transfers.LatestTransfer(ctx, dom.RoID)
		require.NoError(t, err)
		assert.Equal(t, tr.ID, latest.ID)
		assert.Equal(t, entities.ClIDType(rar2), latest.RequestingClID)
		// Postgres keeps microseconds; the transfer's times are taken to the
		// microsecond, so the answer and the stored transfer agree.
		assert.WithinDuration(t, tr.ActionDate, latest.ActionDate, time.Microsecond)
		assert.Equal(t, tr.ActionDate, tr.ActionDate.Truncate(time.Microsecond))
		msg, n, err := polls.Request(ctx, scope1)
		require.NoError(t, err)
		require.NotNil(t, msg)
		assert.Equal(t, int64(1), n)
		assert.Equal(t, entities.PollMsgTransferRequested, msg.Message)
		assert.Equal(t, tr.ID, msg.Transfer.ID)
		assert.Equal(t, entities.TransferStatusPending, msg.Transfer.Status)
		_, err = polls.Ack(ctx, scope2, strconv.FormatInt(msg.ID, 10))
		require.ErrorIs(t, err, entities.ErrPollMessageNotFound, "another registrar's message")
		left, err := polls.Ack(ctx, scope1, strconv.FormatInt(msg.ID, 10))
		require.NoError(t, err)
		assert.Zero(t, left)
	})

	t.Run("an approval planned over a renamed nameserver is refused whole, to be planned again", func(t *testing.T) {
		stale, err := f.domainRepo.GetDomainByName(ctx, name, true)
		require.NoError(t, err)
		pending, err := transfers.LatestTransfer(ctx, stale.RoID)
		require.NoError(t, err)
		// A rename does not change the domain's version.
		_, err = hosts.Update(ctx, scope1, commands.EPPUpdateHostCommand{Name: "ns1." + rar1 + ".net", NewName: "ns9." + rar1 + ".net"})
		require.NoError(t, err)
		// The plan is made from stale's nameservers, read before the rename.
		_, _, err = f.registry.ResolveTransferAsRead(ctx, stale, pending, entities.TransferStatusClientApproved, entities.ClIDType(rar1))
		require.ErrorIs(t, err, entities.ErrDomainChanged)
		dom, err := f.domainRepo.GetDomainByName(ctx, name, false)
		require.NoError(t, err)
		assert.Equal(t, entities.ClIDType(rar1), dom.ClID, "nothing written")
		assert.NotNil(t, hostRow("ns1."+name, rar1), "no host moved")
		_, err = hosts.Update(ctx, scope1, commands.EPPUpdateHostCommand{Name: "ns9." + rar1 + ".net", NewName: "ns1." + rar1 + ".net"})
		require.NoError(t, err)
	})

	tr, err = svc.TransferApprove(ctx, scope1, name)
	require.NoError(t, err)
	require.Equal(t, entities.TransferStatusClientApproved, tr.Status)

	t.Run("approved: the domain is the gaining registrar's", func(t *testing.T) {
		dom, err := f.domainRepo.GetDomainByName(ctx, name, true)
		require.NoError(t, err)
		assert.Equal(t, entities.ClIDType(rar2), dom.ClID)
		assert.False(t, dom.Status.PendingTransfer)
		assert.False(t, dom.AuthInfo.IsSet())
		assert.False(t, dom.TransferredAt.IsZero())
		assert.WithinDuration(t, tr.ExpiryDate, dom.ExpiryDate, time.Microsecond)
		assert.False(t, dom.RGPStatus.TransferPeriodEnd.IsZero())
		byName := map[string]entities.ClIDType{}
		for _, h := range dom.Hosts {
			byName[h.Name.String()] = h.ClID
		}
		assert.Equal(t, map[string]entities.ClIDType{
			"ns1." + name:          entities.ClIDType(rar2), // moved with the domain
			"ns1." + rar1 + ".net": entities.ClIDType(rar2), // the gaining registrar's own
			"ns." + other:          entities.ClIDType(rar1), // its domain's sponsor's
		}, byName)
	})
	t.Run("hosts: moved, created and left as the resolution says", func(t *testing.T) {
		moved := hostRow("ns1."+name, rar2)
		require.NotNil(t, moved)
		assert.Nil(t, hostRow("ns1."+name, rar1), "moved, not copied")
		assert.NotNil(t, hostRow(name, rar2), "a host named as the domain moves too")
		assert.Nil(t, hostRow(name, rar1))
		var addrs int64
		require.NoError(t, db.Table("host_addresses").Where("host_ro_id = ?", moved.RoID).Count(&addrs).Error)
		assert.Equal(t, int64(1), addrs, "its address goes with it")
		assert.True(t, moved.Linked)
		assert.True(t, moved.InBailiwick)
		created := hostRow("ns1."+rar1+".net", rar2)
		require.NotNil(t, created, "created for the gaining registrar")
		assert.True(t, created.Linked)
		left := hostRow("ns1."+rar1+".net", rar1)
		require.NotNil(t, left)
		assert.True(t, left.Linked, "still the nameserver of the losing registrar's other domain")
		assert.Nil(t, hostRow("ns."+other, rar2), "no copy under another registrar's domain")
	})
	t.Run("the gaining registrar is told, and the transfer is billed", func(t *testing.T) {
		msg, _, err := polls.Request(ctx, scope2)
		require.NoError(t, err)
		require.NotNil(t, msg)
		assert.Contains(t, msg.Message, "replace them with your own")
		assert.Equal(t, entities.TransferStatusClientApproved, msg.Transfer.Status)
		var types []string
		require.NoError(t, db.Table("domain_events").Where("subject = ? AND type LIKE ?", name, "domain.transfer%").Pluck("type", &types).Error)
		assert.ElementsMatch(t, []string{EventDomainTransferRequested, EventDomainTransferred}, types)
		var data string
		require.NoError(t, db.Table("domain_events").Select("data::text").Where("subject = ? AND type = ?", name, EventDomainTransferred).Scan(&data).Error)
		assert.Contains(t, data, `"TransactionType": "transfer"`)
		assert.NotContains(t, data, code)
	})
	t.Run("answered once", func(t *testing.T) {
		_, err := svc.TransferApprove(ctx, scope2, name)
		require.ErrorIs(t, err, entities.ErrTransferNotPending)
		_, err = svc.TransferReject(ctx, scope1, name)
		require.ErrorIs(t, err, entities.ErrEPPNotSponsor)
	})
}

func ptr[T any](v T) *T { return &v }

// TestTransferAutoApprovalOverPostgres runs the auto-approval loop's
// service path on Postgres: a transfer past its acDate is listed and
// approved by the registry, both registrars are told, a transfer not yet
// due is left alone, and a second run approves nothing twice.
func TestTransferAutoApprovalOverPostgres(t *testing.T) {
	f := newEPPFixture(t)
	ctx, db, tld, rar1, rar2 := context.Background(), f.db, f.tld, f.rar1, f.rar2
	scope1, scope2 := entities.RegistrarClID(rar1), entities.RegistrarClID(rar2)
	transfers := postgres.NewDomainTransferRepository(db, "EPPTEST")
	f.registry.SetTransferRepo(transfers)
	svc := NewEPPDomainService(EPPDomainServiceDeps{
		Domains: f.domainRepo, NNDNs: postgres.NewGormNNDNRepository(db), TLDs: f.tldRepo, Phases: postgres.NewGormPhaseRepository(db),
		Contacts: postgres.NewContactRepository(db, "EPPTEST"), Hosts: f.hostRepo, Registrars: postgres.NewGormRegistrarRepository(db),
		Transfers: transfers, Register: f.registry, CheckParallelism: 2,
	})
	polls := NewEPPPollService(transfers)
	require.NoError(t, NewAccreditationService(postgres.NewAccreditationRepository(db), postgres.NewGormRegistrarRepository(db), f.tldRepo, f.events).
		CreateAccreditation(ctx, tld, rar2))
	require.NoError(t, db.Exec("UPDATE phases SET transfer_lock_period = 0 WHERE tld_name = ?", tld).Error)

	const code = "Tr4nsf3r-C0de-Xq9#Lm2!" // #nosec G101 -- a test fixture's authInfo, not a credential
	due, notDue, gone := "due."+tld, "later."+tld, "gone."+tld
	for _, n := range []string{due, notDue, gone} {
		_, err := svc.Create(ctx, scope1, commands.EPPCreateDomainCommand{
			Name: n, RegistrantID: "c" + rar1, Contacts: []commands.EPPDomainContact{{Type: "tech", ID: "c" + rar1}},
			HostNames: []string{"ns1." + rar1 + ".net"}, AuthInfo: code,
		})
		require.NoError(t, err)
		_, err = svc.TransferRequest(ctx, scope2, commands.EPPTransferDomainCommand{Name: n, AuthInfo: ptr(code)})
		require.NoError(t, err)
	}
	// The losing registrar did not answer in time: due's pending period ended
	// an hour ago.
	require.NoError(t, db.Exec("UPDATE domain_transfers SET action_date = ? WHERE domain_name = ?", time.Now().UTC().Add(-time.Hour), due).Error)
	// gone was due earlier still, but the domain has since been deleted: its
	// transfer stays pending in the history, and must not hold up the sweep.
	require.NoError(t, db.Exec("UPDATE domain_transfers SET action_date = ? WHERE domain_name = ?", time.Now().UTC().Add(-2*time.Hour), gone).Error)
	goneDom, err := f.domainRepo.GetDomainByName(ctx, gone, false)
	require.NoError(t, err)
	goneID, err := goneDom.RoID.Int64()
	require.NoError(t, err)
	require.NoError(t, db.Exec("DELETE FROM domain_hosts WHERE domain_ro_id = ?", goneID).Error)
	require.NoError(t, db.Exec("DELETE FROM domains WHERE ro_id = ?", goneID).Error)
	for _, s := range []entities.RegistrarClID{scope1, scope2} {
		for {
			m, _, err := polls.Request(ctx, s)
			require.NoError(t, err)
			if m == nil {
				break
			}
			_, err = polls.Ack(ctx, s, strconv.FormatInt(m.ID, 10))
			require.NoError(t, err)
		}
	}

	now := time.Now().UTC()
	listed, err := f.registry.ListDueTransfers(ctx, now, time.Time{}, 0, 1000)
	require.NoError(t, err)
	names := dueNames(listed)
	require.Contains(t, names, due)
	require.NotContains(t, names, notDue, "not yet due")
	require.NotContains(t, names, gone, "the domain no longer exists")

	t.Run("the cursor passes over what a sweep has listed", func(t *testing.T) {
		var mine DueTransfer
		for _, d := range listed {
			if d.DomainName == due {
				mine = d
			}
		}
		after, err := f.registry.ListDueTransfers(ctx, now, mine.ActionDate, mine.ID, 1000)
		require.NoError(t, err)
		assert.NotContains(t, dueNames(after), due)
	})

	// notDue is passed too: the batch re-checks each domain, and skips it.
	res := f.registry.BatchApproveDueTransfers(ctx, []string{due, notDue}, now)
	assert.Equal(t, []string{due}, res.Succeeded)
	assert.Equal(t, []string{notDue}, res.Skipped)
	assert.Empty(t, res.Failed)

	t.Run("approved by the registry", func(t *testing.T) {
		dom, err := f.domainRepo.GetDomainByName(ctx, due, true)
		require.NoError(t, err)
		assert.Equal(t, entities.ClIDType(rar2), dom.ClID)
		assert.False(t, dom.Status.PendingTransfer)
		tr, err := transfers.LatestTransfer(ctx, dom.RoID)
		require.NoError(t, err)
		assert.Equal(t, entities.TransferStatusServerApproved, tr.Status)
		assert.Equal(t, entities.ClIDType(rar1), tr.ActingClID, "acID stays the losing registrar when the registry acts")
		assert.Equal(t, entities.ClIDType(rar2), dom.Hosts[0].ClID, "the external nameserver is re-linked to the gaining registrar's")
	})
	t.Run("both registrars are told", func(t *testing.T) {
		for _, s := range []entities.RegistrarClID{scope1, scope2} {
			m, n, err := polls.Request(ctx, s)
			require.NoError(t, err)
			require.NotNil(t, m, "registrar %s", s)
			assert.Equal(t, int64(1), n)
			assert.Contains(t, m.Message, entities.PollMsgTransferServerApproved)
			assert.Equal(t, entities.TransferStatusServerApproved, m.Transfer.Status)
		}
	})
	t.Run("the domain not due stays pending", func(t *testing.T) {
		dom, err := f.domainRepo.GetDomainByName(ctx, notDue, false)
		require.NoError(t, err)
		assert.True(t, dom.Status.PendingTransfer)
		assert.Equal(t, entities.ClIDType(rar1), dom.ClID)
	})
	t.Run("a second run approves nothing twice", func(t *testing.T) {
		again := f.registry.BatchApproveDueTransfers(ctx, []string{due}, now)
		assert.Empty(t, again.Succeeded)
		assert.Equal(t, []string{due}, again.Skipped)
		listed, err := f.registry.ListDueTransfers(ctx, now, time.Time{}, 0, 1000)
		require.NoError(t, err)
		assert.NotContains(t, dueNames(listed), due)
	})
}

func dueNames(due []DueTransfer) []string {
	out := make([]string, len(due))
	for i, d := range due {
		out[i] = d.DomainName
	}
	return out
}
