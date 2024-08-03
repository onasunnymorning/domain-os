package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type DNSRecordSuite struct {
	suite.Suite
	db      *gorm.DB
	tldName string
}

func TestDNSRecordSuite(t *testing.T) {
	suite.Run(t, new(DNSRecordSuite))
}

func (s *DNSRecordSuite) SetupSuite() {
	s.db = setupTestDB()

	// Create a TLD
	tld, err := entities.NewTLD("waves")
	s.Require().NoError(err)
	tldRepo := NewGormTLDRepo(s.db)
	err = tldRepo.Create(context.Background(), tld)
	s.Require().NoError(err)
	s.tldName = tld.Name.String()
}

func (s *DNSRecordSuite) TearDownSuite() {
	if s.tldName != "" {
		tldRepo := NewGormTLDRepo(s.db)
		err := tldRepo.DeleteByName(context.Background(), s.tldName)
		s.Require().NoError(err)
	}
}

func (s *DNSRecordSuite) TestCreateDNSRecord() {
	// Create a DNS record
	record := &TLDDNSRecord{
		Zone: "waves",
		Name: "windy",
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
	records := []*TLDDNSRecord{
		{
			Zone: "waves",
			Name: "windy",
			Type: "A",
			TTL:  300,
			Data: `{"address": "192.168.0.1"}`,
		},
		{
			Zone: "waves",
			Name: "mail.windy",
			Type: "MX",
			TTL:  300,
			Data: `{"preference": 10, "exchange": "mail.windy.waves"}`,
		},
		{
			Zone: "waves",
			Name: "ns1.windy",
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
	createdRecords, err := repo.GetByZone(context.Background(), "waves")
	s.Require().NoError(err)
	s.Require().Len(createdRecords, 3)

	// Delete the records
	for _, record := range createdRecords {
		err = repo.Delete(context.Background(), record.ID)
		s.Require().NoError(err)
	}

	// Get the DNS records
	createdRecords, err = repo.GetByZone(context.Background(), "windy.waves")
	s.Require().NoError(err)
	s.Require().Len(createdRecords, 0)
}
