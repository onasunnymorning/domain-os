package services

import (
	"testing"

	"github.com/go-pg/pg/v10"
	"github.com/stretchr/testify/require"
)

// RDS- and Secrets Manager-generated passwords routinely contain URL-reserved
// characters. Interpolating them raw into a postgres:// URL makes pg.ParseURL
// misread the URL (e.g. "database name not provided"), so the fallback path
// must percent-escape credentials.
func TestFallbackPGURL_EscapesReservedCharacters(t *testing.T) {
	const nastyPass = `p?a/s#s@w:o&rd%25`

	t.Setenv("DB_USER", "escrow_user")
	t.Setenv("DB_PASS", nastyPass)
	t.Setenv("DB_HOST", "db.example.internal")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_NAME", "domain_os")
	t.Setenv("DB_SSLMODE", "require")

	opt, err := pg.ParseURL(fallbackPGURL())
	require.NoError(t, err)
	require.Equal(t, "escrow_user", opt.User)
	require.Equal(t, nastyPass, opt.Password)
	require.Equal(t, "db.example.internal:5433", opt.Addr)
	require.Equal(t, "domain_os", opt.Database)
	require.NotNil(t, opt.TLSConfig, "sslmode=require should enable TLS")
}

func TestFallbackPGURL_Defaults(t *testing.T) {
	for _, v := range []string{"DB_USER", "DB_PASS", "DB_HOST", "DB_PORT", "DB_NAME", "DB_SSLMODE"} {
		t.Setenv(v, "")
	}

	opt, err := pg.ParseURL(fallbackPGURL())
	require.NoError(t, err)
	require.Equal(t, "postgres", opt.User)
	require.Equal(t, "postgres", opt.Password)
	require.Equal(t, "localhost:5432", opt.Addr)
	require.Equal(t, "domain_os", opt.Database)
	require.Nil(t, opt.TLSConfig, "sslmode=disable should not enable TLS")
}
