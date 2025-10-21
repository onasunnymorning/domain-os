package dnsevents

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	pgschema "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
)

const (
	testDBUser       = "postgres"
	testDBPass       = "unittest"
	testDBHost       = "127.0.0.1"
	testDBPortString = "5432"
	testDBName       = "dos_unittests"
	testSSLMode      = "require"
)

func setupTestDB() *gorm.DB {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		testDBUser, testDBPass, testDBHost, testDBPortString, testDBName, testSSLMode)
	db, err := gorm.Open(postgres.Open(dsn))
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to test database: %v", err))
	}
	return db
}

// BatchPublisherTestSuite tests the BatchPublisher
type BatchPublisherTestSuite struct {
	suite.Suite
	db *gorm.DB
}

func TestBatchPublisherSuite(t *testing.T) {
	suite.Run(t, new(BatchPublisherTestSuite))
}

func (s *BatchPublisherTestSuite) SetupSuite() {
	s.db = setupTestDB()

	// Run migrations
	err := s.db.Exec(pgschema.DNSZoneSchemaMigration).Error
	require.NoError(s.T(), err)
	err = s.db.Exec(pgschema.DNSQueueSchemaMigration).Error
	require.NoError(s.T(), err)
}

func (s *BatchPublisherTestSuite) TearDownSuite() {
	// Cleanup test data
	s.db.Exec("DROP TABLE IF EXISTS dns_change_queue CASCADE")
	s.db.Exec("DROP TABLE IF EXISTS dns_zone_journal CASCADE")
	s.db.Exec("DROP TABLE IF EXISTS dns_zone_serials CASCADE")
	s.db.Exec("DROP VIEW IF EXISTS dns_zone_status CASCADE")
	s.db.Exec("DROP VIEW IF EXISTS dns_queue_stats CASCADE")
	s.db.Exec("DROP FUNCTION IF EXISTS get_next_serial CASCADE")
	s.db.Exec("DROP FUNCTION IF EXISTS get_current_serial CASCADE")
	s.db.Exec("DROP FUNCTION IF EXISTS cleanup_dns_journal CASCADE")
	s.db.Exec("DROP FUNCTION IF EXISTS cleanup_dns_queue CASCADE")
}

func (s *BatchPublisherTestSuite) SetupTest() {
	// Clean queue before each test
	s.db.Exec("DELETE FROM dns_change_queue")
	s.db.Exec("DELETE FROM dns_zone_journal")
	s.db.Exec("DELETE FROM dns_zone_serials")
}

func (s *BatchPublisherTestSuite) TestQueueChange() {
	ctx := context.Background()

	config := &BatchPublisherConfig{
		BatchInterval: 10 * time.Second,
		MaxBatchSize:  100,
	}
	bp := NewBatchPublisher(s.db, config)

	// Queue a DNS change
	change := &DNSChange{
		ZoneName:        "test",
		ChangeType:      DNSChangeTypeAdd,
		RecordType:      DNSRecordTypeNS,
		RecordName:      "example.test.",
		RecordData:      "ns1.example.com.",
		TTL:             3600,
		SourceOperation: "TestCreate",
		DomainName:      "example.test",
	}

	err := bp.QueueChange(ctx, change)
	s.NoError(err)

	// Verify it's in the queue
	var count int64
	s.db.Raw("SELECT COUNT(*) FROM dns_change_queue WHERE published_at IS NULL").Scan(&count)
	s.Equal(int64(1), count)

	// Verify details
	type QueueRecord struct {
		ZoneName   string
		RecordName string
		RecordData string
	}
	var record QueueRecord
	s.db.Raw("SELECT zone_name, record_name, record_data FROM dns_change_queue").Scan(&record)
	s.Equal("test", record.ZoneName)
	s.Equal("example.test.", record.RecordName)
	s.Equal("ns1.example.com.", record.RecordData)
}

func (s *BatchPublisherTestSuite) TestQueueMultipleChanges() {
	ctx := context.Background()

	bp := NewBatchPublisher(s.db, nil)

	changes := []*DNSChange{
		{
			ZoneName:   "test",
			ChangeType: DNSChangeTypeAdd,
			RecordType: DNSRecordTypeNS,
			RecordName: "domain1.test.",
			RecordData: "ns1.example.com.",
			TTL:        3600,
		},
		{
			ZoneName:   "test",
			ChangeType: DNSChangeTypeAdd,
			RecordType: DNSRecordTypeNS,
			RecordName: "domain2.test.",
			RecordData: "ns2.example.com.",
			TTL:        3600,
		},
	}

	err := bp.QueueChanges(ctx, changes)
	s.NoError(err)

	var count int64
	s.db.Raw("SELECT COUNT(*) FROM dns_change_queue WHERE published_at IS NULL").Scan(&count)
	s.Equal(int64(2), count)
}

