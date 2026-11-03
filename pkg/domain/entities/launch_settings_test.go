package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLaunchSettings(t *testing.T) {
	s := NewLaunchSettings()
	assert.Equal(t, LaunchModeOpen, s.Mode)
	assert.Equal(t, RegistrationModelFirstCome, s.Model)
	assert.True(t, s.AcceptSignedMarks)
	assert.False(t, s.AcceptCodeMarks)
	assert.False(t, s.RetainMarks)
	assert.NoError(t, s.Validate())
	assert.False(t, s.IsZero())
	assert.True(t, LaunchSettings{}.IsZero())
}

func TestLaunchSettings_Validate(t *testing.T) {
	tests := []struct {
		name     string
		settings LaunchSettings
		err      error
	}{
		{"open first-come", LaunchSettings{Mode: LaunchModeOpen, Model: RegistrationModelFirstCome}, nil},
		{"landrush application", LaunchSettings{Mode: LaunchModeLandrush, Model: RegistrationModelApplication}, nil},
		{"custom", LaunchSettings{Mode: LaunchModeCustom, Model: RegistrationModelFirstCome}, nil},
		{"claims", LaunchSettings{Mode: LaunchModeClaims, Model: RegistrationModelFirstCome}, nil},
		{"sunrise with signed marks", LaunchSettings{Mode: LaunchModeSunrise, Model: RegistrationModelApplication, AcceptSignedMarks: true}, nil},
		{"sunrise with code marks", LaunchSettings{Mode: LaunchModeSunrise, Model: RegistrationModelApplication, AcceptCodeMarks: true}, nil},
		{"sunrise without marks", LaunchSettings{Mode: LaunchModeSunrise, Model: RegistrationModelApplication}, ErrInvalidLaunchSettings},
		{"zero value", LaunchSettings{}, ErrInvalidLaunchMode},
		{"unknown mode", LaunchSettings{Mode: "premium", Model: RegistrationModelFirstCome}, ErrInvalidLaunchMode},
		{"unknown model", LaunchSettings{Mode: LaunchModeOpen, Model: "auction"}, ErrInvalidRegistrationModel},
		{"mode case matters", LaunchSettings{Mode: "Sunrise", Model: RegistrationModelFirstCome}, ErrInvalidLaunchMode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.Validate()
			if tt.err == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.err)
		})
	}
}

func TestLaunchSettings_RequiresVerification(t *testing.T) {
	for mode, want := range map[LaunchMode]bool{
		LaunchModeOpen: false, LaunchModeLandrush: false, LaunchModeCustom: false,
		LaunchModeSunrise: true, LaunchModeClaims: true,
	} {
		assert.Equal(t, want, LaunchSettings{Mode: mode}.RequiresVerification(), mode)
	}
}

func TestLaunchSettings_AdmitsRegistration(t *testing.T) {
	tests := []struct {
		name     string
		settings LaunchSettings
		err      error
	}{
		{"GA phase (zero value)", LaunchSettings{}, nil},
		{"open first-come", LaunchSettings{Mode: LaunchModeOpen, Model: RegistrationModelFirstCome}, nil},
		{"landrush first-come", LaunchSettings{Mode: LaunchModeLandrush, Model: RegistrationModelFirstCome}, nil},
		{"landrush by application", LaunchSettings{Mode: LaunchModeLandrush, Model: RegistrationModelApplication}, ErrLaunchApplicationsUnavailable},
		{"sunrise first-come", LaunchSettings{Mode: LaunchModeSunrise, Model: RegistrationModelFirstCome, AcceptSignedMarks: true}, ErrLaunchModeUnsupported},
		{"sunrise by application: verification first", LaunchSettings{Mode: LaunchModeSunrise, Model: RegistrationModelApplication, AcceptSignedMarks: true}, ErrLaunchModeUnsupported},
		{"claims", LaunchSettings{Mode: LaunchModeClaims, Model: RegistrationModelFirstCome}, ErrLaunchModeUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.AdmitsRegistration()
			if tt.err == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.err)
		})
	}
}

func TestLaunchMode_IsValid(t *testing.T) {
	assert.True(t, LaunchModeOpen.IsValid())
	assert.False(t, LaunchMode("").IsValid())
	assert.False(t, LaunchMode("GA").IsValid())
	assert.True(t, RegistrationModelApplication.IsValid())
	assert.False(t, RegistrationModel("").IsValid())
}
