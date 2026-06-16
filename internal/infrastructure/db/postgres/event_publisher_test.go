package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
)

type mockCore struct {
	writeCount int
}

func (m *mockCore) Enabled(lvl zapcore.Level) bool {
	return true
}

func (m *mockCore) With(fields []zapcore.Field) zapcore.Core {
	return m
}

func (m *mockCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(ent, m)
}

func (m *mockCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	m.writeCount++
	return nil
}

func (m *mockCore) Sync() error {
	return nil
}

type EventPublisherSuite struct {
	suite.Suite
	db *gorm.DB
}

func TestEventPublisherSuite(t *testing.T) {
	suite.Run(t, new(EventPublisherSuite))
}

func (s *EventPublisherSuite) SetupSuite() {
	s.db = setupTestDB()
}

func (s *EventPublisherSuite) TestPublish() {
	core := &mockCore{}
	logger := zap.New(core)

	pub := NewPostgresEventPublisher(s.db, logger, true)

	data := map[string]string{"key": "value"}
	event := entities.NewDomainEvent("test-source", "domain.registered", "test-domain.com", data)
	event.TraceID = "trace-123"
	event.CorrelationID = "corr-456"

	err := pub.Publish(context.Background(), event)
	s.Require().NoError(err)

	// Verify logged
	s.Require().Equal(1, core.writeCount)

	// Verify database record
	var record DomainEventRecord
	err = s.db.Where("id = ?", event.ID).First(&record).Error
	s.Require().NoError(err)

	s.Equal("test-source", record.Source)
	s.Equal("domain.registered", record.Type)
	s.Equal("test-domain.com", record.Subject)
	s.Equal("trace-123", record.TraceID)
	s.Equal("corr-456", record.CorrelationID)
	s.False(record.Published)

	// Test mapping back
	mappedEvent, err := record.ToDomainEvent()
	s.Require().NoError(err)
	s.Equal(event.ID, mappedEvent.ID)
	s.Equal(event.Source, mappedEvent.Source)
	s.Equal(event.Type, mappedEvent.Type)
	s.Equal(event.Subject, mappedEvent.Subject)
	s.Equal(event.TraceID, mappedEvent.TraceID)
	s.Equal(event.CorrelationID, mappedEvent.CorrelationID)

	// Toggle logEvents = false
	core2 := &mockCore{}
	logger2 := zap.New(core2)
	pubDisabledLogs := NewPostgresEventPublisher(s.db, logger2, false)

	event2 := entities.NewDomainEvent("test-source", "domain.renewed", "test-domain2.com", data)
	err = pubDisabledLogs.Publish(context.Background(), event2)
	s.Require().NoError(err)
	s.Require().Equal(0, core2.writeCount) // No logs generated!
}
