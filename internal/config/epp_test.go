package config

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eppEnv is every variable LoadEPP reads. Tests clear them all so the
// developer's own environment cannot leak in.
var eppEnv = []string{
	"EPP_PORT", "EPP_OPS_PORT", "LOG_LEVEL", "REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD",
	"EPP_TLS_CERT_FILE", "EPP_TLS_KEY_FILE", "EPP_TLS_SELF_SIGNED", "EPP_TLS_RELOAD_INTERVAL",
	"EPP_PROXY_PROTOCOL", "EPP_PROXY_TRUSTED_CIDRS",
	"EPP_MAX_CONNECTIONS", "EPP_MAX_HANDSHAKES", "EPP_MAX_FRAME_BYTES", "EPP_XML_MAX_DEPTH", "EPP_XML_MAX_ELEMENTS",
	"EPP_HANDSHAKE_TIMEOUT", "EPP_LOGIN_TIMEOUT", "EPP_IDLE_TIMEOUT", "EPP_READ_TIMEOUT", "EPP_WRITE_TIMEOUT",
	"EPP_MAX_SESSION_DURATION", "EPP_DRAIN_TIMEOUT",
}

func clearEPPEnv(t *testing.T) {
	t.Helper()
	for _, name := range eppEnv {
		t.Setenv(name, "")
	}
}

func TestLoadEPPDefaults(t *testing.T) {
	clearEPPEnv(t)
	t.Setenv("EPP_TLS_CERT_FILE", "/certs/epp.crt")
	t.Setenv("EPP_TLS_KEY_FILE", "/certs/epp.key")

	cfg, err := LoadEPP()
	require.NoError(t, err)

	want := eppDefaults
	want.TLS.CertFile = "/certs/epp.crt"
	want.TLS.KeyFile = "/certs/epp.key"
	assert.Equal(t, want, cfg)

	// The documented defaults (docs/epp/POLICY_REFERENCE.md).
	assert.Equal(t, 700, cfg.Port)
	assert.Equal(t, 9700, cfg.OpsPort)
	assert.Equal(t, uint32(65536), cfg.Limits.MaxFrameBytes)
	assert.Equal(t, 2000, cfg.Limits.MaxConnections)
	assert.Equal(t, 200, cfg.Limits.MaxHandshakes)
	assert.Equal(t, 32, cfg.Limits.XMLMaxDepth)
	assert.Equal(t, 2000, cfg.Limits.XMLMaxElements)
	assert.Equal(t, EPPTimeouts{
		Handshake: 10 * time.Second, Login: 30 * time.Second, Idle: 10 * time.Minute,
		Read: 30 * time.Second, Write: 60 * time.Second, MaxSession: 24 * time.Hour, Drain: 30 * time.Second,
	}, cfg.Timeouts)
	assert.Equal(t, "localhost:6379", cfg.Redis.Addr())
}

func TestLoadEPPOverrides(t *testing.T) {
	clearEPPEnv(t)
	set := map[string]string{
		"EPP_PORT": "7700", "EPP_OPS_PORT": "9800", "LOG_LEVEL": "WARNING",
		"REDIS_HOST": "redis", "REDIS_PORT": "6380", "REDIS_PASSWORD": "s3cret",
		"EPP_TLS_SELF_SIGNED": "true", "EPP_TLS_RELOAD_INTERVAL": "5m",
		"EPP_PROXY_PROTOCOL": "true", "EPP_PROXY_TRUSTED_CIDRS": "10.0.0.0/8, 192.0.2.10",
		"EPP_MAX_CONNECTIONS": "50", "EPP_MAX_HANDSHAKES": "5", "EPP_MAX_FRAME_BYTES": "131072",
		"EPP_XML_MAX_DEPTH": "16", "EPP_XML_MAX_ELEMENTS": "500",
		"EPP_HANDSHAKE_TIMEOUT": "5s", "EPP_LOGIN_TIMEOUT": "15s", "EPP_IDLE_TIMEOUT": "20m",
		"EPP_READ_TIMEOUT": "10s", "EPP_WRITE_TIMEOUT": "15s", "EPP_MAX_SESSION_DURATION": "12h", "EPP_DRAIN_TIMEOUT": "1m",
	}
	for k, v := range set {
		t.Setenv(k, v)
	}

	cfg, err := LoadEPP()
	require.NoError(t, err)
	assert.Equal(t, EPP{
		Port:     7700,
		OpsPort:  9800,
		LogLevel: slog.LevelWarn,
		Redis:    EPPRedis{Host: "redis", Port: "6380", Password: "s3cret"},
		TLS:      EPPTLS{SelfSigned: true, ReloadInterval: 5 * time.Minute},
		Proxy:    EPPProxy{Enabled: true, TrustedCIDRs: []string{"10.0.0.0/8", "192.0.2.10"}},
		Limits:   EPPLimits{MaxConnections: 50, MaxHandshakes: 5, MaxFrameBytes: 131072, XMLMaxDepth: 16, XMLMaxElements: 500},
		Timeouts: EPPTimeouts{
			Handshake: 5 * time.Second, Login: 15 * time.Second, Idle: 20 * time.Minute,
			Read: 10 * time.Second, Write: 15 * time.Second, MaxSession: 12 * time.Hour, Drain: time.Minute,
		},
	}, cfg)
}

