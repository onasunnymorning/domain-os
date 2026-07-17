package activities

import (
	"context"
	"errors"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/api/frankfurter"
	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── mocks ──────────────────────────────────────────────────────────────────────

type mockFXStore struct{ mock.Mock }

func (m *mockFXStore) UpdateAll(ctx context.Context, fxs []*postgres.FX) error {
	return m.Called(ctx, fxs).Error(0)
}

type mockBaseCurrencyLister struct{ mock.Mock }

func (m *mockBaseCurrencyLister) ListDistinctBaseCurrencies(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

type mockRatesSource struct{ mock.Mock }

func (m *mockRatesSource) GetLatestRates(ctx context.Context, base string, quotes []string) ([]frankfurter.Rate, error) {
	args := m.Called(ctx, base, quotes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]frankfurter.Rate), args.Error(1)
}

func testRates(base string) []frankfurter.Rate {
	return []frankfurter.Rate{
		{Date: "2026-07-17", Base: base, Quote: "EUR", Rate: 0.87},
		{Date: "2026-07-17", Base: base, Quote: "PEN", Rate: 3.39},
	}
}

// ── tests ──────────────────────────────────────────────────────────────────────

func TestUpdateFXRates_ExplicitBases_Success(t *testing.T) {
	store := new(mockFXStore)
	lister := new(mockBaseCurrencyLister)
	rates := new(mockRatesSource)
	a := NewFXActivitiesWithDeps(store, lister, rates)

	rates.On("GetLatestRates", mock.Anything, "USD", []string(nil)).Return(testRates("USD"), nil)
	rates.On("GetLatestRates", mock.Anything, "EUR", []string(nil)).Return(testRates("EUR"), nil)
	store.On("UpdateAll", mock.Anything, mock.AnythingOfType("[]*postgres.FX")).Return(nil)

	result, err := a.UpdateFXRates(context.Background(), "corr", []string{"USD", "EUR"})
	require.NoError(t, err)

	assert.Equal(t, []string{"USD", "EUR"}, result.BasesUpdated)
	assert.Equal(t, 4, result.RatesStored)
	assert.Empty(t, result.Failures)
	assert.False(t, result.DerivedFromPhases)
	// The phase lister must not be consulted when bases are supplied
	lister.AssertNotCalled(t, "ListDistinctBaseCurrencies", mock.Anything)
}

func TestUpdateFXRates_DerivesBasesFromPhases(t *testing.T) {
	store := new(mockFXStore)
	lister := new(mockBaseCurrencyLister)
	rates := new(mockRatesSource)
	a := NewFXActivitiesWithDeps(store, lister, rates)

	lister.On("ListDistinctBaseCurrencies", mock.Anything).Return([]string{"PEN", "USD"}, nil)
	rates.On("GetLatestRates", mock.Anything, "PEN", []string(nil)).Return(testRates("PEN"), nil)
	rates.On("GetLatestRates", mock.Anything, "USD", []string(nil)).Return(testRates("USD"), nil)
	store.On("UpdateAll", mock.Anything, mock.Anything).Return(nil)

	result, err := a.UpdateFXRates(context.Background(), "corr", nil)
	require.NoError(t, err)

	assert.True(t, result.DerivedFromPhases)
	assert.Equal(t, []string{"PEN", "USD"}, result.BasesUpdated)
}

func TestUpdateFXRates_NoPhases_FallsBackToUSD(t *testing.T) {
	store := new(mockFXStore)
	lister := new(mockBaseCurrencyLister)
	rates := new(mockRatesSource)
	a := NewFXActivitiesWithDeps(store, lister, rates)

	lister.On("ListDistinctBaseCurrencies", mock.Anything).Return([]string{}, nil)
	rates.On("GetLatestRates", mock.Anything, "USD", []string(nil)).Return(testRates("USD"), nil)
	store.On("UpdateAll", mock.Anything, mock.Anything).Return(nil)

	result, err := a.UpdateFXRates(context.Background(), "corr", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"USD"}, result.BasesUpdated)
}