func (s *BatchPublisherTestSuite) TestFlushZone() {
	ctx := context.Background()

	bp := NewBatchPublisher(s.db, nil)

	// Queue some changes
	changes := []*DNSChange{
		{
			ZoneName:   "test",
			ChangeType: DNSChangeTypeAdd,
			RecordType: DNSRecordTypeNS,
			RecordName: "domain1.test.",
			RecordData: "ns1.example.com.",
			TTL:        3600,
		},
		{
			ZoneName:   "test",
			ChangeType: DNSChangeTypeAdd,
			RecordType: DNSRecordTypeNS,
			RecordName: "domain2.test.",
			RecordData: "ns2.example.com.",
			TTL:        3600,
		},
	}

	err := bp.QueueChanges(ctx, changes)
	s.NoError(err)

	// Flush the zone
	err = bp.flushZone(ctx, "test")
	s.NoError(err)

	// Verify items were marked as published
	var publishedCount int64
	s.db.Raw("SELECT COUNT(*) FROM dns_change_queue WHERE published_at IS NOT NULL").Scan(&publishedCount)
	s.Equal(int64(2), publishedCount)

	// Verify journal entries were created
	var journalCount int64
	s.db.Raw("SELECT COUNT(*) FROM dns_zone_journal WHERE zone_name = 'test'").Scan(&journalCount)
	s.Equal(int64(2), journalCount)

	// Verify they all have the same serial
	var serials []int64
	s.db.Raw("SELECT DISTINCT serial FROM dns_zone_journal WHERE zone_name = 'test'").Scan(&serials)
	s.Equal(1, len(serials), "All changes should have the same serial")
}

func (s *BatchPublisherTestSuite) TestStartStop() {
	bp := NewBatchPublisher(s.db, &BatchPublisherConfig{
		BatchInterval: 100 * time.Millisecond,
		MaxBatchSize:  100,
	})

	// Start the worker
	err := bp.Start()
	s.NoError(err)

	// Verify it's running
	bp.mu.Lock()
	s.True(bp.running)
	bp.mu.Unlock()

	// Try to start again (should error)
	err = bp.Start()
	s.Error(err)

	// Stop the worker
	err = bp.Stop()
	s.NoError(err)

	// Verify it's stopped
	bp.mu.Lock()
	s.False(bp.running)
	bp.mu.Unlock()

	// Stop again (should be no-op)
	err = bp.Stop()
	s.NoError(err)
}

