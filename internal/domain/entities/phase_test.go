package entities

import (
	"errors"
	"fmt"
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
			start:       time.Now().UTC(),
			expectedErr: nil,
		},
		{
			name:        "Launch NoErr",
			phaseType:   "Launch",
			start:       time.Now().UTC(),
			expectedErr: nil,
		},
		{
			name:        "Launch Not UTC",
			phaseType:   "Launch",
			start:       time.Now().In(time.FixedZone("UTC+1", 3600)),
			expectedErr: ErrTimeStampNotUTC,
		},
		{
			name:        "I",
			phaseType:   "Launch",
			start:       time.Now().UTC(),
			expectedErr: errors.Join(ErrInvalidPhaseName, ErrInvalidClIDType),
		},
		{
			name:        "Invalid Type",
			phaseType:   "Invalid",
			start:       time.Now().UTC(),
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
			end:         time.Now().UTC().Add(time.Hour * 48),
			expectedErr: nil,
		},
		{
			name:        "Err Not in UTC",
			end:         time.Now().In(time.FixedZone("UTC+1", 3600)).Add(time.Hour * 48),
			expectedErr: ErrTimeStampNotUTC,
		},
		{
			name:        "Before Start",
			end:         time.Now().UTC().Add(time.Hour * 2),
			expectedErr: ErrEndDateBeforeStart,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			phase := &Phase{
				Starts: time.Now().UTC().Add(time.Hour * 24),
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
			end:         time.Now().UTC().Add(time.Hour * 48),
			expectedErr: nil,
		},
		{
			name:        "In the Past",
			end:         time.Now().UTC().Add(-time.Hour * 2),
			expectedErr: ErrEndDateInPast,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			phase := &Phase{
				Starts: time.Now().UTC().Add(-time.Hour * 24),
			}
			err := phase.SetEnd(tt.end)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestPhase_IsCurrentlyActive(t *testing.T) {
	tc := []struct {
		name     string
		start    time.Time
		end      time.Time
		expected bool
	}{
		{
			name:     "Active",
			start:    time.Now().UTC().Add(-time.Hour * 24),
			end:      time.Now().UTC().Add(time.Hour * 24),
			expected: true,
		},
		{
			name:     "Not Active",
			start:    time.Now().UTC().Add(time.Hour * 24),
			end:      time.Now().UTC().Add(time.Hour * 48),
			expected: false,
		},
		{
			name:     "Not Active No End",
			start:    time.Now().UTC().Add(-time.Hour * 24),
			end:      time.Time{},
			expected: false,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			phase := &Phase{
				Starts: tt.start,
				Ends:   &tt.end,
			}
			assert.Equal(t, tt.expected, phase.IsCurrentlyActive())
		})
	}
}

func TestPhase_OverlapsWith(t *testing.T) {
	tc := []struct {
		name       string
		thisStart  string
		thisEnd    string
		otherStart string
		otherEnd   string
		expected   bool
	}{
		{
			name:       "both no end date",
			thisStart:  "2021-01-01T00:00:00Z",
			thisEnd:    "",
			otherStart: "2021-01-01T00:00:00Z",
			otherEnd:   "",
			expected:   true,
		},
		{
			name:       "no end + start after",
			thisStart:  "2021-01-01T00:00:00Z",
			thisEnd:    "",
			otherStart: "2022-01-01T00:00:00Z",
			otherEnd:   "2123-01-01T00:00:00Z",
			expected:   true,
		},
		{
			name:       "no end + starts on end date",
			thisStart:  "2121-01-01T00:00:00Z",
			thisEnd:    "",
			otherStart: "2120-01-01T00:00:00Z",
			otherEnd:   "2121-01-01T00:00:00Z",
			expected:   true,
		},
		{
			name:       "no end + starts just before end date",
			thisStart:  "2121-01-01T00:00:00Z",
			thisEnd:    "",
			otherStart: "2120-01-01T00:00:00Z",
			otherEnd:   "2120-12-12T23:59:59Z",
			expected:   false,
		},
		{
			name:       "other has no end + start after",
			thisStart:  "2022-01-01T00:00:00Z",
			thisEnd:    "2123-01-01T00:00:00Z",
			otherStart: "2021-01-01T00:00:00Z",
			otherEnd:   "",
			expected:   true,
		},
		{
			name:       "other has no end + starts on end date",
			thisStart:  "2120-01-01T00:00:00Z",
			thisEnd:    "2121-01-01T00:00:00Z",
			otherStart: "2121-01-01T00:00:00Z",
			otherEnd:   "",
			expected:   true,
		},
		{
			name:       "other has no end + starts just before end date",
			thisStart:  "2120-01-01T00:00:00Z",
			thisEnd:    "2120-12-12T23:59:59Z",
			otherStart: "2121-01-01T00:00:00Z",
			otherEnd:   "",
			expected:   false,
		},
		{
			name:       "both end and are adjacent with this phase first",
			thisStart:  "2120-01-01T00:00:00Z",
			thisEnd:    "2120-12-12T23:59:59Z",
			otherStart: "2121-01-01T00:00:00Z",
			otherEnd:   "2122-01-01T00:00:00Z",
			expected:   false,
		},
		{
			name:       "both end and are overlap slightly wiht this phase first",
			thisStart:  "2120-01-01T00:00:00Z",
			thisEnd:    "2121-01-01T00:00:00Z",
			otherStart: "2121-01-01T00:00:00Z",
			otherEnd:   "2122-01-01T00:00:00Z",
			expected:   true,
		},
		{
			name:       "both end and are overlap slightly wiht other phase first",
			thisStart:  "2121-01-01T00:00:00Z",
			thisEnd:    "2122-01-01T00:00:00Z",
			otherStart: "2120-01-01T00:00:00Z",
			otherEnd:   "2121-01-01T00:00:00Z",
			expected:   true,
		},
		{
			name:       "both end and are adjacent with the other phase first",
			thisStart:  "2121-01-01T00:00:00Z",
			thisEnd:    "2122-01-01T00:00:00Z",
			otherStart: "2120-01-01T00:00:00Z",
			otherEnd:   "2120-12-12T23:59:59Z",
			expected:   false,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			// VARS
			var thisStart, thisEnd, otherStart, otherEnd time.Time
			var err error
			// SETUP
			thisStart, err = time.Parse(time.RFC3339, tt.thisStart)
			assert.Nil(t, err)
			otherStart, err = time.Parse(time.RFC3339, tt.otherStart)
			assert.Nil(t, err)
			// Create the two phases
			thisPhase, err := NewPhase("thisPhase", "GA", thisStart)
			assert.Nil(t, err)
			otherPhase, err := NewPhase("otherPhase", "GA", otherStart)
			assert.Nil(t, err)
			// Set the enddates if applicable
			if tt.thisEnd != "" {
				thisEnd, err = time.Parse(time.RFC3339, tt.thisEnd)
				assert.Nil(t, err)
				err = thisPhase.SetEnd(thisEnd)
				assert.Nil(t, err)
			}
			if tt.otherEnd != "" {
				otherEnd, err = time.Parse(time.RFC3339, tt.otherEnd)
				assert.Nil(t, err)
				err = otherPhase.SetEnd(otherEnd)
				assert.Nil(t, err)
			}

			// Run the test
			fmt.Printf("thisPhase: %v\n", thisPhase)
			fmt.Printf("otherPhase: %v\n", otherPhase)
			assert.Equal(t, tt.expected, thisPhase.OverlapsWith(otherPhase))
		})
	}
}
