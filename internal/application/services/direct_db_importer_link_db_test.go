package services_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// linkImportDB is domainsImportDB with the foreign keys on hosts dropped
// too, so hosts can be seeded without a registrar behind them.
func linkImportDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := domainsImportDB(t)
	var fks []string
	require.NoError(t, db.Raw(`SELECT conname FROM pg_constraint WHERE conrelid = 'hosts'::regclass AND contype = 'f'`).Scan(&fks).Error)
	for _, fk := range fks {
		require.NoError(t, db.Exec(fmt.Sprintf(`ALTER TABLE hosts DROP CONSTRAINT IF EXISTS %q`, fk)).Error)
	}
	return db
}

// stagedLinks is a staging database holding domain_nameservers rows.
func stagedLinks(t *testing.T, links [][2]string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", t.TempDir()+"/staged.db")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE domain_nameservers (id INTEGER PRIMARY KEY AUTOINCREMENT, domain_name TEXT NOT NULL, nameserver TEXT NOT NULL)`)
	require.NoError(t, err)
	for _, l := range links {
		_, err = db.Exec(`INSERT INTO domain_nameservers (domain_name, nameserver) VALUES (?, ?)`, l[0], l[1])
		require.NoError(t, err)
	}
	return db
}

// testImportDSN is the connection string of the database authInfoTestDB
// put in the environment.
func testImportDSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(os.Getenv("DB_USER"), os.Getenv("DB_PASS")),
		Host:     os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT"),
		Path:     "/" + os.Getenv("DB_NAME"),
		RawQuery: "sslmode=require",
	}
	return u.String()
}

// LinkDomainHosts builds a TEMP table, which lives in one session, and reads
// it in every batch. Two linkers on one shared pool, and a linker whose
// batches outnumber the pool's connections, must each see their own table
// throughout: every statement of one linking runs on one held connection.
// Before that, a batch could borrow a connection without the table, and on
// a shared pool one linker's cleanup could drop the other's table.
func TestLinkDomainHosts_SharesOnePoolAcrossConcurrentLinkers(t *testing.T) {
	db := linkImportDB(t)
	ctx := context.Background()

	// Two disjoint sets of domains, each linked to its own hosts; small
	// batches so each linking runs many batches on the pool.
	const perLinker, batch = 40, 7
	seed := func(set int) [][2]string {
		var links [][2]string
		for i := range perLinker {
			dom := fmt.Sprintf("link%d-%d.paco", set, i)
			host := fmt.Sprintf("ns%d-%d.link%d-%d.paco", set, i, set, i)
			roid := int64(900000 + set*1000 + i)
			require.NoError(t, db.Exec(`INSERT INTO domains (ro_id, name, tld_name, cl_id, auth_info, expiry_date, created_at, updated_at)
				VALUES (?, ?, 'paco', 'rar', 'x', now(), now(), now())`, roid, dom).Error)
			require.NoError(t, db.Exec(`INSERT INTO hosts (ro_id, name, cl_id, created_at, updated_at) VALUES (?, ?, 'rar', now(), now())`, roid, host).Error)
			links = append(links, [2]string{dom, host})
		}
		return links
	}
	linksA, linksB := seed(1), seed(2)

	for _, conns := range []int{1, 3} {
		t.Run(fmt.Sprintf("%d connections", conns), func(t *testing.T) {
			require.NoError(t, db.Exec(`DELETE FROM domain_hosts`).Error)
			pool, err := services.OpenImportPool(testImportDSN(), conns)
			require.NoError(t, err)
			defer pool.Close()

			run := func(links [][2]string) (int64, int64, error) {
				imp, err := services.NewDirectDBImporterWith("TEST", pool)
				require.NoError(t, err)
				imp.SetBatchSize(batch)
				return imp.LinkDomainHosts(ctx, stagedLinks(t, links), "", func(string) {})
			}
			var wg sync.WaitGroup
			results := make([][3]int64, 2)
			errs := make([]error, 2)
			for i, links := range [][][2]string{linksA, linksB} {
				wg.Add(1)
				go func() {
					defer wg.Done()
					total, inserted, err := run(links)
					results[i] = [3]int64{total, inserted}
					errs[i] = err
				}()
			}
			wg.Wait()
			for i := range 2 {
				require.NoError(t, errs[i], "linker %d", i)
				require.Equal(t, int64(perLinker), results[i][0], "linker %d read every staged link", i)
				require.Equal(t, int64(perLinker), results[i][1], "linker %d inserted every link", i)
			}
			var n int64
			require.NoError(t, db.Raw(`SELECT count(*) FROM domain_hosts`).Scan(&n).Error)
			require.Equal(t, int64(2*perLinker), n)

			// The pool is usable afterwards: the held connections were
			// released, and the temp tables are gone with their sessions.
			imp, err := services.NewDirectDBImporterWith("TEST", pool)
			require.NoError(t, err)
			_, err = imp.PG.Exec(`SELECT 1`)
			require.NoError(t, err)
			var leftover int64
			require.NoError(t, db.Raw(`SELECT count(*) FROM pg_tables WHERE tablename = '_host_dedup'`).Scan(&leftover).Error)
			require.Zero(t, leftover, "no linker's temp table outlives its session")
		})
	}
}