func (s *BatchPublisherTestSuite) TestValidationErrors() {
	ctx := context.Background()

	bp := NewBatchPublisher(s.db, nil)

	tests := []struct {
		name    string
		change  *DNSChange
		wantErr bool
	}{
		{
			name: "missing zone name",
			change: &DNSChange{
				RecordName: "example.test.",
				RecordData: "ns1.example.com.",
			},
			wantErr: true,
		},
		{
			name: "missing record name",
			change: &DNSChange{
				ZoneName:   "test",
				RecordData: "ns1.example.com.",
			},
			wantErr: true,
		},
		{
			name: "missing record data",
			change: &DNSChange{
				ZoneName:   "test",
				RecordName: "example.test.",
			},
			wantErr: true,
		},
		{
			name: "valid change",
			change: &DNSChange{
				ZoneName:   "test",
				ChangeType: DNSChangeTypeAdd,
				RecordType: DNSRecordTypeNS,
				RecordName: "example.test.",
				RecordData: "ns1.example.com.",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			err := bp.QueueChange(ctx, tt.change)
			if tt.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *BatchPublisherTestSuite) TestDefaultTTL() {
	ctx := context.Background()

	bp := NewBatchPublisher(s.db, nil)

	// Queue change without TTL
	change := &DNSChange{
		ZoneName:   "test",
		ChangeType: DNSChangeTypeAdd,
		RecordType: DNSRecordTypeNS,
		RecordName: "example.test.",
		RecordData: "ns1.example.com.",
		// TTL is 0 (not set)
	}

	err := bp.QueueChange(ctx, change)
	s.NoError(err)

	// Verify default TTL was set
	var ttl int64
	s.db.Raw("SELECT ttl FROM dns_change_queue LIMIT 1").Scan(&ttl)
	s.Equal(int64(3600), ttl)
}

func (s *BatchPublisherTestSuite) TestGetQueueStats() {
	ctx := context.Background()

	bp := NewBatchPublisher(s.db, nil)

	// Queue some changes
	changes := []*DNSChange{
		{
			ZoneName:   "test",
			ChangeType: DNSChangeTypeAdd,
			RecordType: DNSRecordTypeNS,
			RecordName: "domain1.test.",
			RecordData: "ns1.example.com.",
			TTL:        3600,
		},
		{
			ZoneName:   "test",
			ChangeType: DNSChangeTypeAdd,
			RecordType: DNSRecordTypeNS,
			RecordName: "domain2.test.",
			RecordData: "ns2.example.com.",
			TTL:        3600,
		},
	}

	err := bp.QueueChanges(ctx, changes)
	s.NoError(err)

	// Get stats
	stats, err := bp.GetQueueStats(ctx)
	s.NoError(err)
	s.NotNil(stats)

	// Should have one zone with 2 pending
	found := false
	for _, stat := range stats {
		if stat.ZoneName == "test" {
			s.Equal(int64(2), stat.PendingCount)
			s.Equal(int64(0), stat.PublishedCount)
			s.Equal(int64(0), stat.ErrorCount)
			found = true
			break
		}
	}
	s.True(found, "Should find stats for test zone")
}

func (s *BatchPublisherTestSuite) TestWorkerProcessing() {
	ctx := context.Background()

	// Create publisher with very short interval
	bp := NewBatchPublisher(s.db, &BatchPublisherConfig{
		BatchInterval: 100 * time.Millisecond,
		MaxBatchSize:  100,
	})

	// Start the worker
	err := bp.Start()
	s.NoError(err)
	defer bp.Stop()

	// Queue changes
	changes := []*DNSChange{
		{
			ZoneName:   "test",
			ChangeType: DNSChangeTypeAdd,
			RecordType: DNSRecordTypeNS,
			RecordName: "domain1.test.",
			RecordData: "ns1.example.com.",
			TTL:        3600,
		},
		{
			ZoneName:   "test",
			ChangeType: DNSChangeTypeAdd,
			RecordType: DNSRecordTypeNS,
			RecordName: "domain2.test.",
			RecordData: "ns2.example.com.",
			TTL:        3600,
		},
	}

	err = bp.QueueChanges(ctx, changes)
	s.NoError(err)

	// Wait for worker to process (should happen within 200ms)
	time.Sleep(300 * time.Millisecond)

	// Verify items were published
	var publishedCount int64
	s.db.Raw("SELECT COUNT(*) FROM dns_change_queue WHERE published_at IS NOT NULL").Scan(&publishedCount)
	s.Equal(int64(2), publishedCount)

	// Verify journal entries
	var journalCount int64
	s.db.Raw("SELECT COUNT(*) FROM dns_zone_journal WHERE zone_name = 'test'").Scan(&journalCount)
	s.Equal(int64(2), journalCount)
}

func (s *BatchPublisherTestSuite) TestCleanupPublished() {
	ctx := context.Background()

	bp := NewBatchPublisher(s.db, nil)

	// Queue and publish some changes
	changes := []*DNSChange{
		{
			ZoneName:   "test",
			ChangeType: DNSChangeTypeAdd,
			RecordType: DNSRecordTypeNS,
			RecordName: "domain1.test.",
			RecordData: "ns1.example.com.",
			TTL:        3600,
		},
	}

	err := bp.QueueChanges(ctx, changes)
	s.NoError(err)

	err = bp.flushZone(ctx, "test")
	s.NoError(err)

	// Manually set published_at to old date
	s.db.Exec("UPDATE dns_change_queue SET published_at = NOW() - INTERVAL '8 days'")

	// Cleanup with 7 day retention
	deleted, err := bp.CleanupPublished(ctx, 7)
	s.NoError(err)
	s.Equal(int64(1), deleted)

	// Should be deleted
	var count int64
	s.db.Raw("SELECT COUNT(*) FROM dns_change_queue").Scan(&count)
	s.Equal(int64(0), count)
}
