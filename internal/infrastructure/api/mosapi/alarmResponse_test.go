package mosapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlarmResponse_IsAlarmed(t *testing.T) {
	alarmResponse := AlarmResponse{
		Alarmed: "Yes",
	}

	assert.True(t, alarmResponse.IsAlarmed(), "Expected IsAlarmed to return true")

	alarmResponse.Alarmed = "No"

	assert.False(t, alarmResponse.IsAlarmed(), "Expected IsAlarmed to return false")
}
