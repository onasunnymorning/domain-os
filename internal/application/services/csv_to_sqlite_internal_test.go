package services

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const contactsHeader = "id,roid,voice,fax,email,clid,crrr,crdate,uprr,update"

func writeCSVFile(t *testing.T, path, header string, rows ...string) {
	t.Helper()
	content := header + "\n"
	if len(rows) > 0 {
		content += strings.Join(rows, "\n") + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func contactRow(i int) string {
	return fmt.Sprintf("c%d,roid%d,+1.1,+1.2,c%d@example.test,clid1,crrr1,2020-01-01,uprr1,2020-01-02", i, i, i)
}

func countRows(t *testing.T, dbPath, table string) int {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	defer db.Close()

	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// TestImportCSVCommitsInBatches covers the batched-commit path: the loader has
// to close its statement, commit, and re-prepare against a fresh transaction
// partway through a file without losing rows.
func TestImportCSVCommitsInBatches(t *testing.T) {
	original := commitBatchSize
	commitBatchSize = 10
	t.Cleanup(func() { commitBatchSize = original })

	dir := t.TempDir()
	base := filepath.Join(dir, "escrow")

	const rowCount = 25
	rows := make([]string, 0, rowCount)
	for i := 0; i < rowCount; i++ {
		rows = append(rows, contactRow(i))
	}
	writeCSVFile(t, base+"-contacts.csv", contactsHeader, rows...)

	dbPath := filepath.Join(dir, "staging.db")
	if err := NewCSVToSQLiteService(base).ConvertToSQLite(dbPath, nil); err != nil {
		t.Fatalf("ConvertToSQLite: %v", err)
	}

	if got := countRows(t, dbPath, "contacts"); got != rowCount {
		t.Fatalf("contacts = %d, want %d", got, rowCount)
	}
}

// TestConvertToSQLiteHeartbeatsEveryImportPhase guards the regression that
// stalled BuildStagingDatabase: contact postal info and contact statuses were
// loaded without reporting any progress, so Temporal saw a long silence between
// the last "importing contacts" heartbeat and the first "importing domains" one
// and killed the activity.
func TestConvertToSQLiteHeartbeatsEveryImportPhase(t *testing.T) {
	original := heartbeatInterval
	heartbeatInterval = 0 // report on every row
	t.Cleanup(func() { heartbeatInterval = original })

	dir := t.TempDir()
	base := filepath.Join(dir, "escrow")

	writeCSVFile(t, base+"-contacts.csv", contactsHeader, contactRow(1), contactRow(2))
	writeCSVFile(t, base+"-contactPostalInfo.csv",
		"contact_id,type,name,org,street1,street2,street3,city,state_province,postal_code,country_code",
		"c1,int,Name,Org,S1,S2,S3,City,ST,1000,BE")
	writeCSVFile(t, base+"-contactStatuses.csv", "contact_id,status", "c1,ok")
	writeCSVFile(t, base+"-domains.csv",
		"name,roid,uname,idntableid,originalname,registrant,clid,crrr,crdate,exdate,uprr,update",
		"example.co,roid1,example.co,,,c1,clid1,crrr1,2020-01-01,2030-01-01,uprr1,2020-01-02")

	var mu sync.Mutex
	seen := map[string]bool{}
	heartbeat := func(details ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		if len(details) > 0 {
			if phase, ok := details[0].(string); ok {
				seen[phase] = true
			}
		}
	}

	dbPath := filepath.Join(dir, "staging.db")
	if err := NewCSVToSQLiteService(base).ConvertToSQLite(dbPath, heartbeat); err != nil {
		t.Fatalf("ConvertToSQLite: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, phase := range []string{
		"importing contacts",
		"importing contact postal info records",
		"importing contact statuses",
		"importing domains",
	} {
		if !seen[phase] {
			t.Errorf("no heartbeat for phase %q; phases seen: %v", phase, seen)
		}
	}
}

// TestImportCSVSkipsMalformedRows checks that a short row is skipped rather than
// aborting the file. The csv reader enforces the header's field count by
// default, which turned one ragged row into a failure of the whole import.
func TestImportCSVSkipsMalformedRows(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "escrow")

	writeCSVFile(t, base+"-contacts.csv", contactsHeader,
		contactRow(1),
		"c2,roid2,truncated",
		contactRow(3),
	)

	dbPath := filepath.Join(dir, "staging.db")
	if err := NewCSVToSQLiteService(base).ConvertToSQLite(dbPath, nil); err != nil {
		t.Fatalf("ConvertToSQLite: %v", err)
	}

	if got := countRows(t, dbPath, "contacts"); got != 2 {
		t.Fatalf("contacts = %d, want 2 (the well-formed rows either side of the short one)", got)
	}
}