func TestLoadEPPRejects(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"unparseable port", map[string]string{"EPP_PORT": "seven hundred"}, "EPP_PORT"},
		{"port out of range", map[string]string{"EPP_OPS_PORT": "65536"}, "EPP_OPS_PORT"},
		{"unknown log level", map[string]string{"LOG_LEVEL": "verbose"}, "LOG_LEVEL"},
		{"no certificate", map[string]string{"EPP_TLS_SELF_SIGNED": ""}, "EPP_TLS_CERT_FILE"},
		{"half a certificate", map[string]string{"EPP_TLS_SELF_SIGNED": "", "EPP_TLS_CERT_FILE": "/c.crt"}, "EPP_TLS_KEY_FILE"},
		{"certificate and self-signed", map[string]string{"EPP_TLS_CERT_FILE": "/c.crt", "EPP_TLS_KEY_FILE": "/c.key"}, "conflicts"},
		{"proxy without trusted CIDRs", map[string]string{"EPP_PROXY_PROTOCOL": "true"}, "EPP_PROXY_TRUSTED_CIDRS"},
		{"bad CIDR", map[string]string{"EPP_PROXY_TRUSTED_CIDRS": "10.0.0.0/33"}, "EPP_PROXY_TRUSTED_CIDRS"},
		{"zero connections", map[string]string{"EPP_MAX_CONNECTIONS": "0"}, "EPP_MAX_CONNECTIONS"},
		{"handshakes above connections", map[string]string{"EPP_MAX_CONNECTIONS": "10", "EPP_MAX_HANDSHAKES": "11"}, "EPP_MAX_HANDSHAKES"},
		{"frame below floor", map[string]string{"EPP_MAX_FRAME_BYTES": "1000"}, "EPP_MAX_FRAME_BYTES"},
		{"duration without unit", map[string]string{"EPP_IDLE_TIMEOUT": "600"}, "EPP_IDLE_TIMEOUT"},
		{"negative duration", map[string]string{"EPP_DRAIN_TIMEOUT": "-1s"}, "EPP_DRAIN_TIMEOUT"},
		{"same port twice", map[string]string{"EPP_PORT": "9700"}, "EPP_OPS_PORT"},
		{"non-boolean flag", map[string]string{"EPP_PROXY_PROTOCOL": "yes please"}, "EPP_PROXY_PROTOCOL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearEPPEnv(t)
			t.Setenv("EPP_TLS_SELF_SIGNED", "true")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := LoadEPP()
			require.Error(t, err, "a set but unusable value stops the server instead of falling back silently")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestLoadEPPReportsEveryProblem(t *testing.T) {
	clearEPPEnv(t)
	t.Setenv("EPP_TLS_SELF_SIGNED", "true")
	t.Setenv("EPP_PORT", "x")
	t.Setenv("EPP_LOGIN_TIMEOUT", "y")
	_, err := LoadEPP()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EPP_PORT")
	assert.Contains(t, err.Error(), "EPP_LOGIN_TIMEOUT", "one start-up attempt lists every bad variable")
}
