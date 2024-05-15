package postgres

import (
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
	err := repo.UpdateAll(fxs)
	s.Require().NoError(err)

	// Check that the records were inserted
	list, err := repo.ListByBaseCurrency("USD")
	s.Require().NoError(err)
	s.Require().Len(list, 3)

}
