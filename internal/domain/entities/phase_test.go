package entities

import (
	"errors"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
)

func TestNewPhase(t *testing.T) {
	tc := []struct {
		name        string
		phaseType   string
		start       time.Time
		expected    *Phase
		expectedErr error
	}{
		{
			name:        "GA NoErr",
			phaseType:   "GA",
			start:       time.Now(),
			expectedErr: nil,
		},
		{
			name:        "Launch NoErr",
			phaseType:   "Launch",
			start:       time.Now(),
			expectedErr: nil,
		},
		{
			name:        "I",
			phaseType:   "Launch",
			start:       time.Now(),
			expectedErr: errors.Join(ErrInvalidPhaseName, ErrInvalidClIDType),
		},
		{
			name:        "Invalid Type",
			phaseType:   "Invalid",
			start:       time.Now(),
			expectedErr: ErrInvalidPhaseType,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			phase, err := NewPhase(tt.name, tt.phaseType, tt.start)
			assert.Equal(t, tt.expectedErr, err)

			if tt.expectedErr == nil {
				assert.Equal(t, ClIDType(tt.name), phase.Name)
				assert.Equal(t, PhaseType(tt.phaseType), phase.Type)
				assert.True(t, phase.Starts.Before(time.Now()))
			}
		})
	}
}