func TestUpdateFXRates_PartialFailure_ReportedNotFatal(t *testing.T) {
	store := new(mockFXStore)
	lister := new(mockBaseCurrencyLister)
	rates := new(mockRatesSource)
	a := NewFXActivitiesWithDeps(store, lister, rates)

	rates.On("GetLatestRates", mock.Anything, "USD", []string(nil)).Return(testRates("USD"), nil)
	rates.On("GetLatestRates", mock.Anything, "XXX", []string(nil)).Return(nil, errors.New("404 no rates"))
	store.On("UpdateAll", mock.Anything, mock.Anything).Return(nil)

	result, err := a.UpdateFXRates(context.Background(), "corr", []string{"USD", "XXX"})
	require.NoError(t, err, "a partial failure must not fail the activity")

	assert.Equal(t, []string{"USD"}, result.BasesUpdated)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "XXX", result.Failures[0].BaseCurrency)
	assert.Contains(t, result.Failures[0].Error, "404")
}

func TestUpdateFXRates_AllBasesFail_ReturnsError(t *testing.T) {
	store := new(mockFXStore)
	lister := new(mockBaseCurrencyLister)
	rates := new(mockRatesSource)
	a := NewFXActivitiesWithDeps(store, lister, rates)

	rates.On("GetLatestRates", mock.Anything, mock.Anything, []string(nil)).Return(nil, errors.New("network down"))

	result, err := a.UpdateFXRates(context.Background(), "corr", []string{"USD", "EUR"})
	require.Error(t, err, "when nothing succeeds the activity must error so Temporal retries")
	assert.Empty(t, result.BasesUpdated)
	assert.Len(t, result.Failures, 2)
	store.AssertNotCalled(t, "UpdateAll", mock.Anything, mock.Anything)
}

func TestUpdateFXRates_StoreFailure_LeavesOtherBasesUnaffected(t *testing.T) {
	store := new(mockFXStore)
	lister := new(mockBaseCurrencyLister)
	rates := new(mockRatesSource)
	a := NewFXActivitiesWithDeps(store, lister, rates)

	rates.On("GetLatestRates", mock.Anything, mock.Anything, []string(nil)).Return(testRates("USD"), nil)
	// First store call fails, second succeeds
	store.On("UpdateAll", mock.Anything, mock.Anything).Return(errors.New("db lock")).Once()
	store.On("UpdateAll", mock.Anything, mock.Anything).Return(nil).Once()

	result, err := a.UpdateFXRates(context.Background(), "corr", []string{"USD", "EUR"})
	require.NoError(t, err)
	assert.Len(t, result.BasesUpdated, 1)
	require.Len(t, result.Failures, 1)
	assert.Contains(t, result.Failures[0].Error, "failed to store rates")
}

func TestUpdateFXRates_InvalidRateDate_RecordedAsFailure(t *testing.T) {
	store := new(mockFXStore)
	lister := new(mockBaseCurrencyLister)
	rates := new(mockRatesSource)
	a := NewFXActivitiesWithDeps(store, lister, rates)

	bad := []frankfurter.Rate{{Date: "not-a-date", Base: "USD", Quote: "EUR", Rate: 0.87}}
	rates.On("GetLatestRates", mock.Anything, "USD", []string(nil)).Return(bad, nil)
	rates.On("GetLatestRates", mock.Anything, "EUR", []string(nil)).Return(testRates("EUR"), nil)
	store.On("UpdateAll", mock.Anything, mock.Anything).Return(nil)

	result, err := a.UpdateFXRates(context.Background(), "corr", []string{"USD", "EUR"})
	require.NoError(t, err)
	assert.Equal(t, []string{"EUR"}, result.BasesUpdated)
	require.Len(t, result.Failures, 1)
	assert.Contains(t, result.Failures[0].Error, "invalid rate date")
}

func TestUpdateFXRates_PhaseListerError_ReturnsError(t *testing.T) {
	store := new(mockFXStore)
	lister := new(mockBaseCurrencyLister)
	rates := new(mockRatesSource)
	a := NewFXActivitiesWithDeps(store, lister, rates)

	lister.On("ListDistinctBaseCurrencies", mock.Anything).Return(nil, errors.New("db down"))

	_, err := a.UpdateFXRates(context.Background(), "corr", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list phase base currencies")
}
