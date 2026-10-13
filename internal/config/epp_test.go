package config

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEPP(t *testing.T) {
	for _, name := range []string{"EPP_PORT", "LOG_LEVEL", "REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD"} {
		t.Setenv(name, "")
	}
	cfg, err := LoadEPP()
	require.NoError(t, err)
	assert.Equal(t, EPP{
		Port:     DefaultEPPPort,
		LogLevel: slog.LevelInfo,
		Redis:    EPPRedis{Host: "localhost", Port: "6379"},
	}, cfg, "everything has a default")
	assert.Equal(t, "localhost:6379", cfg.Redis.Addr())

	t.Setenv("EPP_PORT", "7700")
	t.Setenv("LOG_LEVEL", "WARNING")
	t.Setenv("REDIS_HOST", "redis")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "s3cret")
	cfg, err = LoadEPP()
	require.NoError(t, err)
	assert.Equal(t, EPP{
		Port:     7700,
		LogLevel: slog.LevelWarn,
		Redis:    EPPRedis{Host: "redis", Port: "6380", Password: "s3cret"},
	}, cfg)
}

func TestLoadEPPRejectsUnusableValues(t *testing.T) {
	cases := map[string]string{
		"EPP_PORT":  "seven hundred",
		"LOG_LEVEL": "verbose",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("EPP_PORT", "")
			t.Setenv("LOG_LEVEL", "")
			t.Setenv(name, raw)
			_, err := LoadEPP()
			require.Error(t, err, "a set but unusable value stops the server instead of falling back silently")
			assert.Contains(t, err.Error(), name)
		})
	}
}

func TestParseEPPPortRange(t *testing.T) {
	for _, raw := range []string{"0", "65536", "-1"} {
		_, err := parseEPPPort(raw)
		assert.Error(t, err, raw)
	}
	port, err := parseEPPPort(" 65535 ")
	require.NoError(t, err)
	assert.Equal(t, 65535, port)
}
