package services_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

const dsTLD = "dsimport"

var (
	sha256Digest = strings.Repeat("ab", 32)
	sha1Digest   = strings.Repeat("cd", 20)
)

// stageDeposit builds a staging database the way the escrow import workflow
// does, from analysis CSVs through CSVToSQLiteService. dnssecCSV is the whole
// DNSSEC file, header included; "" leaves the file out.
func stageDeposit(t *testing.T, domains []string, dnssecCSV string) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	base := filepath.Join(dir, "escrow")

	lines := []string{"name,roid,uname,idntableid,originalname,registrant,clid,crrr,crdate,exdate,uprr,update"}
	for _, d := range domains {
		lines = append(lines, fmt.Sprintf("%s,R-%s,%s,,,c1,registrar1,,2020-01-01 00:00:00,2030-01-01 00:00:00,,", d, d, d))
	}
	require.NoError(t, os.WriteFile(base+"-domains.csv", []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	if dnssecCSV != "" {
		require.NoError(t, os.WriteFile(base+"-DomainDnssec.csv", []byte(dnssecCSV), 0o600))
	}

	dbPath := filepath.Join(dir, "staging.db")
	require.NoError(t, services.NewCSVToSQLiteService(base).ConvertToSQLite(dbPath, nil))
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func dnssecCSV(rows ...string) string {
	return strings.Join(append([]string{"DomainName,KeyTag,Algorithm,DigestType,Digest"}, rows...), "\n") + "\n"
}

type dsRow struct {
	DomainRoID int64
	KeyTag     int
	Algorithm  int
	DigestType int
	Digest     string
}

func dsRecordsOf(t *testing.T, db *gorm.DB, name string) []dsRow {
	t.Helper()
	var rows []dsRow
	require.NoError(t, db.Raw(`
		SELECT ds.domain_ro_id, ds.key_tag, ds.algorithm, ds.digest_type, ds.digest
		FROM domain_ds_records ds JOIN domains d ON d.ro_id = ds.domain_ro_id
		WHERE d.name = ? ORDER BY ds.id`, name).Scan(&rows).Error)
	return rows
}

func seedSignedDomain(t *testing.T, db *gorm.DB, roID int64, name string, ds ...dsRow) {
	t.Helper()
	require.NoError(t, db.Exec(`
		INSERT INTO domains (ro_id, name, tld_name, cl_id, registrant_id, auth_info, expiry_date, created_at, updated_at)
		VALUES (?, ?, ?, 'registrar1', 'c1', 'x', now(), now(), now())`, roID, name, dsTLD).Error)
	for _, r := range ds {
		require.NoError(t, db.Exec(`INSERT INTO domain_ds_records (domain_ro_id, key_tag, algorithm, digest_type, digest) VALUES (?, ?, ?, ?, ?)`,
			roID, r.KeyTag, r.Algorithm, r.DigestType, r.Digest).Error)
	}
}

func newImporter(t *testing.T) *services.DirectDBImporter {
	t.Helper()
	importer := testImporter(t)
	return importer
}

// A deposit's DS records reach domain_ds_records on the bulk path, as they do
// on the entity path. Every well-formed record is kept whatever its algorithm;
// a malformed one is skipped and reported without costing the domain or the
// batch.
func TestDirectDBImporter_ImportDomains_WritesDSRecords(t *testing.T) {
	db := domainsImportDB(t)
	importer := newImporter(t)

	const signed, unsigned = "signed." + dsTLD, "unsigned." + dsTLD
	staged := stageDeposit(t, []string{signed, unsigned}, dnssecCSV(
		signed+",12345,13,2,"+sha256Digest,                  // kept, digest upper-cased
		signed+",1,5,1,"+sha1Digest,                         // RSA/SHA-1: not for registrars to submit, but kept from a deposit
		signed+",2,253,2,"+sha256Digest,                     // private algorithm: kept
		signed+",x,13,2,"+sha256Digest,                      // key tag not a number
		signed+",70000,13,2,"+sha256Digest,                  // key tag out of range
		signed+",3,13,2,zz",                                 // digest not hex
		signed+",4,13,2,abcd",                               // wrong length for SHA-256
		signed+",5,0,2,"+sha256Digest,                       // algorithm 0 is reserved
		signed+",12345,13,2,"+strings.ToUpper(sha256Digest), // repeats the first
	))

	total, _, _, err := importer.ImportDomains(context.Background(), staged, dsTLD, nil, services.ImportCursor{}, func(services.ImportCursor) {})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)

	got := dsRecordsOf(t, db, signed)
	require.Len(t, got, 3, "%+v", got)
	upper := strings.ToUpper(sha256Digest)
	require.Equal(t, []dsRow{
		{got[0].DomainRoID, 12345, 13, 2, upper},
		{got[0].DomainRoID, 1, 5, 1, strings.ToUpper(sha1Digest)},
		{got[0].DomainRoID, 2, 253, 2, upper},
	}, got, "kept in deposit order")
	require.Empty(t, dsRecordsOf(t, db, unsigned))

	require.Len(t, importer.Report.SkippedDSRecords, 6)
	for _, s := range importer.Report.SkippedDSRecords {
		require.Equal(t, signed, s.Name)
		require.NotEmpty(t, s.Reason)
	}
	require.Contains(t, importer.Report.SkippedDSRecords[5].Reason, "duplicate")
}

// A re-import makes each deposited domain's DS records the deposit's: replaced
// where the deposit has others, cleared where it has none. They attach to the
// existing domain row, whose ro_id the upsert kept, and domains outside the
// deposit keep theirs. Importing the same deposit twice changes nothing.
func TestDirectDBImporter_ImportDomains_ReimportReplacesDSRecords(t *testing.T) {
	db := domainsImportDB(t)

	const rekeyed, unsignedNow, untouched = "rekeyed." + dsTLD, "unsigned-now." + dsTLD, "untouched." + dsTLD
	old := dsRow{KeyTag: 111, Algorithm: 8, DigestType: 2, Digest: strings.ToUpper(strings.Repeat("11", 32))}
	seedSignedDomain(t, db, 7101, rekeyed, old)
	seedSignedDomain(t, db, 7102, unsignedNow, old)
	seedSignedDomain(t, db, 7103, untouched, old)

	staged := stageDeposit(t, []string{rekeyed, unsignedNow}, dnssecCSV(rekeyed+",222,13,2,"+sha256Digest))

	for run := 1; run <= 2; run++ {
		_, _, _, err := newImporter(t).ImportDomains(context.Background(), staged, dsTLD, nil, services.ImportCursor{}, func(services.ImportCursor) {})
		require.NoError(t, err, "run %d", run)

		require.Equal(t, []dsRow{{7101, 222, 13, 2, strings.ToUpper(sha256Digest)}}, dsRecordsOf(t, db, rekeyed), "run %d: replaced, on the existing row", run)
		require.Empty(t, dsRecordsOf(t, db, unsignedNow), "run %d: cleared", run)
		require.Equal(t, []dsRow{{7103, old.KeyTag, old.Algorithm, old.DigestType, old.Digest}}, dsRecordsOf(t, db, untouched), "run %d: not in the deposit", run)
	}
}

// When the staging database holds no DS records at all (built before they were
// staged, or from the old placeholder format), the importer cannot tell a
// signed domain from an unsigned one, so it leaves DS records as they are.
func TestDirectDBImporter_ImportDomains_KeepsDSRecordsWhenNoneStaged(t *testing.T) {
	db := domainsImportDB(t)

	const name = "kept." + dsTLD
	existing := dsRow{KeyTag: 111, Algorithm: 13, DigestType: 2, Digest: strings.ToUpper(sha256Digest)}
	seedSignedDomain(t, db, 7201, name, existing)

	staged := stageDeposit(t, []string{name}, "DomainName,DnssecData\n"+name+",dnssec_data_present\n")
	_, _, _, err := newImporter(t).ImportDomains(context.Background(), staged, dsTLD, nil, services.ImportCursor{}, func(services.ImportCursor) {})
	require.NoError(t, err)

	require.Equal(t, []dsRow{{7201, existing.KeyTag, existing.Algorithm, existing.DigestType, existing.Digest}}, dsRecordsOf(t, db, name))
}

// Items are reported once their batch commits. A batch that rolls back is
// retried from the same cursor, so reporting it on the failed attempt too would
// list every item twice in the saved report.
func TestDirectDBImporter_ImportDomains_RolledBackBatchIsNotReported(t *testing.T) {
	domainsImportDB(t)
	importer := newImporter(t)

	const name = "rolledback." + dsTLD
	staged := stageDeposit(t, []string{name}, dnssecCSV(name+",1,13,2,zz"))
	_, err := staged.Exec(`UPDATE domains SET exdate = ''`) // a fallback expiry: a ModifiedDomains item
	require.NoError(t, err)

	// Filed under another TLD, the batch is refused and rolls back.
	_, _, _, err = importer.ImportDomains(context.Background(), staged, "otherds", nil, services.ImportCursor{}, func(services.ImportCursor) {})
	require.Error(t, err)
	require.Empty(t, importer.Report.ModifiedDomains)
	require.Empty(t, importer.Report.SkippedDSRecords)

	// The same batch, committed, is reported.
	_, _, _, err = importer.ImportDomains(context.Background(), staged, dsTLD, nil, services.ImportCursor{}, func(services.ImportCursor) {})
	require.NoError(t, err)
	require.Len(t, importer.Report.ModifiedDomains, 1)
	require.Len(t, importer.Report.SkippedDSRecords, 1)
}
