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

func TestPhase_AddFee(t *testing.T) {
	tr := true
	tc := []struct {
		name        string
		fees        []Fee
		expectedErr error
	}{
		{
			name: "NoErr",
			fees: []Fee{
				{
					Name:       "fee1",
					Currency:   "USD",
					Refundable: &tr,
					Amount:     100,
				},
			},
			expectedErr: nil,
		},
		{
			name: "NoErr One Fee Multiple Currencies",
			fees: []Fee{
				{
					Name:       "fee1",
					Currency:   "USD",
					Refundable: &tr,
					Amount:     100,
				},
				{
					Name:       "fee1",
					Currency:   "EUR",
					Refundable: &tr,
					Amount:     100,
				},
				{
					Name:       "fee1",
					Currency:   "GBP",
					Refundable: &tr,
					Amount:     100,
				},
				{
					Name:       "fee1",
					Currency:   "CHF",
					Refundable: &tr,
					Amount:     100,
				},
			},
			expectedErr: nil,
		},
		{
			name: "NoErr Multiple Fees and Currencies",
			fees: []Fee{
				{
					Name:       "fee1",
					Currency:   "USD",
					Refundable: &tr,
					Amount:     100,
				},
				{
					Name:       "fee1",
					Currency:   "EUR",
					Refundable: &tr,
					Amount:     100,
				},
				{
					Name:       "fee2",
					Currency:   "USD",
					Refundable: &tr,
					Amount:     100,
				},
				{
					Name:       "fee3",
					Currency:   "USD",
					Refundable: &tr,
					Amount:     100,
				},
			},
			expectedErr: nil,
		},
		{
			name: "Duplicate Fees",
			fees: []Fee{
				{
					Name:       "fee1",
					Currency:   "USD",
					Refundable: &tr,
					Amount:     100,
				},
				{
					Name:       "fee1",
					Currency:   "USD",
					Refundable: &tr,
					Amount:     1000,
				},
			},
			expectedErr: ErrDuplicateFeeEntry,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			phase := &Phase{}
			var err error
			for _, fee := range tt.fees {
				_, err = phase.AddFee(fee)
			}
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}
