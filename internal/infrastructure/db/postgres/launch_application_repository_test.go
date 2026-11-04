package postgres

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// testApplication is an application for name by clid in the auction phase
// of .apptest, with a host and a DS record, as the EPP service would file
// it. The ID is a fresh number for each call.
func testApplication(t *testing.T, id int64, name, clid string) *entities.LaunchApplication {
	t.Helper()
	roid, err := entities.NewRoidType(id, entities.RoidTypeApplication, "TEST")
	require.NoError(t, err)
	h, err := entities.HashAuthInfo("Tr4nsf3r-C0de-Xq9#Lm2!")
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &entities.LaunchApplication{
		ID: roid, Name: entities.DomainName(name), TLDName: "apptest", PhaseName: "auction", PhaseMode: entities.LaunchModeLandrush,
		ClID: entities.ClIDType(clid), Status: entities.ApplicationStatusPendingAllocation,
		RegistrantID: "c-" + entities.ClIDType(clid), TechID: "c-" + entities.ClIDType(clid),
		HostNames: []string{"ns1." + clid + ".net"}, DSRecords: []entities.DSRecord{{KeyTag: 2371, Algorithm: 13, DigestType: 2, Digest: "49FD46E6C4B45C55D4AC69CBD3CD34AC1AFE51DE3DC8B27C3F72E84BC2E1D3F4"}},
		Years: 2, AuthInfo: h, Quote: entities.ApplicationQuote{Currency: "USD", Amount: 3000, Class: "standard"},
		CreatedAt: now, UpdatedAt: now,
	}
}

