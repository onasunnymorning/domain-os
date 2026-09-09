package rdevalidate

import (
	"errors"
	"strconv"
	"time"
)

// Limits are the resource bounds the pipeline enforces. They are configuration
// (design constraint 7), loaded from the env registry by the activities layer;
// the defaults here are only a fallback for tests and local runs.
type Limits struct {
	MaxCompressedBytes int64         // largest .ryde accepted
	MaxUnpackedBytes   int64         // cumulative plaintext budget across every decompression layer
	MaxFiles           int           // tar entries, including non-XML ones
	MaxNesting         int           // decompression layers (gzip inside gzip, gzip inside tar…)
	Timeout            time.Duration // wall-clock bound for one run
}

// DefaultLimits returns conservative defaults.
func DefaultLimits() Limits {
	return Limits{
		MaxCompressedBytes: 10 << 30, // 10 GiB
		MaxUnpackedBytes:   50 << 30, // 50 GiB
		MaxFiles:           8,
		MaxNesting:         2,
		Timeout:            2 * time.Hour,
	}
}

// Validate rejects limits that would make the pipeline either unbounded or
// unable to accept anything.
func (l Limits) Validate() error {
	if l.MaxCompressedBytes <= 0 || l.MaxUnpackedBytes <= 0 {
		return errors.New("limits: byte budgets must be positive")
	}
	if l.MaxFiles <= 0 {
		return errors.New("limits: MaxFiles must be positive")
	}
	if l.MaxNesting < 0 {
		return errors.New("limits: MaxNesting must not be negative")
	}
	if l.Timeout <= 0 {
		return errors.New("limits: Timeout must be positive")
	}
	return nil
}

func itoa(i int) string { return strconv.Itoa(i) }

func i64toa(i int64) string { return strconv.FormatInt(i, 10) }
