package entities

import (
	"time"
)

// RoundTime rounds time.Time to the nearest microsecond to avoid precision issues with storage layers that use microsecond and golang nanosecond
func RoundTime(t time.Time) time.Time {
	return t.Round(time.Microsecond)
}
