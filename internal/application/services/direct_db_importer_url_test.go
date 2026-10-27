package services

import (
	"net/url"
	"testing"

	"github.com/go-pg/pg/v10"
	"github.com/stretchr/testify/require"
)

// The connection string reaches the importer from config.Database.DSN, which
// percent-escapes credentials; RDS- and Secrets Manager-generated passwords
// routinely contain URL-reserved characters. go-pg must read them back
// intact, and must not choke on the parameters managed providers add to
// DATABASE_URL (Neon's channel_binding), which sanitizePGURLForGoPG strips.
func TestImportPoolURL_KeepsEscapedCredentialsAndDropsUnsupportedParams(t *testing.T) {
	const nastyPass = `p?a/s#s@w:o&rd%25` // #nosec G101 -- not a credential: a fixture of URL-reserved characters, which is the whole subject of this test
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword("escrow_user", nastyPass),
		Host:     "db.example.internal:5433",
		Path:     "/domain_os",
		RawQuery: "sslmode=require&channel_binding=require",
	}

	opt, err := pg.ParseURL(sanitizePGURLForGoPG(u.String()))
	require.NoError(t, err)
	require.Equal(t, "escrow_user", opt.User)
	require.Equal(t, nastyPass, opt.Password)
	require.Equal(t, "db.example.internal:5433", opt.Addr)
	require.Equal(t, "domain_os", opt.Database)
	require.NotNil(t, opt.TLSConfig, "sslmode=require should enable TLS")

	plain := "postgres://postgres:postgres@localhost:5432/domain_os?sslmode=disable" // #nosec G101 -- not a credential: the local stack's default fixture
	opt, err = pg.ParseURL(sanitizePGURLForGoPG(plain))
	require.NoError(t, err)
	require.Nil(t, opt.TLSConfig, "sslmode=disable should not enable TLS")
}
