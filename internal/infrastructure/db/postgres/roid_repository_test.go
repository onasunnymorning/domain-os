package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// roidMigrationDB is a database of its own, created fresh: the test drops a
// column, which would break every other suite on the shared one. It returns
// a function that opens a new connection to it.
func roidMigrationDB(t *testing.T) func() *gorm.DB {
	t.Helper()
	const name = "dos_roid_migration_test"
	admin, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s sslmode=%s", dbHost, dbPortString, dbUser, dbPass, sslmode))
	require.NoError(t, err)
	defer admin.Close()
	_, err = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
	require.NoError(t, err)
	_, err = admin.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)

	t.Cleanup(func() {
		if admin, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s sslmode=%s", dbHost, dbPortString, dbUser, dbPass, sslmode)); err == nil {
			_, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
			_ = admin.Close()
		}
	})
	open := func() *gorm.DB {
		db, err := gorm.Open(postgres.Open(fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", dbUser, dbPass, dbHost, dbPortString, name, sslmode)))
		require.NoError(t, err)
		t.Cleanup(func() {
			if sqlDB, err := db.DB(); err == nil {
				_ = sqlDB.Close()
			}
		})
		return db
	}
	require.NoError(t, AutoMigrate(open()))
	return open
}

// TestROIDRepositoryColumn proves ADR 0017's storage rule on Postgres: a
// ROID's repository ID is stored with the object, the rows that predate the
// column read back with the "APEX" they were minted with, and nothing that
// leaves the column out gets a value silently.
func TestROIDRepositoryColumn(t *testing.T) {
	open := roidMigrationDB(t)
	db := open()
	ctx := context.Background()

	rar, err := entities.NewRegistrar("roidrar", "roidrar", "ops@roidrar.example", 4242, getValidRegistrarPostalInfoArr())
	require.NoError(t, err)
	_, err = NewGormRegistrarRepository(db).Create(ctx, rar)
	require.NoError(t, err)
	ro, err := entities.NewRegistryOperator("roidry", "roidry", "ry@roidry.example")
	require.NoError(t, err)
	_, err = NewGORMRegistryOperatorRepository(db).Create(ctx, ro)
	require.NoError(t, err)
	tld, err := entities.NewTLD("roidtld", "roidry")
	require.NoError(t, err)
	require.NoError(t, NewGormTLDRepo(db).Create(ctx, tld))
	ga, err := entities.NewPhase("GA1", "GA", time.Now().AddDate(0, 0, -1).UTC())
	require.NoError(t, err)
	ga.TLDName = "roidtld"
	_, err = NewGormPhaseRepository(db).CreatePhase(ctx, ga)
	require.NoError(t, err)

	var contacts *ContactRepository
	var hosts *HostRepository
	var domains *DomainRepository
	// connect starts over on a new connection, as a service does after the
	// migrate job: a connection that has prepared a SELECT * before the
	// schema changed refuses it afterwards ("cached plan must not change
	// result type").
	connect := func() {
		db = open()
		contacts, hosts, domains = NewContactRepository(db), NewGormHostRepository(db), NewDomainRepository(db)
	}
	connect()
	create := func(t *testing.T, n int64, repository string) {
		t.Helper()
		c := getValidContactEntity()
		c.ID, c.ClID, c.CrRr, c.UpRr = entities.ClIDType(fmt.Sprintf("roidc%d", n)), "roidrar", "roidrar", "roidrar"
		c.RoID = entities.RoidType(fmt.Sprintf("%d_CONT-%s", n, repository))
		_, err := contacts.CreateContact(ctx, c)
		require.NoError(t, err)
		h, err := entities.NewHost(fmt.Sprintf("ns%d.roid.example", n), fmt.Sprintf("%d_HOST-%s", n, repository), "roidrar")
		require.NoError(t, err)
		_, err = hosts.CreateHost(ctx, h)
		require.NoError(t, err)
		d, err := entities.NewDomain(fmt.Sprintf("%d_DOM-%s", n, repository), fmt.Sprintf("d%d.roidtld", n), "roidrar", "Roid-Auth-1!")
		require.NoError(t, err)
		_, err = domains.Create(ctx, d)
		require.NoError(t, err)
	}
	read := func(t *testing.T, n int64) []string {
		t.Helper()
		c, err := contacts.GetContactByID(ctx, fmt.Sprintf("roidc%d", n))
		require.NoError(t, err)
		h, err := hosts.GetHostByRoid(ctx, n)
		require.NoError(t, err)
		d, err := domains.GetDomainByID(ctx, n, false)
		require.NoError(t, err)
		return []string{c.RoID.String(), h.RoID.String(), d.RoID.String()}
	}

	t.Run("a fresh table has the column NOT NULL, with no default", func(t *testing.T) {
		for _, table := range roidRepositoryTables {
			var nullable string
			var def sql.NullString
			require.NoError(t, db.Raw(`SELECT is_nullable, column_default FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = ? AND column_name = 'ro_id_repository'`, table).Row().Scan(&nullable, &def))
			assert.Equal(t, "NO", nullable, table)
			assert.False(t, def.Valid, "%s: default %q", table, def.String)
		}
	})

	t.Run("an object keeps the repository ID it was minted with", func(t *testing.T) {
		create(t, 1, "OLDID")
		create(t, 2, "NEWID")
		assert.Equal(t, []string{"1_CONT-OLDID", "1_HOST-OLDID", "1_DOM-OLDID"}, read(t, 1))
		assert.Equal(t, []string{"2_CONT-NEWID", "2_HOST-NEWID", "2_DOM-NEWID"}, read(t, 2))
	})

	t.Run("rows from before the column read back as APEX", func(t *testing.T) {
		// The schema as it was before Phase 4d: the rows keep their numbers
		// and lose the suffix, which the code used to add as "APEX".
		for _, table := range roidRepositoryTables {
			require.NoError(t, db.Exec("ALTER TABLE "+table+" DROP COLUMN ro_id_repository").Error)
		}
		require.NoError(t, AutoMigrate(db))
		connect()
		assert.Equal(t, []string{"1_CONT-APEX", "1_HOST-APEX", "1_DOM-APEX"}, read(t, 1))
		assert.Equal(t, []string{"2_CONT-APEX", "2_HOST-APEX", "2_DOM-APEX"}, read(t, 2))

		create(t, 3, "NEWID")
		assert.Equal(t, []string{"3_CONT-NEWID", "3_HOST-NEWID", "3_DOM-NEWID"}, read(t, 3), "new objects after the migration")
	})

	t.Run("the backfill default does not outlive the migration", func(t *testing.T) {
		err := db.Exec(`INSERT INTO hosts (ro_id, name, cl_id) VALUES (99, 'ns99.roid.example', 'roidrar')`).Error
		require.Error(t, err, "a writer that leaves the column out must fail, not get APEX")
		assert.Contains(t, err.Error(), "ro_id_repository")
	})

	// The lifecycle lists select a few columns, not the row, and their ROIDs
	// are the next page's cursor: without ro_id_repository they came back
	// empty.
	t.Run("the lifecycle lists read the repository ID too", func(t *testing.T) {
		require.NoError(t, db.Exec(`UPDATE domains SET expiry_date = CASE WHEN ro_id = 3 THEN now() - interval '1 day' ELSE now() + interval '1 year' END`).Error)
		expiring, err := domains.ListExpiringDomains(ctx, time.Now(), 10, "", "roidtld", "")
		require.NoError(t, err)
		require.Len(t, expiring, 1)
		assert.Equal(t, "3_DOM-NEWID", expiring[0].RoID.String())

		require.NoError(t, db.Exec(`UPDATE domains SET pending_delete = true, purge_date = now() - interval '1 day' WHERE ro_id = 3`).Error)
		purgeable, err := domains.ListPurgeableDomains(ctx, time.Now(), 10, "", "roidtld", "")
		require.NoError(t, err)
		require.Len(t, purgeable, 1)
		assert.Equal(t, "3_DOM-NEWID", purgeable[0].RoID.String())

		require.NoError(t, db.Exec(`UPDATE domains SET pending_delete = false, pending_restore = true WHERE ro_id = 3`).Error)
		restored, err := domains.ListRestoredDomains(ctx, 10, "", "roidtld", "")
		require.NoError(t, err)
		require.Len(t, restored, 1)
		assert.Equal(t, "3_DOM-NEWID", restored[0].RoID.String())
	})

	t.Run("migrating again changes nothing", func(t *testing.T) {
		require.NoError(t, AutoMigrate(db))
		connect()
		assert.Equal(t, []string{"3_CONT-NEWID", "3_HOST-NEWID", "3_DOM-NEWID"}, read(t, 3))
	})
}
