package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/api/frankfurter"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockFXRatesSource mocks the Frankfurter client for RefreshFXRates tests.
type mockFXRatesSource struct{ mock.Mock }

func (m *mockFXRatesSource) GetLatestRates(ctx context.Context, base string, quotes []string) ([]frankfurter.Rate, error) {
	args := m.Called(ctx, base, quotes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]frankfurter.Rate), args.Error(1)
}

func newFXTestSyncService(fxRepo *mockFXRepository, rates *mockFXRatesSource) *SyncService {
	svc := NewSyncService(nil, nil, nil, nil, fxRepo)
	svc.SetFXRatesSource(rates)
	return svc
}

func TestRefreshFXRates_Success(t *testing.T) {
	fxRepo := new(mockFXRepository)
	rates := new(mockFXRatesSource)
	svc := newFXTestSyncService(fxRepo, rates)

	rates.On("GetLatestRates", mock.Anything, "USD", []string(nil)).Return([]frankfurter.Rate{
		{Date: "2026-07-17", Base: "USD", Quote: "EUR", Rate: 0.87209},
		{Date: "2026-07-17", Base: "USD", Quote: "PEN", Rate: 3.3903},
	}, nil)

	var stored []*entities.FX
	fxRepo.On("UpdateAll", mock.Anything, mock.AnythingOfType("[]*entities.FX")).
		Run(func(args mock.Arguments) { stored = args.Get(1).([]*entities.FX) }).
		Return(nil)

	err := svc.RefreshFXRates(context.Background(), "USD")
	require.NoError(t, err)

	require.Len(t, stored, 2)
	assert.Equal(t, "USD", stored[0].BaseCurrency)
	assert.Equal(t, "EUR", stored[0].TargetCurrency)
	assert.Equal(t, 0.87209, stored[0].Rate)
	assert.Equal(t, time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC), stored[0].Date)
}

func TestRefreshFXRates_APIError_PropagatesWithoutPanic(t *testing.T) {
	// Regression: the previous implementation printed the error to stdout and
	// then dereferenced the nil response, panicking the caller.
	fxRepo := new(mockFXRepository)
	rates := new(mockFXRatesSource)
	svc := newFXTestSyncService(fxRepo, rates)

	rates.On("GetLatestRates", mock.Anything, "USD", []string(nil)).
		Return(nil, errors.New("connection refused"))

	err := svc.RefreshFXRates(context.Background(), "USD")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRetrievingFXRates)
	assert.Contains(t, err.Error(), "connection refused")
	fxRepo.AssertNotCalled(t, "UpdateAll", mock.Anything, mock.Anything)
}

func TestRefreshFXRates_InvalidDate_Errors(t *testing.T) {
	fxRepo := new(mockFXRepository)
	rates := new(mockFXRatesSource)
	svc := newFXTestSyncService(fxRepo, rates)

	rates.On("GetLatestRates", mock.Anything, "USD", []string(nil)).Return([]frankfurter.Rate{
		{Date: "garbage", Base: "USD", Quote: "EUR", Rate: 0.87},
	}, nil)

	err := svc.RefreshFXRates(context.Background(), "USD")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRetrievingFXRates)
	fxRepo.AssertNotCalled(t, "UpdateAll", mock.Anything, mock.Anything)
}