func TestLaunchApplicationRepository(t *testing.T) {
	db := getTestDB()
	ctx := context.Background()
	repo := NewLaunchApplicationRepository(db, "TEST")
	t.Cleanup(func() {
		db.Where("tld_name = ?", "apptest").Delete(&LaunchApplicationRecord{})
	})
	base := time.Now().UnixNano() % 1e9 * 10 // IDs unique to this run

	t.Run("create, read and list", func(t *testing.T) {
		a := testApplication(t, base+1, "bid.apptest", "rar-one")
		stored, err := repo.CreateApplication(ctx, a)
		require.NoError(t, err)
		assert.Equal(t, a.ID, stored.ID, "the ID carries the repository ID")
		assert.Equal(t, a.HostNames, stored.HostNames)
		assert.Equal(t, a.DSRecords, stored.DSRecords)
		assert.Equal(t, a.Quote, stored.Quote)
		assert.Equal(t, entities.LaunchModeLandrush, stored.PhaseMode)
		assert.True(t, stored.AuthInfo.Matches("Tr4nsf3r-C0de-Xq9#Lm2!"))
		assert.Empty(t, stored.DomainRoID)

		got, err := repo.GetApplicationByID(ctx, a.ID)
		require.NoError(t, err)
		assert.Equal(t, stored, got)

		other := testApplication(t, base+2, "bid.apptest", "rar-two")
		_, err = repo.CreateApplication(ctx, other)
		require.NoError(t, err, "another registrar applies for the same name")
		list, err := repo.ListApplicationsByName(ctx, "bid.apptest")
		require.NoError(t, err)
		require.Len(t, list, 2)
		assert.Equal(t, []entities.RoidType{a.ID, other.ID}, []entities.RoidType{list[0].ID, list[1].ID}, "oldest first")

		n, err := repo.CountApplicationsByPhase(ctx, "apptest", "auction")
		require.NoError(t, err)
		assert.Equal(t, int64(2), n)
		n, err = repo.CountApplicationsByPhase(ctx, "apptest", "other")
		require.NoError(t, err)
		assert.Zero(t, n)
	})
	t.Run("one open application per registrar, name and phase", func(t *testing.T) {
		again := testApplication(t, base+3, "bid.apptest", "rar-one")
		_, err := repo.CreateApplication(ctx, again)
		assert.ErrorIs(t, err, entities.ErrApplicationExists)
		again.PhaseName = "other"
		_, err = repo.CreateApplication(ctx, again)
		assert.NoError(t, err, "another phase")
	})
	t.Run("an ID that is not an application's", func(t *testing.T) {
		for _, id := range []entities.RoidType{"1_DOM-TEST", "nope", entities.RoidType("1_APP-OTHER")} {
			_, err := repo.GetApplicationByID(ctx, id)
			assert.ErrorIs(t, err, entities.ErrApplicationNotFound, id)
		}
		_, err := repo.GetApplicationByID(ctx, "999999999999_APP-TEST")
		assert.ErrorIs(t, err, entities.ErrApplicationNotFound)
	})
	t.Run("update if unchanged", func(t *testing.T) {
		a := testApplication(t, base+4, "chg.apptest", "rar-one")
		stored, err := repo.CreateApplication(ctx, a)
		require.NoError(t, err)
		stale := stored.DeepCopy()

		stored.HostNames = []string{"ns2.rar-one.net", "ns3.rar-one.net"}
		stored.DSRecords = nil
		stored.BillingID = "c-rar-one"
		stored.Status = entities.ApplicationStatusRejected
		stored.Reason = "went elsewhere"
		updated, err := repo.UpdateApplicationIfUnchanged(ctx, stored)
		require.NoError(t, err)
		assert.Equal(t, []string{"ns2.rar-one.net", "ns3.rar-one.net"}, updated.HostNames)
		assert.Empty(t, updated.DSRecords, "replaced with the application")
		assert.Equal(t, entities.ClIDType("c-rar-one"), updated.BillingID)
		assert.Equal(t, entities.ApplicationStatusRejected, updated.Status)
		assert.Equal(t, "went elsewhere", updated.Reason)
		assert.True(t, updated.UpdatedAt.After(stale.UpdatedAt))

		_, err = repo.UpdateApplicationIfUnchanged(ctx, stale)
		assert.ErrorIs(t, err, entities.ErrApplicationChanged, "written on a version since replaced")

		got, err := repo.GetApplicationByID(ctx, a.ID)
		require.NoError(t, err)
		assert.Equal(t, updated, got)

		updated.Status = entities.ApplicationStatusAllocated
		updated.DomainRoID = "77_DOM-TEST"
		allocated, err := repo.UpdateApplicationIfUnchanged(ctx, updated)
		require.NoError(t, err)
		assert.Equal(t, entities.RoidType("77_DOM-TEST"), allocated.DomainRoID)

		missing := testApplication(t, base+5, "none.apptest", "rar-one")
		_, err = repo.UpdateApplicationIfUnchanged(ctx, missing)
		assert.ErrorIs(t, err, entities.ErrApplicationNotFound)
	})
	t.Run("a transition made on the read version is written", func(t *testing.T) {
		a := testApplication(t, base+8, "won.apptest", "rar-one")
		a.Status = entities.ApplicationStatusPendingValidation
		stored, err := repo.CreateApplication(ctx, a)
		require.NoError(t, err)
		time.Sleep(2 * time.Millisecond) // the transition happens later than the read
		require.NoError(t, stored.Validated())
		validated, err := repo.UpdateApplicationIfUnchanged(ctx, stored)
		require.NoError(t, err, "the entity left the read version alone")
		assert.Equal(t, entities.ApplicationStatusPendingAllocation, validated.Status)
		require.NoError(t, validated.Allocate("78_DOM-TEST"))
		allocated, err := repo.UpdateApplicationIfUnchanged(ctx, validated)
		require.NoError(t, err)
		assert.Equal(t, entities.ApplicationStatusAllocated, allocated.Status)
		assert.Equal(t, entities.RoidType("78_DOM-TEST"), allocated.DomainRoID)
		_, err = repo.UpdateApplicationIfUnchanged(ctx, validated)
		assert.ErrorIs(t, err, entities.ErrApplicationChanged, "the version before allocation is stale")
	})
	t.Run("list by phase and status, a page at a time", func(t *testing.T) {
		all, err := repo.ListApplicationsByPhase(ctx, "apptest", "auction", "", 100, "")
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(all), 2)
		first, err := repo.ListApplicationsByPhase(ctx, "apptest", "auction", "", 1, "")
		require.NoError(t, err)
		require.Len(t, first, 1)
		assert.Equal(t, all[0].ID, first[0].ID)
		id, _ := first[0].ID.Int64()
		next, err := repo.ListApplicationsByPhase(ctx, "apptest", "auction", "", 1, strconv.FormatInt(id, 10))
		require.NoError(t, err)
		require.Len(t, next, 1)
		assert.Equal(t, all[1].ID, next[0].ID, "the cursor is the last ID")
		_, err = repo.ListApplicationsByPhase(ctx, "apptest", "auction", "", 1, "not-a-cursor")
		assert.ErrorIs(t, err, entities.ErrInvalidApplication)
		rejected, err := repo.ListApplicationsByPhase(ctx, "apptest", "auction", entities.ApplicationStatusRejected, 100, "")
		require.NoError(t, err)
		for _, a := range rejected {
			assert.Equal(t, entities.ApplicationStatusRejected, a.Status)
		}
		none, err := repo.ListApplicationsByPhase(ctx, "apptest", "nothing", "", 100, "")
		require.NoError(t, err)
		assert.Empty(t, none)
	})
	t.Run("an open application counts as a contact link", func(t *testing.T) {
		contacts := NewContactRepository(db, "TEST")
		n, err := contacts.CountContactLinks(ctx, "c-rar-two")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, n, int64(1), "bid.apptest by rar-two names c-rar-two and is open")
		n, err = contacts.CountContactLinks(ctx, "c-nobody")
		require.NoError(t, err)
		assert.Zero(t, n)
	})
	t.Run("delete if unchanged", func(t *testing.T) {
		a := testApplication(t, base+6, "gone.apptest", "rar-one")
		stored, err := repo.CreateApplication(ctx, a)
		require.NoError(t, err)
		stale := stored.DeepCopy()
		stored.Years = 3
		stored, err = repo.UpdateApplicationIfUnchanged(ctx, stored)
		require.NoError(t, err)

		assert.ErrorIs(t, repo.DeleteApplicationIfUnchanged(ctx, stale), entities.ErrApplicationChanged)
		require.NoError(t, repo.DeleteApplicationIfUnchanged(ctx, stored))
		assert.ErrorIs(t, repo.DeleteApplicationIfUnchanged(ctx, stored), entities.ErrApplicationNotFound)
		_, err = repo.GetApplicationByID(ctx, a.ID)
		assert.ErrorIs(t, err, entities.ErrApplicationNotFound)
		var hosts int64
		id, _ := a.ID.Int64()
		require.NoError(t, db.Model(&LaunchApplicationHost{}).Where("application_id = ?", id).Count(&hosts).Error)
		assert.Zero(t, hosts, "its nameservers went with it")

		_, err = repo.CreateApplication(ctx, testApplication(t, base+7, "gone.apptest", "rar-one"))
		assert.NoError(t, err, "the registrar may apply again")
	})
}
