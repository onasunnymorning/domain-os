package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var databaseEnv = []string{"DATABASE_URL", "DB_USER", "DB_PASS", "DB_HOST", "DB_PORT", "DB_NAME", "DB_SSLMODE", "DB_MAX_OPEN_CONNS"}

func clearDatabaseEnv(t *testing.T) {
	t.Helper()
	for _, name := range databaseEnv {
		t.Setenv(name, "")
	}
}

func TestLoadDatabaseDefaults(t *testing.T) {
	clearDatabaseEnv(t)
	db, err := LoadDatabase()
	require.NoError(t, err)
	assert.Equal(t, Database{
		User: "postgres", Pass: "postgres", Host: "localhost", Port: "5432", DBName: "domain_os", SSLmode: "disable",
		MaxOpenConns: DefaultDBMaxOpenConns,
	}, db)
}

func TestLoadDatabaseReadsEveryVariable(t *testing.T) {
	clearDatabaseEnv(t)
	const url = "postgres://u:p@db.example:5433/dos?sslmode=require" // #nosec G101 -- not a credential: a fixture URL, to be read back verbatim
	for name, value := range map[string]string{
		"DATABASE_URL": url,
		"DB_USER":      "u", "DB_PASS": "p", "DB_HOST": "db.example", "DB_PORT": "5433", "DB_NAME": "dos", "DB_SSLMODE": "require",
		"DB_MAX_OPEN_CONNS": "7",
	} {
		t.Setenv(name, value)
	}
	db, err := LoadDatabase()
	require.NoError(t, err)
	assert.Equal(t, Database{
		URL:  url,
		User: "u", Pass: "p", Host: "db.example", Port: "5433", DBName: "dos", SSLmode: "require",
		MaxOpenConns: 7,
	}, db)
}

// A pool of one hangs the migrator: Migrate holds a connection for its
// advisory lock while AutoMigrate asks the same pool for another.
func TestLoadDatabaseRefusesABadPoolBound(t *testing.T) {
	for _, raw := range []string{"1", "0", "-3", "many"} {
		t.Run(raw, func(t *testing.T) {
			clearDatabaseEnv(t)
			t.Setenv("DB_MAX_OPEN_CONNS", raw)
			_, err := LoadDatabase()
			require.ErrorContains(t, err, "DB_MAX_OPEN_CONNS")
		})
	}
	clearDatabaseEnv(t)
	t.Setenv("DB_MAX_OPEN_CONNS", "2")
	db, err := LoadDatabase()
	require.NoError(t, err)
	assert.Equal(t, 2, db.MaxOpenConns)
}
