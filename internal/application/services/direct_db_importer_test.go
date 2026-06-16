package services_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestDirectDBImporter_ImportNNDNs_NoTable(t *testing.T) {
	// Create local mock sqlite db without nndns table
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test_staged_no_nndns.db"
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Initialize importer (doesn't hit PG until an insert occurs)
	importer, err := services.NewDirectDBImporter()
	// If PG connect fails because it's not a real environment, skip the test
	if err != nil || importer == nil {
		t.Skip("Postgres environment not available, skipping DirectDBImporter test")
	}

	// Test graceful handle or error
	// Since we removed graceful handle, it should ERROR!
	_, _, err = importer.ImportNNDNs(context.Background(), db, "radio", "", func(processed string) {})
	require.Error(t, err, "Expected error when nndns table does not exist")
	require.Contains(t, err.Error(), "no such table: nndns")
}

func TestDirectDBImporter_ImportNNDNs_EmptyTable(t *testing.T) {
	// Create local mock sqlite db
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test_staged.db"
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Pre-create table
	_, err = db.Exec("CREATE TABLE nndns (aname TEXT PRIMARY KEY, uname TEXT, idntableid TEXT, originalname TEXT, namestate TEXT, crdate TEXT);")
	require.NoError(t, err)

	// Initialize importer
	importer, err := services.NewDirectDBImporter()
	if err != nil || importer == nil {
		t.Skip("Postgres environment not available, skipping DirectDBImporter test")
	}

	// This should run without errors since it has 0 items and no conflicts
	total, skipped, err := importer.ImportNNDNs(context.Background(), db, "radio", "", func(processed string) {})
	require.NoError(t, err, "Should not error on empty correctly-structured table")
	require.Equal(t, int64(0), total)
	require.Equal(t, int64(0), skipped)
}
