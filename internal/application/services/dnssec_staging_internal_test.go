package services

import (
	"database/sql"
	"encoding/csv"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const domainsCSVHeader = "name,roid,uname,idntableid,originalname,registrant,clid,crrr,crdate,exdate,uprr,update"

// The streaming analyzer feeds the staging database in the escrow import
// workflow. It used to write one placeholder row per signed domain instead of
// its DS records, so the bulk import had nothing to carry.
func TestStreamingAnalyzerWritesDSRecords(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewStreamingXMLEscrowServiceFromReader(strings.NewReader(""), dir, "escrow", 0)
	if err != nil {
		t.Fatalf("new streaming service: %v", err)
	}

	const deposit = `<rdeDomain:domain xmlns:rdeDomain="urn:ietf:params:xml:ns:rdeDomain-1.0" xmlns:secDNS="urn:ietf:params:xml:ns:secDNS-1.1">
		<rdeDomain:name>signed.example</rdeDomain:name>
		<rdeDomain:secDNS>
			<secDNS:dsData><secDNS:keyTag>12345</secDNS:keyTag><secDNS:alg>13</secDNS:alg><secDNS:digestType>2</secDNS:digestType><secDNS:digest>49fd46e6</secDNS:digest></secDNS:dsData>
			<secDNS:dsData><secDNS:keyTag>54321</secDNS:keyTag><secDNS:alg>8</secDNS:alg><secDNS:digestType>1</secDNS:digestType><secDNS:digest>AB12</secDNS:digest></secDNS:dsData>
		</rdeDomain:secDNS>
	</rdeDomain:domain>`
	dec := xml.NewDecoder(strings.NewReader(deposit))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("read start element: %v", err)
	}
	if err := svc.processDomainTag(dec, tok.(xml.StartElement)); err != nil {
		t.Fatalf("processDomainTag: %v", err)
	}
	if err := svc.closeCSVWriters(); err != nil {
		t.Fatalf("closeCSVWriters: %v", err)
	}

	f, err := os.Open(filepath.Join(dir, "escrow-DomainDnssec.csv"))
	if err != nil {
		t.Fatalf("open DNSSEC csv: %v", err)
	}
	defer f.Close()
	got, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read DNSSEC csv: %v", err)
	}
	want := [][]string{
		{"DomainName", "KeyTag", "Algorithm", "DigestType", "Digest"},
		{"signed.example", "12345", "13", "2", "49fd46e6"}, // as deposited: the importer validates and normalises
		{"signed.example", "54321", "8", "1", "AB12"},
	}
	if len(got) != len(want) {
		t.Fatalf("DNSSEC csv = %q, want %q", got, want)
	}
	for i := range want {
		if strings.Join(got[i], ",") != strings.Join(want[i], ",") {
			t.Errorf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func stagedDSRows(t *testing.T, dbPath string) [][5]string {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT domain_name, key_tag, algorithm, digest_type, digest FROM domain_ds_records ORDER BY id`)
	if err != nil {
		t.Fatalf("query staged DS records: %v", err)
	}
	defer rows.Close()
	var out [][5]string
	for rows.Next() {
		var r [5]string
		if err := rows.Scan(&r[0], &r[1], &r[2], &r[3], &r[4]); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// DS records are staged as text, malformed ones included, so that the importer
// can report each one it skips rather than have it vanish at this stage.
func TestConvertToSQLiteStagesDSRecords(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "escrow")
	writeCSVFile(t, base+"-domains.csv", domainsCSVHeader,
		"Signed.Example,roid1,signed.example,,,c1,clid1,,2020-01-01,2030-01-01,,")
	writeCSVFile(t, base+"-DomainDnssec.csv", "DomainName,KeyTag,Algorithm,DigestType,Digest",
		"Signed.Example,12345,13,2,49fd46e6",
		"signed.example,not-a-number,13,2,AB",
		"signed.example,1,13", // short: skipped here, as every phase does
	)

	dbPath := filepath.Join(dir, "staging.db")
	if err := NewCSVToSQLiteService(base).ConvertToSQLite(dbPath, nil); err != nil {
		t.Fatalf("ConvertToSQLite: %v", err)
	}

	got := stagedDSRows(t, dbPath)
	want := [][5]string{
		{"signed.example", "12345", "13", "2", "49fd46e6"}, // name lower-cased to join the domains table
		{"signed.example", "not-a-number", "13", "2", "AB"},
	}
	if len(got) != len(want) {
		t.Fatalf("staged DS records = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// A deposit with no signed domains still stages the table, empty: that is what
// tells the importer to clear DS records rather than leave them.
func TestConvertToSQLiteStagesEmptyDSRecordsTable(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "escrow")
	writeCSVFile(t, base+"-DomainDnssec.csv", "DomainName,KeyTag,Algorithm,DigestType,Digest")

	dbPath := filepath.Join(dir, "staging.db")
	if err := NewCSVToSQLiteService(base).ConvertToSQLite(dbPath, nil); err != nil {
		t.Fatalf("ConvertToSQLite: %v", err)
	}
	if got := countRows(t, dbPath, stagedDSRecordsTable); got != 0 {
		t.Fatalf("staged DS records = %d, want 0", got)
	}
}

// The DS table is left out when the analysis did not carry DS records: no
// DNSSEC file, or the old placeholder format. An empty table there would read
// as "nothing is signed" and the import would clear DS records it never saw.
func TestConvertToSQLiteOmitsDSRecordsTableWhenNotStaged(t *testing.T) {
	for name, write := range map[string]func(base string){
		"no DNSSEC file": func(string) {},
		"placeholder format": func(base string) {
			writeCSVFile(t, base+"-DomainDnssec.csv", "DomainName,DnssecData", "signed.example,dnssec_data_present")
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			base := filepath.Join(dir, "escrow")
			write(base)

			dbPath := filepath.Join(dir, "staging.db")
			if err := NewCSVToSQLiteService(base).ConvertToSQLite(dbPath, nil); err != nil {
				t.Fatalf("ConvertToSQLite: %v", err)
			}

			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer db.Close()
			exists, err := stagedTableExists(db, stagedDSRecordsTable)
			if err != nil {
				t.Fatalf("stagedTableExists: %v", err)
			}
			if exists {
				t.Fatalf("%s exists; it must be absent when DS records were not staged", stagedDSRecordsTable)
			}
		})
	}
}
