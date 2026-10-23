package schema

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDateTimeValue(t *testing.T) {
	for raw, want := range map[string]time.Time{
		"2003-07-10T22:00:00.0Z":            time.Date(2003, 7, 10, 22, 0, 0, 0, time.UTC),
		"2003-07-10T22:00:00":               time.Date(2003, 7, 10, 22, 0, 0, 0, time.UTC),
		"2026-10-01T23:59:59.5+02:00":       time.Date(2026, 10, 1, 21, 59, 59, 500_000_000, time.UTC),
		"2026-10-01T00:00:00-01:30\n":       time.Date(2026, 10, 1, 1, 30, 0, 0, time.UTC),
		"2026-12-31T24:00:00Z":              time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		"2026-10-01T00:00:00.123456789123Z": time.Date(2026, 10, 1, 0, 0, 0, 123_456_789, time.UTC),
	} {
		t.Run(raw, func(t *testing.T) {
			require.True(t, isDateTime(raw))
			got, err := DateTimeValue(raw)
			require.NoError(t, err)
			assert.True(t, want.Equal(got), "got %s", got)
		})
	}
	for _, raw := range []string{"12026-10-01T00:00:00Z", "-0004-02-29T00:00:00Z", "9999-12-31T23:00:00-14:00", "0001-01-01T00:00:00+14:00"} {
		require.True(t, isDateTime(raw), raw)
		_, err := DateTimeValue(raw)
		assert.Error(t, err, "%s: valid, but its UTC year is outside 1 to 9999", raw)
	}
	_, err := DateTimeValue("0001-01-01T00:00:00Z")
	assert.NoError(t, err)
	_, err = DateTimeValue("not a time")
	assert.Error(t, err)
}
