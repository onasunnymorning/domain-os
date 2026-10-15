package services_test

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"testing"

	_ "github.com/lib/pq"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

// These tests prove an ingest phase resumed from a heartbeated cursor writes
// exactly the rows after it: none skipped, none written again. They run the
// importers against a real Postgres in a database of their own
// (authInfoTestDB); `make test` provides it on TEST_DB_PORT (5433).

const resumeBatchSize = 2

// resumeCase is one ingest phase under test.
type resumeCase struct {
	// run imports from the given cursor with a fresh importer committing every
	// resumeBatchSize rows, and returns its total and skipped counts.
	run func(t *testing.T, from services.ImportCursor, heartbeat func(services.ImportCursor)) (total, skipped int64)
	// stored lists the keys the phase has written to Postgres.
	stored func(t *testing.T) []string
	// clear deletes everything the phase has written.
	clear func(t *testing.T)
	// batches are the keys each committed batch writes, in commit order.
	batches [][]string
}

// requireResumesExactlyTheRest runs the phase once to the end, recording the
// cursor it heartbeats after each batch, then resumes from each of those
// cursors in turn. Each resume starts from an empty table, so a row it writes
// again shows up as well as a row it skips. It returns the recorded cursors.
func requireResumesExactlyTheRest(t *testing.T, c resumeCase) []services.ImportCursor {
	t.Helper()
	var cursors []services.ImportCursor
	total, skipped := c.run(t, services.ImportCursor{}, func(cur services.ImportCursor) { cursors = append(cursors, cur) })
	require.Equal(t, keysOf(c.batches), c.stored(t), "the uninterrupted run")
	require.Len(t, cursors, len(c.batches), "one heartbeat per committed batch")

	for k, from := range cursors {
		t.Run(fmt.Sprintf("after batch %d", k+1), func(t *testing.T) {
			c.clear(t)
			gotTotal, gotSkipped := c.run(t, from, func(services.ImportCursor) {})
			require.Equal(t, keysOf(c.batches[k+1:]), c.stored(t), "resumed from %+v", from)
			require.Equal(t, total, gotTotal, "a resumed run ends with the uninterrupted run's total")
			require.Equal(t, skipped, gotSkipped, "a resumed run ends with the uninterrupted run's skipped count")
		})
	}
	return cursors
}

func keysOf(batches [][]string) []string {
	keys := []string{}
	for _, b := range batches {
		keys = append(keys, b...)
	}
	sort.Strings(keys)
	return keys
}

func storedKeys(t *testing.T, db *gorm.DB, query string, args ...any) []string {
	t.Helper()
	keys := []string{}
	require.NoError(t, db.Raw(query, args...).Scan(&keys).Error)
	sort.Strings(keys)
	return keys
}

// dropForeignKeys lets a test seed rows into table without building the
// registry objects they reference; the behaviour under test is paging.
func dropForeignKeys(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	var fks []string
	require.NoError(t, db.Raw(`SELECT conname FROM pg_constraint WHERE conrelid = ?::regclass AND contype = 'f'`, table).Scan(&fks).Error)
	for _, fk := range fks {
		require.NoError(t, db.Exec(fmt.Sprintf(`ALTER TABLE %q DROP CONSTRAINT IF EXISTS %q`, table, fk)).Error)
	}
}

func resumeImporter(t *testing.T) *services.DirectDBImporter {
	t.Helper()
	importer := newImporter(t)
	importer.SetBatchSize(resumeBatchSize)
	return importer
}

