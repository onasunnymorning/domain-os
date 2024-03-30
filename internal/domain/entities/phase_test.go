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

func TestPhase_AddPrice(t *testing.T) {
	tc := []struct {
		name        string
		prices      []Price
		expectedErr error
	}{
		{
			name: "NoErr",
			prices: []Price{
				{
					Currency:           "USD",
					RegistrationAmount: 100,
					RenewalAmount:      100,
					TransferAmount:     100,
					RestoreAmount:      100,
				},
			},
			expectedErr: nil,
		},
		{
			name: "NoErr Multiple Currencies",
			prices: []Price{
				{
					Currency:           "USD",
					RegistrationAmount: 100,
					RenewalAmount:      100,
					TransferAmount:     100,
					RestoreAmount:      100,
				},
				{
					Currency:           "EUR",
					RegistrationAmount: 100,
					RenewalAmount:      100,
					TransferAmount:     100,
					RestoreAmount:      100,
				},
				{
					Currency:           "PEN",
					RegistrationAmount: 100,
					RenewalAmount:      100,
					TransferAmount:     100,
					RestoreAmount:      100,
				},
			},
			expectedErr: nil,
		},
		{
			name: "Duplicate Prices",
			prices: []Price{
				{
					Currency:           "USD",
					RegistrationAmount: 100,
					RenewalAmount:      100,
					TransferAmount:     100,
					RestoreAmount:      100,
				},
				{
					Currency:           "EUR",
					RegistrationAmount: 100,
					RenewalAmount:      100,
					TransferAmount:     100,
					RestoreAmount:      100,
				},
				{
					Currency:           "EUR",
					RegistrationAmount: 100,
					RenewalAmount:      100,
					TransferAmount:     100,
					RestoreAmount:      100,
				},
			},
			expectedErr: ErrDuplicatePriceEntry,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			phase := &Phase{}
			var err error
			for _, price := range tt.prices {
				_, err = phase.AddPrice(price)
			}
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestPhase_SetEndFuturePhase(t *testing.T) {
	tc := []struct {
		name        string
		end         time.Time
		expectedErr error
	}{
		{
			name:        "NoErr",
			end:         time.Now().Add(time.Hour * 48),
			expectedErr: nil,
		},
		{
			name:        "Before Start",
			end:         time.Now().Add(time.Hour * 2),
			expectedErr: ErrEndDateBeforeStart,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			phase := &Phase{
				Starts: time.Now().Add(time.Hour * 24),
			}
			err := phase.SetEnd(tt.end)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestPhase_SetEndCurrentPhase(t *testing.T) {
	tc := []struct {
		name        string
		end         time.Time
		expectedErr error
	}{
		{
			name:        "NoErr",
			end:         time.Now().Add(time.Hour * 48),
			expectedErr: nil,
		},
		{
			name:        "In the Past",
			end:         time.Now().Add(-time.Hour * 2),
			expectedErr: ErrEndDateInPast,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			phase := &Phase{
				Starts: time.Now().Add(-time.Hour * 24),
			}
			err := phase.SetEnd(tt.end)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestPhase_SetEndAlreadySet(t *testing.T) {
	tc := []struct {
		name        string
		end         time.Time
		expectedErr error
	}{
		{
			name:        "Already Set",
			end:         time.Now().Add(-time.Hour * 2),
			expectedErr: ErrEndDateAlreadySet,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			phase := &Phase{
				Starts: time.Now().Add(-time.Hour * 24),
				Ends:   &tt.end,
			}
			err := phase.SetEnd(tt.end)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}
