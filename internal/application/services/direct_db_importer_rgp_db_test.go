package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// A deposit says which RGP period a deleted domain is in, not when it ends:
// the import starts that period in full, under the TLD's current GA phase
// (entities.Domain.ApplyDepositEndOfLife). A domain pending restore keeps its
// restore, its report now due on this registry.
func TestDirectDBImporter_ImportDomains_StartsTheEndOfLifeOfDeletedDomains(t *testing.T) {
	db := domainsImportDB(t)
	importer := newImporter(t)
	const tld = "rgpimp"
	clid := seedFixtures(t, db, tld, 9871)
	// The phase sets its own periods: the import follows them.
	phase, err := entities.NewPhase("GA1", string(entities.PhaseTypeGA), time.Now().UTC().AddDate(0, 0, -1))
	require.NoError(t, err)
	phase.TLDName = tld
	phase.Policy.RedemptionGP, phase.Policy.PendingRestorePeriod = 10, 3
	_, err = postgres.NewGormPhaseRepository(db).CreatePhase(context.Background(), phase)
	require.NoError(t, err)

	red, res, gone, live := "red."+tld, "res."+tld, "gone."+tld, "live."+tld
	staged := stagedDomains(t, [3]string{red, "", clid}, [3]string{res, "", clid}, [3]string{gone, "", clid}, [3]string{live, "", clid})
	_, err = staged.Exec(`CREATE TABLE domain_rgp_statuses (domain_name TEXT, rgp_status TEXT)`)
	require.NoError(t, err)
	for _, s := range [][2]string{
		{red, "pendingDelete"}, {res, "pendingDelete"}, {gone, "pendingDelete"}, {live, "ok"},
	} {
		_, err = staged.Exec(`INSERT INTO domain_statuses (domain_name, status) VALUES (?, ?)`, s[0], s[1])
		require.NoError(t, err)
	}
	for _, s := range [][2]string{{red, "redemptionPeriod"}, {res, "pendingRestore"}, {gone, "pendingDelete"}, {live, "addPeriod"}} {
		_, err = staged.Exec(`INSERT INTO domain_rgp_statuses (domain_name, rgp_status) VALUES (?, ?)`, s[0], s[1])
		require.NoError(t, err)
	}

	before := time.Now().UTC()
	_, _, _, err = importer.ImportDomains(context.Background(), staged, tld, nil, services.ImportCursor{}, func(services.ImportCursor) {})
	require.NoError(t, err)

	repo := postgres.NewDomainRepository(db, "TEST")
	get := func(name string) *entities.Domain {
		d, err := repo.GetDomainByName(context.Background(), name, false)
		require.NoError(t, err)
		return d
	}
	within := func(t *testing.T, want, got time.Time) {
		t.Helper()
		assert.WithinDuration(t, want, got, time.Minute)
	}

	d := get(red)
	assert.True(t, d.Status.PendingDelete)
	assert.False(t, d.Status.PendingRestore)
	within(t, before.AddDate(0, 0, 10), d.RGPStatus.RedemptionPeriodEnd)
	within(t, before.AddDate(0, 0, 10+entities.PendingDeleteGP), d.RGPStatus.PurgeDate)

	d = get(res)
	assert.True(t, d.Status.PendingDelete)
	assert.True(t, d.Status.PendingRestore)
	within(t, before.AddDate(0, 0, 3), d.RGPStatus.PendingRestoreEnd)
	within(t, before.AddDate(0, 0, 10), d.RGPStatus.RedemptionPeriodEnd)
	assert.True(t, d.RestorePending(time.Now()), "its report is due here")

	d = get(gone)
	assert.True(t, d.Status.PendingDelete)
	within(t, before, d.RGPStatus.RedemptionPeriodEnd)
	within(t, before.AddDate(0, 0, entities.PendingDeleteGP), d.RGPStatus.PurgeDate)

	d = get(live)
	assert.False(t, d.Status.PendingDelete)
	assert.True(t, d.RGPStatus.PurgeDate.IsZero())
}