// Contacts with an unmapped registrar are skipped, not written, and the
// workflow refuses to ingest domains if any were. A resumed run has to carry
// the earlier batches' skips forward or that gate passes when it should not.
func TestDirectDBImporter_ImportContacts_ResumesFromCursor(t *testing.T) {
	db := authInfoTestDB(t)
	dropForeignKeys(t, db, "contacts")

	staged := stagedSQLite(t)
	for _, c := range [][2]string{
		{"resume-c1", "escrow-rar"}, {"resume-c2", "unmapped"},
		{"resume-c3", "escrow-rar"}, {"resume-c4", "unmapped"},
		{"resume-c5", "escrow-rar"},
	} {
		_, err := staged.Exec(`INSERT INTO contacts (id, email, clid) VALUES (?, 'someone@example.invalid', ?)`, c[0], c[1])
		require.NoError(t, err)
	}
	clidMap := map[string]string{"escrow-rar": "rar1"}

	run := func(t *testing.T, from services.ImportCursor, hb func(services.ImportCursor)) (int64, int64) {
		total, _, _, skipped, err := resumeImporter(t).ImportContacts(context.Background(), staged, clidMap, from, hb)
		require.NoError(t, err)
		return total, skipped
	}
	stored := func(t *testing.T) []string { return storedKeys(t, db, `SELECT id FROM contacts WHERE cl_id = 'rar1'`) }
	clear := func(t *testing.T) { require.NoError(t, db.Exec(`DELETE FROM contacts WHERE cl_id = 'rar1'`).Error) }

	cursors := requireResumesExactlyTheRest(t, resumeCase{
		run: run, stored: stored, clear: clear,
		batches: [][]string{
			{"resume-c1"}, // resume-c2 skipped
			{"resume-c3"}, // resume-c4 skipped
			{"resume-c5"},
		},
	})
	require.Equal(t, services.ImportCursor{Version: services.ImportCursorVersion, LastKey: "resume-c2", Processed: 1, Total: 5, Skipped: 1}, cursors[0])

	// A legacy contacts cursor was already the staged id, so it resumes.
	t.Run("legacy cursor", func(t *testing.T) {
		clear(t)
		total, skipped := run(t, services.ImportCursor{LastKey: "resume-c2", Processed: 1, Total: 5, Skipped: 1}, func(services.ImportCursor) {})
		require.Equal(t, []string{"resume-c3", "resume-c5"}, stored(t))
		require.Equal(t, int64(3), total)
		require.Equal(t, int64(2), skipped)
	})
}

// Hosts page on (name, clid), so a name can straddle two batches. The cursor
// has to carry the clid or a resume re-reads the first half of the name.
func TestDirectDBImporter_ImportHosts_ResumesFromCursor(t *testing.T) {
	db := authInfoTestDB(t)
	dropForeignKeys(t, db, "hosts")

	// The staging the workflow builds keys hosts on name alone; the importer
	// pages on (name, clid) so that it does not depend on that.
	staged, err := sql.Open("sqlite", t.TempDir()+"/staged.db")
	require.NoError(t, err)
	t.Cleanup(func() { _ = staged.Close() })
	_, err = staged.Exec(`
		CREATE TABLE hosts (name TEXT, clid TEXT, crrr TEXT, uprr TEXT, PRIMARY KEY (name, clid));
		CREATE TABLE host_addresses (host_name TEXT, ip_address TEXT);
		CREATE TABLE host_statuses (host_name TEXT, status TEXT);
		INSERT INTO hosts (name, clid) VALUES
			('ns.a.example', 'rar1'), ('ns.b.example', 'rar1'),
			('ns.b.example', 'rar2'), ('ns.c.example', 'rar1'),
			('ns.d.example', 'rar1');`)
	require.NoError(t, err)

	run := func(t *testing.T, from services.ImportCursor, hb func(services.ImportCursor)) (int64, int64) {
		total, _, _, err := resumeImporter(t).ImportHosts(context.Background(), staged, nil, from, hb)
		require.NoError(t, err)
		return total, 0
	}
	stored := func(t *testing.T) []string { return storedKeys(t, db, `SELECT name || '|' || cl_id FROM hosts`) }
	clear := func(t *testing.T) { require.NoError(t, db.Exec(`DELETE FROM hosts`).Error) }

	cursors := requireResumesExactlyTheRest(t, resumeCase{
		run: run, stored: stored, clear: clear,
		batches: [][]string{
			{"ns.a.example|rar1", "ns.b.example|rar1"},
			{"ns.b.example|rar2", "ns.c.example|rar1"},
			{"ns.d.example|rar1"},
		},
	})
	require.Equal(t, services.ImportCursor{Version: services.ImportCursorVersion, LastKey: "ns.b.example", LastClID: "rar1", Processed: 2, Total: 5}, cursors[0])

	// A cursor heartbeated before it carried the clid does not say how much of
	// its name was committed, and its count already includes some of it.
	// Resuming at the start of the name would write and count those hosts
	// again; the import starts over with its counts reset instead.
	t.Run("legacy cursor without clid", func(t *testing.T) {
		clear(t)
		total, _ := run(t, services.ImportCursor{LastKey: "ns.b.example", Processed: 2, Total: 5}, func(services.ImportCursor) {})
		require.Equal(t, keysOf([][]string{{"ns.a.example|rar1", "ns.b.example|rar1", "ns.b.example|rar2", "ns.c.example|rar1", "ns.d.example|rar1"}}), stored(t))
		require.Equal(t, int64(5), total)
	})
}

