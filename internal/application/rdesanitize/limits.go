package rdesanitize

import (
	"errors"
	"strconv"
	"time"
)

// Limits bound the XML-shaped resources the validator's byte budget does not
// already cover.
//
// Compression-ratio bombs are deliberately absent: rdevalidate's unpack budget
// caps the absolute number of decompressed bytes across every nesting layer
// (internal/application/rdevalidate/archive.go), which subsumes a ratio check
// and cannot be evaded by nesting. What that budget cannot see is document
// shape, so depth, element count and field length live here.
type Limits struct {
	MaxXMLDepth   int
	MaxElements   int64
	MaxFieldBytes int
	Timeout       time.Duration
}

// DefaultLimits returns the shipped defaults. MaxElements is generous because a
// large TLD legitimately produces hundreds of millions of elements; it exists
// to stop a pathological document, not to size a real one.
func DefaultLimits() Limits {
	return Limits{
		MaxXMLDepth:   32,
		MaxElements:   1_000_000_000,
		MaxFieldBytes: 64 << 10,
		Timeout:       4 * time.Hour,
	}
}

// Validate rejects a misconfiguration rather than silently running unbounded.
func (l Limits) Validate() error {
	switch {
	case l.MaxXMLDepth <= 0:
		return errors.New("MaxXMLDepth must be positive")
	case l.MaxElements <= 0:
		return errors.New("MaxElements must be positive")
	case l.MaxFieldBytes <= 0:
		return errors.New("MaxFieldBytes must be positive")
	case l.Timeout <= 0:
		return errors.New("Timeout must be positive")
	}
	return nil
}

func itoa(n int) string     { return strconv.Itoa(n) }
func i64toa(n int64) string { return strconv.FormatInt(n, 10) }
