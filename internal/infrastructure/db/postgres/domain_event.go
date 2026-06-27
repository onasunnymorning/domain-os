package postgres

import (
	"encoding/json"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type DomainEventRecord struct {
	ID            string    `gorm:"primaryKey;type:uuid"`
	Source        string    `gorm:"not null;index"`
	Type          string    `gorm:"not null;index"`
	Subject       string    `gorm:"not null;index"`
	Description   string    `gorm:"type:text"`
	OccurredAt    time.Time `gorm:"not null;index"`
	TraceID       string    `gorm:"index"`
	CorrelationID string    `gorm:"index"`
	Data          []byte    `gorm:"type:jsonb;not null"`
	Published     bool      `gorm:"default:false;index"` // outbox relay flag
	CreatedAt     time.Time
}

func (DomainEventRecord) TableName() string {
	return "domain_events"
}

func ToDBDomainEvent(e entities.DomainEvent) (DomainEventRecord, error) {
	dataBytes, err := json.Marshal(e.Data)
	if err != nil {
		return DomainEventRecord{}, err
	}
	return DomainEventRecord{
		ID:            e.ID,
		Source:        e.Source,
		Type:          e.Type,
		Subject:       e.Subject,
		Description:   e.Description,
		OccurredAt:    e.Time,
		TraceID:       e.TraceID,
		CorrelationID: e.CorrelationID,
		Data:          dataBytes,
		Published:     false,
	}, nil
}

func (r DomainEventRecord) ToDomainEvent() (entities.DomainEvent, error) {
	var raw json.RawMessage
	if err := json.Unmarshal(r.Data, &raw); err != nil {
		return entities.DomainEvent{}, err
	}
	return entities.DomainEvent{
		ID:            r.ID,
		Source:        r.Source,
		Type:          r.Type,
		Subject:       r.Subject,
		Description:   r.Description,
		Time:          r.OccurredAt,
		TraceID:       r.TraceID,
		CorrelationID: r.CorrelationID,
		Data:          raw,
	}, nil
}
