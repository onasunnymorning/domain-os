package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTMCHRefreshInterval(t *testing.T) {
	d, err := parseTMCHRefreshInterval("")
	require.NoError(t, err)
	assert.Equal(t, DefaultTMCHRefreshInterval, d)
	d, err = parseTMCHRefreshInterval(" 12h ")
	require.NoError(t, err)
	assert.Equal(t, 12*time.Hour, d)
	d, err = parseTMCHRefreshInterval("10m")
	assert.Error(t, err, "below the minimum is reported")
	assert.Equal(t, MinTMCHRefreshInterval, d)
	d, err = parseTMCHRefreshInterval("soon")
	assert.Error(t, err)
	assert.Equal(t, DefaultTMCHRefreshInterval, d)
	d, err = parseTMCHRefreshInterval("-1h")
	assert.Error(t, err)
	assert.Equal(t, DefaultTMCHRefreshInterval, d)
}

func TestLoadTMCH(t *testing.T) {
	t.Setenv("TMCH_CA_URL", "")
	t.Setenv("TMCH_SMDRL_URL", "")
	t.Setenv("TMCH_DNL_URL", "")
	t.Setenv("TMCH_USERNAME", "")
	t.Setenv("TMCH_PASSWORD", "")
	t.Setenv("TMCH_REFRESH_INTERVAL", "")
	cfg, err := LoadTMCH()
	require.NoError(t, err)
	assert.Equal(t, DefaultTMCHCertificatesURL, cfg.CertificatesURL)
	assert.Equal(t, DefaultTMCHSMDRLURL, cfg.SMDRLURL)
	assert.Equal(t, DefaultTMCHDNLURL, cfg.DNLURL)
	assert.Equal(t, DefaultTMCHRefreshInterval, cfg.RefreshInterval)
	assert.False(t, cfg.Configured(), "no credentials, no feed")

	t.Setenv("TMCH_SMDRL_URL", "https://test.ry.marksdb.org/smdrl/smdrl-latest.csv")
	t.Setenv("TMCH_USERNAME", " registry ")
	t.Setenv("TMCH_PASSWORD", "test-feed-password") // #nosec G101 -- a test value for the loader, not a credential
	t.Setenv("TMCH_REFRESH_INTERVAL", "3h")
	cfg, err = LoadTMCH()
	require.NoError(t, err)
	assert.True(t, cfg.Configured())
	assert.Equal(t, "https://test.ry.marksdb.org/smdrl/smdrl-latest.csv", cfg.SMDRLURL)
	assert.Equal(t, "registry", cfg.Username)
	assert.Equal(t, 3*time.Hour, cfg.RefreshInterval)
}
