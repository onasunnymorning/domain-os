package entities

import (
	"time"

	"errors"
)

var (
	ErrTimeStampNotUTC   = errors.New("timestamp is not in UTC")
	ErrInvalidTimeFormat = errors.New("invalid time format")
)

// RoundTime rounds time.Time to the nearest microsecond to avoid precision issues with storage layers that use microsecond and golang nanosecond
func RoundTime(t time.Time) time.Time {
	return t.Round(time.Microsecond)
}

// IsUTC checks if the time is in UTC. Returns true if the time is in UTC, false otherwise.
func IsUTC(t time.Time) bool {
	return t.Location() == time.UTC
}