// The domains cursor is the staged name the query compares, not the
// normalised name the domain is stored under. Staged names are not always
// lower-case, and "Z" sorts before "a": a cursor of "a2.resume" after the
// first batch would pass over "Z1.resume" and "Z2.resume" without importing
// them, resumed or not.
func TestDirectDBImporter_ImportDomains_ResumesFromCursor(t *testing.T) {
	db := domainsImportDB(t)

	staged := stagedDomains(t,
		[3]string{"A1.resume", "c1", "rar1"}, [3]string{"A2.resume", "c1", "rar1"},
		[3]string{"Z1.resume", "c1", "rar1"}, [3]string{"Z2.resume", "c1", "rar1"},
		[3]string{"a3.resume", "c1", "rar1"},
	)

	run := func(t *testing.T, from services.ImportCursor, hb func(services.ImportCursor)) (int64, int64) {
		total, _, _, err := resumeImporter(t).ImportDomains(context.Background(), staged, "resume", nil, from, hb)
		require.NoError(t, err)
		return total, 0
	}
	stored := func(t *testing.T) []string {
		return storedKeys(t, db, `SELECT name FROM domains WHERE tld_name = 'resume'`)
	}
	clear := func(t *testing.T) { require.NoError(t, db.Exec(`DELETE FROM domains WHERE tld_name = 'resume'`).Error) }

	cursors := requireResumesExactlyTheRest(t, resumeCase{
		run: run, stored: stored, clear: clear,
		batches: [][]string{
			{"a1.resume", "a2.resume"},
			{"z1.resume", "z2.resume"},
			{"a3.resume"},
		},
	})
	require.Equal(t, services.ImportCursor{Version: services.ImportCursorVersion, LastKey: "A2.resume", Processed: 2, Total: 5}, cursors[0])

	// What the old importer heartbeated after the first batch: the normalised
	// name. Resuming from it would pass over Z1 and Z2, and the staged boundary
	// cannot be recovered from it, so the import starts over.
	t.Run("legacy cursor", func(t *testing.T) {
		clear(t)
		total, _ := run(t, services.ImportCursor{LastKey: "a2.resume", Processed: 2, Total: 5}, func(services.ImportCursor) {})
		require.Equal(t, []string{"a1.resume", "a2.resume", "a3.resume", "z1.resume", "z2.resume"}, stored(t))
		require.Equal(t, int64(5), total)
	})
}

func TestDirectDBImporter_ImportNNDNs_ResumesFromCursor(t *testing.T) {
	db := authInfoTestDB(t)
	dropForeignKeys(t, db, "nndns")

	staged, err := sql.Open("sqlite", t.TempDir()+"/staged.db")
	require.NoError(t, err)
	t.Cleanup(func() { _ = staged.Close() })
	_, err = staged.Exec(`
		CREATE TABLE nndns (aname TEXT PRIMARY KEY, uname TEXT, idntableid TEXT, originalname TEXT, namestate TEXT, crdate TEXT);
		INSERT INTO nndns (aname, uname, namestate) VALUES
			('n1.resume', '', 'blocked'), ('n2.resume', '', 'blocked'),
			('n3.resume', '', 'blocked'), ('n4.resume', '', 'blocked'),
			('n5.resume', '', 'blocked');`)
	require.NoError(t, err)

	cursors := requireResumesExactlyTheRest(t, resumeCase{
		run: func(t *testing.T, from services.ImportCursor, hb func(services.ImportCursor)) (int64, int64) {
			total, _, _, err := resumeImporter(t).ImportNNDNs(context.Background(), staged, "resume", from, hb)
			require.NoError(t, err)
			return total, 0
		},
		stored: func(t *testing.T) []string {
			return storedKeys(t, db, `SELECT name FROM nndns WHERE tld_name = 'resume'`)
		},
		clear: func(t *testing.T) { require.NoError(t, db.Exec(`DELETE FROM nndns WHERE tld_name = 'resume'`).Error) },
		batches: [][]string{
			{"n1.resume", "n2.resume"},
			{"n3.resume", "n4.resume"},
			{"n5.resume"},
		},
	})
	require.Equal(t, services.ImportCursor{Version: services.ImportCursorVersion, LastKey: "n2.resume", Processed: 2, Total: 5}, cursors[0])
}
