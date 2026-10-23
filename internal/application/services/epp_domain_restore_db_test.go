package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestEPPDomainRestoreOverPostgres runs the RGP restore (RFC 3915) through
// the EPP service on the Postgres repositories: the request and the report,
// each a version-checked write; the purge holding off a pending restore;
// and the revert of a lapsed one, as RestoreWorkflow runs it.
func TestEPPDomainRestoreOverPostgres(t *testing.T) {
	f := newEPPFixture(t)
	db, svc, registry, tld := f.db, f.domains, f.registry, f.tld
	scope := entities.RegistrarClID(f.rar1)
	ctx := appcontext.WithUserID(context.Background(), "epp:"+f.rar1)
	now := time.Now().UTC()
	report := entities.RestoreReport{
		PreData: "before", PostData: "after", DelTime: now.Add(-48 * time.Hour), ResTime: now.Add(-time.Minute),
		ResReason: "Registrant error.", Statements: []string{"Not to use or sell it.", "Factual."},
	}
	request := commands.EPPUpdateDomainCommand{Restore: &commands.EPPRestore{}}
	reportCmd := commands.EPPUpdateDomainCommand{Restore: &commands.EPPRestore{Report: &report}}
	update := func(name string, cmd commands.EPPUpdateDomainCommand) (*entities.Domain, error) {
		cmd.Name = name
		return svc.Update(ctx, scope, cmd)
	}
	// deleteRestorable registers name and deletes it after its add grace
	// period, so it is in its redemption period.
	deleteRestorable := func(t *testing.T, name string) *entities.Domain {
		t.Helper()
		_, err := svc.Create(ctx, scope, commands.EPPCreateDomainCommand{
			Name: name, RegistrantID: "c" + f.rar1, Contacts: []commands.EPPDomainContact{{Type: "tech", ID: "c" + f.rar1}},
			HostNames: []string{"ns1." + f.rar1 + ".net"},
		})
		require.NoError(t, err)
		require.NoError(t, db.Exec("UPDATE domains SET add_period_end = ? WHERE name = ?", now.Add(-time.Hour), name).Error)
		dom, err := svc.Delete(ctx, scope, name)
		require.NoError(t, err)
		require.True(t, dom.Status.PendingDelete)
		return dom
	}
	stored := func(t *testing.T, name string) *entities.Domain {
		t.Helper()
		d, err := f.domainRepo.GetDomainByName(context.Background(), name, true)
		require.NoError(t, err)
		return d
	}
	eventsOf := func(t *testing.T, name string) []string {
		t.Helper()
		var types []string
		require.NoError(t, db.Table("domain_events").Where("subject = ?", name).Pluck("type", &types).Error)
		return types
	}

	back, lapsed := "back."+tld, "lapsed."+tld
	deleted := deleteRestorable(t, back)

	t.Run("requested: pendingRestore, still pendingDelete, until the deadline", func(t *testing.T) {
		_, err := update(back, request)
		require.NoError(t, err)
		d := stored(t, back)
		assert.True(t, d.Status.PendingRestore)
		assert.True(t, d.Status.PendingDelete)
		assert.WithinDuration(t, now.AddDate(0, 0, entities.PendingRestorePeriod), d.RGPStatus.PendingRestoreEnd, time.Minute)
		assert.Len(t, d.Hosts, 1, "the nameservers are kept")
	})
	t.Run("a pending restore holds the purge, even past the purge date", func(t *testing.T) {
		require.NoError(t, db.Exec("UPDATE domains SET purge_date = ? WHERE name = ?", now.Add(-time.Hour), back).Error)
		purgeable, err := f.domainRepo.ListPurgeableDomains(context.Background(), now, 1000, "", tld, "")
		require.NoError(t, err)
		for _, d := range purgeable {
			assert.NotEqual(t, back, d.Name.String())
		}
		require.NoError(t, db.Exec("UPDATE domains SET purge_date = ? WHERE name = ?", deleted.RGPStatus.PurgeDate, back).Error)
	})
	t.Run("a stale read of the request is not written over (version check)", func(t *testing.T) {
		stale := stored(t, back)
		fresh := stored(t, back)
		_, err := registry.CompleteRestoreAsRead(ctx, fresh, report, entities.ClIDType(f.rar1))
		require.NoError(t, err)
		_, err = registry.CompleteRestoreAsRead(ctx, stale, report, entities.ClIDType(f.rar1))
		assert.ErrorIs(t, err, entities.ErrDomainChanged)
	})
	t.Run("reported: restored and renewed, with its charges recorded", func(t *testing.T) {
		d := stored(t, back)
		assert.False(t, d.Status.PendingRestore)
		assert.False(t, d.Status.PendingDelete)
		assert.True(t, d.RGPStatus.PendingRestoreEnd.IsZero())
		assert.True(t, d.RGPStatus.RedemptionPeriodEnd.IsZero())
		assert.True(t, d.RGPStatus.PurgeDate.IsZero())
		assert.Equal(t, deleted.ExpiryDate.AddDate(1, 0, 0).Unix(), d.ExpiryDate.Unix())
		assert.Subset(t, eventsOf(t, back), []string{EventDomainRestoreRequested, EventDomainRestored, EventDomainRestoreRenewed})
		_, err := update(back, reportCmd)
		assert.ErrorIs(t, err, entities.ErrRestoreNotPending, "nothing left to report")
	})

	deleteRestorable(t, lapsed)
	_, err := update(lapsed, request)
	require.NoError(t, err)
	before := stored(t, lapsed)
	// No report in time: the deadline passed an hour ago.
	require.NoError(t, db.Exec("UPDATE domains SET pending_restore_end = ? WHERE name = ?", now.Add(-time.Hour), lapsed).Error)

	t.Run("a report after the deadline is refused", func(t *testing.T) {
		_, err := update(lapsed, reportCmd)
		assert.ErrorIs(t, err, entities.ErrRestoreNotPending)
	})
	listed, err := registry.ListLapsedRestores(context.Background(), now, time.Time{}, 0, 1000)
	require.NoError(t, err)
	var mine LapsedRestore
	for _, l := range listed {
		assert.NotEqual(t, back, l.DomainName, "restored, not pending")
		if l.DomainName == lapsed {
			mine = l
		}
	}
	require.Equal(t, lapsed, mine.DomainName)
	t.Run("the cursor passes over what a sweep has listed", func(t *testing.T) {
		after, err := registry.ListLapsedRestores(context.Background(), now, mine.RestoreEnd, mine.RoID, 1000)
		require.NoError(t, err)
		for _, l := range after {
			assert.NotEqual(t, lapsed, l.DomainName)
		}
	})
	t.Run("reverted: back in redemption, as before the request", func(t *testing.T) {
		res := registry.BatchRevertLapsedRestores(context.Background(), []string{lapsed, back}, now)
		assert.Equal(t, []string{lapsed}, res.Succeeded)
		assert.Equal(t, []string{back}, res.Skipped)
		assert.Empty(t, res.Failed)
		d := stored(t, lapsed)
		assert.False(t, d.Status.PendingRestore)
		assert.True(t, d.Status.PendingDelete)
		assert.True(t, d.RGPStatus.PendingRestoreEnd.IsZero())
		assert.Equal(t, before.RGPStatus.RedemptionPeriodEnd.Unix(), d.RGPStatus.RedemptionPeriodEnd.Unix())
		assert.Equal(t, before.RGPStatus.PurgeDate.Unix(), d.RGPStatus.PurgeDate.Unix())
		assert.Equal(t, []string{"redemptionPeriod"}, d.RGPStatuses(time.Now()))
		assert.Contains(t, eventsOf(t, lapsed), EventDomainRestoreReverted)
	})
	t.Run("the sponsor is told by poll message", func(t *testing.T) {
		polls := NewEPPPollService(postgres.NewDomainTransferRepository(db, "EPPTEST"))
		m, n, err := polls.Request(context.Background(), scope)
		require.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, int64(1), n, "one message, for the one domain reverted")
		assert.Nil(t, m.Transfer)
		assert.Contains(t, m.Message, entities.PollMsgRestoreLapsed+": no restore report for "+lapsed)
	})
	t.Run("a second run reverts nothing twice", func(t *testing.T) {
		again := registry.BatchRevertLapsedRestores(context.Background(), []string{lapsed}, now)
		assert.Empty(t, again.Succeeded)
		assert.Equal(t, []string{lapsed}, again.Skipped)
	})
	t.Run("the registrar may request again within redemption", func(t *testing.T) {
		_, err := update(lapsed, request)
		require.NoError(t, err)
		assert.True(t, stored(t, lapsed).Status.PendingRestore)
	})
}
