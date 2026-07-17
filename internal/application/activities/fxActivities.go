package activities

import (
	"context"
	"fmt"
	"os"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/api/frankfurter"
	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"go.temporal.io/sdk/activity"
	"gorm.io/gorm"
)

// FXRatesSource fetches the latest exchange rates for a base currency.
// Implemented by frankfurter.Client; abstracted for testability.
type FXRatesSource interface {
	GetLatestRates(ctx context.Context, base string, quotes []string) ([]frankfurter.Rate, error)
}

// fxStore is the subset of the FX repository the activities need.
type fxStore interface {
	UpdateAll(ctx context.Context, fxs []*postgres.FX) error
}

// baseCurrencyLister provides the distinct base currencies configured across phases.
type baseCurrencyLister interface {
	ListDistinctBaseCurrencies(ctx context.Context) ([]string, error)
}

// FXActivities holds dependencies for FX rate update activities that call the
// Frankfurter API and write to the database directly — no admin-API HTTP hop.
type FXActivities struct {
	fxRepo    fxStore
	phaseRepo baseCurrencyLister
	rates     FXRatesSource
}

// NewFXActivities creates FXActivities wired to the database (same init
// pattern as LifecycleActivities) and the public Frankfurter API.
func NewFXActivities() (*FXActivities, error) {
	var gormDB *gorm.DB
	var err error
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		gormDB, err = postgres.NewConnectionFromURL(dbURL, false)
	} else {
		dbCfg := postgres.Config{
			User:    os.Getenv("DB_USER"),
			Pass:    os.Getenv("DB_PASS"),
			Host:    os.Getenv("DB_HOST"),
			Port:    os.Getenv("DB_PORT"),
			DBName:  os.Getenv("DB_NAME"),
			SSLmode: os.Getenv("DB_SSLMODE"),
		}
		gormDB, err = postgres.NewConnection(dbCfg)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DB for FX activities: %w", err)
	}

	return &FXActivities{
		fxRepo:    postgres.NewFXRepository(gormDB),
		phaseRepo: postgres.NewGormPhaseRepository(gormDB),
		rates:     frankfurter.NewClient(),
	}, nil
}

// NewFXActivitiesWithDeps creates FXActivities with explicit dependencies (used in tests).
func NewFXActivitiesWithDeps(fxRepo fxStore, phaseRepo baseCurrencyLister, rates FXRatesSource) *FXActivities {
	return &FXActivities{fxRepo: fxRepo, phaseRepo: phaseRepo, rates: rates}
}

// FXBaseFailure records a failed rate update for a single base currency.
type FXBaseFailure struct {
	BaseCurrency string `json:"baseCurrency"`
	Error        string `json:"error"`
}

// UpdateFXRatesResult is the structured output of the UpdateFXRates activity.
type UpdateFXRatesResult struct {
	// BasesUpdated lists the base currencies whose rates were replaced.
	BasesUpdated []string `json:"basesUpdated"`
	// RatesStored is the total number of individual rates written.
	RatesStored int `json:"ratesStored"`
	// Failures lists base currencies that could not be updated. Existing rates
	// for those bases are left untouched (the replace is transactional).
	Failures []FXBaseFailure `json:"failures,omitempty"`
	// DerivedFromPhases is true when the base list was derived from the
	// distinct phase base currencies rather than supplied by the caller.
	DerivedFromPhases bool `json:"derivedFromPhases"`
}

// UpdateFXRates fetches the latest rates from Frankfurter for each base
// currency and atomically replaces that base's rates in the database.
//
// When bases is empty, the list is derived from the distinct base currencies
// configured on phases (falling back to USD if none exist) — the exact set
// quoting needs, since GetQuote always converts from a phase's base currency.
//
// A failed base leaves its existing rates untouched and is reported in
// Failures; the activity only errors (triggering a Temporal retry) when no
// base could be updated at all. Safe to retry: each base update is an
// idempotent transactional replace.
func (a *FXActivities) UpdateFXRates(ctx context.Context, correlationID string, bases []string) (UpdateFXRatesResult, error) {
	result := UpdateFXRatesResult{
		BasesUpdated: []string{},
		Failures:     []FXBaseFailure{},
	}

	// Derive the base list from phase configuration when not supplied
	if len(bases) == 0 {
		derived, err := a.phaseRepo.ListDistinctBaseCurrencies(ctx)
		if err != nil {
			return result, fmt.Errorf("UpdateFXRates: failed to list phase base currencies: %w", err)
		}
		bases = derived
		result.DerivedFromPhases = true
		if len(bases) == 0 {
			bases = []string{"USD"}
		}
	}

	for _, base := range bases {
		rates, err := a.rates.GetLatestRates(ctx, base, nil)
		if err != nil {
			result.Failures = append(result.Failures, FXBaseFailure{BaseCurrency: base, Error: err.Error()})
			continue
		}

		fxs := make([]*postgres.FX, 0, len(rates))
		conversionFailed := false
		for _, r := range rates {
			date, err := r.ParsedDate()
			if err != nil {
				result.Failures = append(result.Failures, FXBaseFailure{BaseCurrency: base, Error: fmt.Sprintf("invalid rate date %q for %s/%s: %v", r.Date, r.Base, r.Quote, err)})
				conversionFailed = true
				break
			}
			fxs = append(fxs, &postgres.FX{
				Date:   date,
				Base:   r.Base,
				Target: r.Quote,
				Rate:   r.Rate,
			})
		}
		if conversionFailed {
			continue
		}

		if err := a.fxRepo.UpdateAll(ctx, fxs); err != nil {
			result.Failures = append(result.Failures, FXBaseFailure{BaseCurrency: base, Error: fmt.Sprintf("failed to store rates: %v", err)})
			continue
		}

		result.BasesUpdated = append(result.BasesUpdated, base)
		result.RatesStored += len(fxs)

		// Heartbeat only inside a real activity execution — no-op in unit tests
		if activity.IsActivity(ctx) {
			activity.RecordHeartbeat(ctx, fmt.Sprintf("updated %d/%d bases", len(result.BasesUpdated), len(bases)))
		}

		if ctx.Err() != nil {
			return result, ctx.Err()
		}
	}

	// Only fail the activity when nothing succeeded — a partial result is
	// reported to the workflow, which surfaces the failures in its notes.
	if len(result.BasesUpdated) == 0 && len(result.Failures) > 0 {
		return result, fmt.Errorf("UpdateFXRates: all %d base currencies failed; first error: %s (%s)",
			len(result.Failures), result.Failures[0].Error, result.Failures[0].BaseCurrency)
	}

	return result, nil
}

// ensure interface compliance at compile time
var (
	_ FXRatesSource      = (*frankfurter.Client)(nil)
	_ fxStore            = (*postgres.FXRepository)(nil)
	_ baseCurrencyLister = (*postgres.PhaseRepository)(nil)
)
