package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEscrowIntake(t *testing.T) {
	t.Setenv("ESCROW_INTAKE_SFTP_ENABLED", "")
	t.Setenv("ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT", "")
	t.Setenv("ESCROW_INTAKE_SFTP_PREFIX", "")
	cfg, err := LoadEscrowIntake()
	require.NoError(t, err)
	assert.Equal(t, EscrowIntake{}, cfg, "intake and plaintext are both off unless switched on")

	t.Setenv("ESCROW_INTAKE_SFTP_ENABLED", "true")
	t.Setenv("ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT", "true")
	t.Setenv("ESCROW_INTAKE_SFTP_PREFIX", "intake/")
	cfg, err = LoadEscrowIntake()
	require.NoError(t, err)
	assert.Equal(t, EscrowIntake{Enabled: true, AllowPlaintext: true, Prefix: "intake/"}, cfg)

	t.Setenv("ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT", "yes please")
	_, err = LoadEscrowIntake()
	require.Error(t, err, "a value that is not a boolean is a configuration error, not 'off'")

	t.Setenv("ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT", "")
	t.Setenv("ESCROW_INTAKE_SFTP_ENABLED", "maybe")
	_, err = LoadEscrowIntake()
	require.Error(t, err)
}

func TestParseEscrowIntakeSweepInterval(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		want    time.Duration
		wantErr bool
	}{
		{"", 2 * time.Minute, false},
		{"5m", 5 * time.Minute, false},
		{" 90s ", 90 * time.Second, false},
		{"10s", 30 * time.Second, true},  // raised to the minimum
		{"soon", 2 * time.Minute, true},  // unparseable: the default
		{"-1m", 2 * time.Minute, true},   // not positive: the default
		{"0s", 2 * time.Minute, true},    // not positive: the default
		{"30s", 30 * time.Second, false}, // the minimum itself
		{"24h", 24 * time.Hour, false},   // no upper bound
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := parseEscrowIntakeSweepInterval(tc.raw)
			assert.Equal(t, tc.want, got, "a usable interval is returned either way")
			assert.Equal(t, tc.wantErr, err != nil)
		})
	}
}
