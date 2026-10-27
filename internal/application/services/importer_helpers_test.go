package services_test

import (
	"net/url"
	"os"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/stretchr/testify/require"
)

// testImportDSN is the connection string of the database the DB_* variables
// name, as authInfoTestDB sets them, with the defaults the local stack uses
// when they are unset.
func testImportDSN() string {
	or := func(v, def string) string {
		if v == "" {
			return def
		}
		return v
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(or(os.Getenv("DB_USER"), "postgres"), or(os.Getenv("DB_PASS"), "postgres")),
		Host:     or(os.Getenv("DB_HOST"), "localhost") + ":" + or(os.Getenv("DB_PORT"), "5432"),
		Path:     "/" + or(os.Getenv("DB_NAME"), "domain_os"),
		RawQuery: "sslmode=" + url.QueryEscape(or(os.Getenv("DB_SSLMODE"), "require")),
	}
	return u.String()
}

// testImportPool opens an import pool on testImportDSN, closed with the test.
// Nothing connects until the first query.
func testImportPool(t *testing.T) *services.ImportPool {
	t.Helper()
	pool, err := services.OpenImportPool(testImportDSN(), services.DefaultImportConns)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	return pool
}

// testImporter is an importer for repository TEST on a pool of its own.
func testImporter(t *testing.T) *services.DirectDBImporter {
	t.Helper()
	imp, err := services.NewDirectDBImporterWith("TEST", testImportPool(t))
	require.NoError(t, err)
	return imp
}
