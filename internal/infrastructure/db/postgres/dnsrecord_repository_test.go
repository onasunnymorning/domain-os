package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type DNSRecordSuite struct {
	suite.Suite
	db *gorm.DB
}

func TestDNSRecordSuite(t *testing.T) {
	suite.Run(t, new(DNSRecordSuite))
}

func (s *DNSRecordSuite) SetupSuite() {
	s.db = setupTestDB()
}

func (s *DNSRecordSuite) TearDownSuite() {
}

func (s *DNSRecordSuite) TestCreateDNSRecord() {
	// Create a DNS record
	record := &DNSRecord{
		Zone: "windy.waves",
		Name: "www",
		Type: "A",
		TTL:  300,
		Data: `{"address": "192.168.0.1"}`,
	}

	repo := NewGormDNSRecordRepository(s.db)
	createdRecord, err := repo.Create(context.Background(), record)
	s.Require().NoError(err)
	s.Require().NotNil(createdRecord)
	s.Require().NotZero(createdRecord.ID)

	// Try and insert a duplicate record
	_, err = repo.Create(context.Background(), record)
	s.Require().Error(err)
}
