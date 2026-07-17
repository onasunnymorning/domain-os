package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type FXSuite struct {
	suite.Suite
	db *gorm.DB
}

func TestFXSuite(t *testing.T) {
	suite.Run(t, new(FXSuite))
}

func (s *FXSuite) SetupSuite() {
	s.db = setupTestDB()
}

func (s *FXSuite) TearDownSuite() {
}

func (s *FXSuite) TestFX_UpdateAll() {
	testTimeString := "2021-01-01T00:00:00Z"
	testTime, _ := time.Parse(time.RFC3339, testTimeString)
	fxs := []*FX{
		{
			Date:   testTime,
			Base:   "USD",
			Target: "EUR",
			Rate:   1.5,
		},
		{
			Date:   testTime,
			Base:   "USD",
			Target: "JPY",
			Rate:   100.0,
		},
		{
			Date:   testTime,
			Base:   "USD",
			Target: "PEN",
			Rate:   3.72312,
		},
	}

	repo := NewFXRepository(s.db)
	err := repo.UpdateAll(context.Background(), fxs)
	s.Require().NoError(err)

	// Check that the records were inserted
	list, err := repo.ListByBaseCurrency(context.Background(), "USD")
	s.Require().NoError(err)
	s.Require().Len(list, 3)

	// Check if we can get one record
	fx, err := repo.GetByBaseAndTargetCurrency(context.Background(), "USD", "JPY")
	s.Require().NoError(err)
	s.Require().Equal("USD", fx.BaseCurrency)
	s.Require().Equal("JPY", fx.TargetCurrency)
	s.Require().Equal(100.0, fx.Rate)

}

// TestFX_UpdateAll_ReplacesOnlyGivenBase verifies the replace is scoped to the
// base currency of the supplied rates and leaves other bases untouched.
func (s *FXSuite) TestFX_UpdateAll_ReplacesOnlyGivenBase() {
	repo := NewFXRepository(s.db)
	day1, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	day2, _ := time.Parse(time.RFC3339, "2026-01-02T00:00:00Z")

	s.Require().NoError(repo.UpdateAll(context.Background(), []*FX{
		{Date: day1, Base: "GBP", Target: "EUR", Rate: 1.2},
	}))
	s.Require().NoError(repo.UpdateAll(context.Background(), []*FX{
		{Date: day1, Base: "CAD", Target: "EUR", Rate: 0.7},
	}))

	// Replacing GBP must not touch CAD
	s.Require().NoError(repo.UpdateAll(context.Background(), []*FX{
		{Date: day2, Base: "GBP", Target: "EUR", Rate: 1.25},
	}))

	gbp, err := repo.ListByBaseCurrency(context.Background(), "GBP")
	s.Require().NoError(err)
	s.Require().Len(gbp, 1)
	s.Require().Equal(1.25, gbp[0].Rate)

	cad, err := repo.ListByBaseCurrency(context.Background(), "CAD")
	s.Require().NoError(err)
	s.Require().Len(cad, 1, "replacing one base must not delete another base's rates")
}

// TestFX_GetByBaseAndTargetCurrency_ReturnsLatest is a regression test: the
// query must return the NEWEST rate when rates for multiple dates coexist
// (a bare First() would return the oldest, as the primary key starts with date).
func (s *FXSuite) TestFX_GetByBaseAndTargetCurrency_ReturnsLatest() {
	older, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	newer, _ := time.Parse(time.RFC3339, "2026-06-01T00:00:00Z")

	// Insert two dates for the same pair directly (bypassing UpdateAll,
	// which would wipe the older row)
	s.Require().NoError(s.db.Create(&FX{Date: older, Base: "AUD", Target: "EUR", Rate: 0.55}).Error)
	s.Require().NoError(s.db.Create(&FX{Date: newer, Base: "AUD", Target: "EUR", Rate: 0.60}).Error)
	defer s.db.Where("base = ?", "AUD").Delete(&FX{})

	repo := NewFXRepository(s.db)
	fx, err := repo.GetByBaseAndTargetCurrency(context.Background(), "AUD", "EUR")
	s.Require().NoError(err)
	s.Require().Equal(0.60, fx.Rate, "must return the newest rate, not the oldest")
	s.Require().Equal(newer, fx.Date.UTC())
}

// TestFX_UpdateAll_EmptyInput verifies a no-op on empty input (previously an
// index-out-of-range panic risk on fxs[0]).
func (s *FXSuite) TestFX_UpdateAll_EmptyInput() {
	repo := NewFXRepository(s.db)
	s.Require().NoError(repo.UpdateAll(context.Background(), []*FX{}))
}
