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
	s.Require().NotNil(createdRecord.ID)

	// Try and insert a duplicate record
	_, err = repo.Create(context.Background(), record)
	s.Require().Error(err)

	// Delete the record
	err = repo.Delete(context.Background(), record.ID)
	s.Require().NoError(err)
}

func (s *DNSRecordSuite) TestGetDNSRecordsByZone() {
	// Create some DNS records
	records := []*DNSRecord{
		{
			Zone: "windy.waves",
			Name: "www",
			Type: "A",
			TTL:  300,
			Data: `{"address": "192.168.0.1"}`,
		},
		{
			Zone: "windy.waves",
			Name: "mail",
			Type: "MX",
			TTL:  300,
			Data: `{"preference": 10, "exchange": "mail.windy.waves"}`,
		},
		{
			Zone: "windy.waves",
			Name: "ns1",
			Type: "NS",
			TTL:  300,
			Data: `{"ns": "ns1.windy.waves"}`,
		},
	}

	repo := NewGormDNSRecordRepository(s.db)
	for _, record := range records {
		_, err := repo.Create(context.Background(), record)
		s.Require().NoError(err)
	}

	// Get the DNS records
	createdRecords, err := repo.GetByZone(context.Background(), "windy.waves")
	s.Require().NoError(err)
	s.Require().Len(createdRecords, 3)
}
