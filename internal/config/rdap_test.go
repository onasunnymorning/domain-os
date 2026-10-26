package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rdapEnv is every variable LoadRDAP reads. Tests clear them all so the
// developer's own environment cannot leak in.
var rdapEnv = []string{
	"RDAP_PORT", "RDAP_OPS_PORT", "LOG_LEVEL",
	"RDAP_TLS_CERT_PATH", "RDAP_TLS_KEY_PATH", "RDAP_TLS_SELF_SIGNED", "RDAP_TLS_RELOAD_INTERVAL", "RDAP_SERVER_NAME",
	"RDAP_PROXY_PROTOCOL", "RDAP_PROXY_TRUSTED_CIDRS",
	"RDAP_READ_TIMEOUT", "RDAP_WRITE_TIMEOUT", "RDAP_IDLE_TIMEOUT", "RDAP_DRAIN_TIMEOUT",
	"RDAP_BASE_URL", "RDAP_TERMS_OF_SERVICE_URL", "RDAP_DB_MAX_OPEN_CONNS",
	"DB_USER", "DB_PASS", "DB_HOST", "DB_PORT", "DB_NAME", "DB_SSLMODE",
}

func clearRDAPEnv(t *testing.T) {
	t.Helper()
	for _, name := range rdapEnv {
		t.Setenv(name, "")
	}
}

func TestLoadRDAPDefaults(t *testing.T) {
	clearRDAPEnv(t)
	t.Setenv("RDAP_TLS_CERT_PATH", "/certs/rdap.crt")
	t.Setenv("RDAP_TLS_KEY_PATH", "/certs/rdap.key")
	t.Setenv("RDAP_SERVER_NAME", "RDAP.Example.")

	cfg, err := LoadRDAP()
	require.NoError(t, err)

	// The documented defaults (docs/rdap/POLICY_REFERENCE.md).
	assert.Equal(t, 443, cfg.Port)
	assert.Equal(t, 9701, cfg.OpsPort)
	assert.Equal(t, "rdap.example", cfg.TLS.ServerName)
	assert.Equal(t, "https://rdap.example/", cfg.BaseURL, "derived from the server name")
	assert.Equal(t, time.Minute, cfg.TLS.ReloadInterval)
	assert.Equal(t, RDAPTimeouts{Read: 5 * time.Second, Write: 10 * time.Second, Idle: time.Minute, Drain: 20 * time.Second}, cfg.Timeouts)
	assert.Equal(t, 20, cfg.DB.MaxOpenConns)
	assert.Empty(t, cfg.Notices.TermsOfServiceURL)
}

func TestLoadRDAPBaseURL(t *testing.T) {
	for _, tc := range []struct{ raw, want, err string }{
		{raw: "https://rdap.example/rdap", want: "https://rdap.example/rdap/"},
		{raw: "https://rdap.example/", want: "https://rdap.example/"},
		{raw: "http://rdap.example/", err: "not an absolute https URL"},
		{raw: "https://rdap.example/?a=b", err: "not an absolute https URL"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			clearRDAPEnv(t)
			t.Setenv("RDAP_TLS_SELF_SIGNED", "true")
			t.Setenv("RDAP_BASE_URL", tc.raw)
			cfg, err := LoadRDAP()
			if tc.err != "" {
				assert.ErrorContains(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.BaseURL)
		})
	}
}

func TestLoadRDAPRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		err  string
	}{
		{"no certificate", map[string]string{"RDAP_SERVER_NAME": "rdap.example"}, "RDAP_TLS_CERT_PATH and RDAP_TLS_KEY_PATH are both required"},
		{"self-signed and files", map[string]string{"RDAP_SERVER_NAME": "rdap.example", "RDAP_TLS_SELF_SIGNED": "true", "RDAP_TLS_CERT_PATH": "/c", "RDAP_TLS_KEY_PATH": "/k"}, "conflicts"},
		{"no base URL or server name", map[string]string{"RDAP_TLS_SELF_SIGNED": "true"}, "RDAP_BASE_URL is unset"},
		{"proxy without trusted CIDRs", map[string]string{"RDAP_SERVER_NAME": "rdap.example", "RDAP_TLS_SELF_SIGNED": "true", "RDAP_PROXY_PROTOCOL": "true"}, "RDAP_PROXY_TRUSTED_CIDRS"},
		{"same ports", map[string]string{"RDAP_SERVER_NAME": "rdap.example", "RDAP_TLS_SELF_SIGNED": "true", "RDAP_PORT": "9701"}, "both 9701"},
		{"bad timeout", map[string]string{"RDAP_SERVER_NAME": "rdap.example", "RDAP_TLS_SELF_SIGNED": "true", "RDAP_READ_TIMEOUT": "soon"}, "RDAP_READ_TIMEOUT"},
		{"insecure terms URL", map[string]string{"RDAP_SERVER_NAME": "rdap.example", "RDAP_TLS_SELF_SIGNED": "true", "RDAP_TERMS_OF_SERVICE_URL": "http://x.example/tos"}, "RDAP_TERMS_OF_SERVICE_URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearRDAPEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := LoadRDAP()
			assert.ErrorContains(t, err, tc.err)
		})
	}
}
