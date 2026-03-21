package services_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestCSVToSQLiteService_NNDNs(t *testing.T) {
	// Initialize the service with a non-existent base filename to skip actual inserts
	svc := services.NewCSVToSQLiteService("testbase")
	
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test_staged.db"
	defer os.Remove(dbPath)

	// Suppress heartbeats
	err := svc.ConvertToSQLite(dbPath, func(details ...interface{}) {})
	require.NoError(t, err)

	// Verify the schema includes nndns
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	// If the table doesn't exist, this will return an error
	err = db.QueryRow("SELECT COUNT(*) FROM nndns").Scan(&count)
	require.NoError(t, err, "nndns table should exist in the generated SQLite schema")
}
