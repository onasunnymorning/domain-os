package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewApplicationPollMessage(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	filed := func() *LaunchApplication {
		a, err := NewLaunchApplication(testAppID, "rushed.test", "rar-one", "", applicationPhase(t, true), 1, now)
		require.NoError(t, err)
		return a
	}
	for _, tc := range []struct {
		name string
		step func(*LaunchApplication) error
		text string
	}{
		{"validated", func(a *LaunchApplication) error { return a.Validated() }, "Application validated for rushed.test"},
		{"invalid", func(a *LaunchApplication) error { return a.Invalidate("not eligible") }, "Application invalid for rushed.test: not eligible"},
		{"rejected", func(a *LaunchApplication) error { return a.Reject("went elsewhere") }, "Application rejected for rushed.test: went elsewhere"},
		{"allocated", func(a *LaunchApplication) error {
			if err := a.Validated(); err != nil {
				return err
			}
			return a.Allocate("9_DOM-TEST")
		}, "Application allocated for rushed.test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := filed()
			require.NoError(t, tc.step(a))
			m := NewApplicationPollMessage(a, now)
			assert.Equal(t, ClIDType("rar-one"), m.ClID, "the applicant's queue")
			assert.Equal(t, now, m.CreatedAt)
			assert.Equal(t, tc.text, m.Message)
			require.NotNil(t, m.Application)
			assert.Equal(t, a.Status, m.Application.Status)
			assert.Nil(t, m.Transfer)
			m.Application.Status = "changed"
			assert.NotEqual(t, m.Application.Status, a.Status, "a copy")
		})
	}
}
