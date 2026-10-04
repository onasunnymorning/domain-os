package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// EscrowIntake is the sFTP intake's configuration, read once at worker start
// (INV-17). Prefix is passed on as written; the intake activities normalise it
// and refuse a prefix that overlaps an area other code owns.
type EscrowIntake struct {
	Enabled        bool
	AllowPlaintext bool
	Prefix         string
}

// LoadEscrowIntake reads ESCROW_INTAKE_SFTP_ENABLED,
// ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT and ESCROW_INTAKE_SFTP_PREFIX. A flag that
// is set but not a boolean is an error rather than a silent false.
func LoadEscrowIntake() (EscrowIntake, error) {
	enabled, err := parseBoolDefaultFalse("ESCROW_INTAKE_SFTP_ENABLED", os.Getenv("ESCROW_INTAKE_SFTP_ENABLED"))
	if err != nil {
		return EscrowIntake{}, err
	}
	plaintext, err := parseBoolDefaultFalse("ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT", os.Getenv("ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT"))
	if err != nil {
		return EscrowIntake{}, err
	}
	return EscrowIntake{Enabled: enabled, AllowPlaintext: plaintext, Prefix: os.Getenv("ESCROW_INTAKE_SFTP_PREFIX")}, nil
}

const (
	// DefaultEscrowIntakeSweepInterval is the sweep interval when
	// ESCROW_INTAKE_SWEEP_INTERVAL is unset or invalid.
	DefaultEscrowIntakeSweepInterval = 2 * time.Minute
	// MinEscrowIntakeSweepInterval is the shortest interval accepted.
	MinEscrowIntakeSweepInterval = 30 * time.Second
)

// EscrowIntakeSweepInterval reads ESCROW_INTAKE_SWEEP_INTERVAL. It always
// returns a usable interval: the default for an unset or invalid value, the
// minimum for one below it, since a sweep that overlaps the previous one is
// skipped anyway. The error says why a set value was not used as written, for
// the caller to log; it is not a reason to stop the worker.
func EscrowIntakeSweepInterval() (time.Duration, error) {
	return parseEscrowIntakeSweepInterval(os.Getenv("ESCROW_INTAKE_SWEEP_INTERVAL"))
}

func parseEscrowIntakeSweepInterval(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultEscrowIntakeSweepInterval, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return DefaultEscrowIntakeSweepInterval, fmt.Errorf("ESCROW_INTAKE_SWEEP_INTERVAL=%q is not a positive duration; using %s", raw, DefaultEscrowIntakeSweepInterval)
	}
	if d < MinEscrowIntakeSweepInterval {
		return MinEscrowIntakeSweepInterval, fmt.Errorf("ESCROW_INTAKE_SWEEP_INTERVAL=%q is below the minimum; using %s", raw, MinEscrowIntakeSweepInterval)
	}
	return d, nil
}

// parseBoolDefaultFalse parses an optional boolean variable. Unset is false.
func parseBoolDefaultFalse(name, raw string) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s=%q: %w", name, raw, err)
	}
	return v, nil
}
