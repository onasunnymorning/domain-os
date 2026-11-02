package postgres

import (
	"encoding/json"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type DomainEventRecord struct {
	ID            string    `gorm:"primaryKey;type:uuid"`
	Source        string    `gorm:"not null"`
	Type          string    `gorm:"not null"`
	Subject       string    `gorm:"not null;index:idx_events_subject_occurred,priority:1"`
	Description   string    `gorm:"type:text"`
	OccurredAt    time.Time `gorm:"not null;index:idx_events_subject_occurred,priority:2,sort:desc"`
	TraceID       string
	CorrelationID string
	Data          []byte    `gorm:"type:jsonb;not null"`
	Command       []byte    `gorm:"type:jsonb"`
	BeforeState   []byte    `gorm:"type:jsonb"`
	AfterState    []byte    `gorm:"type:jsonb"`
	Actor         string    `gorm:"type:text"`
	RoID          string    `gorm:"type:text"`
	Published     bool      `gorm:"default:false"` // outbox relay flag; indexed where false, see AutoMigrate
	CreatedAt     time.Time
}

func (DomainEventRecord) TableName() string {
	return "domain_events"
}

// ToDBDomainEvent encodes an event as an outbox row. Every payload — Data,
// Command, BeforeState and AfterState — goes through redactSecrets, so no
// authInfo reaches domain_events or, via the relay, the S3 archive. This is the
// only path into domain_events; keep it that way.
func ToDBDomainEvent(e entities.DomainEvent) (DomainEventRecord, error) {
	dataBytes, err := redactSecrets(e.Data)
	if err != nil {
		return DomainEventRecord{}, err
	}
	var commandBytes, beforeBytes, afterBytes []byte
	if e.Command != nil {
		commandBytes, err = redactSecrets(e.Command)
		if err != nil {
			return DomainEventRecord{}, err
		}
	}
	if e.BeforeState != nil {
		beforeBytes, err = redactSecrets(e.BeforeState)
		if err != nil {
			return DomainEventRecord{}, err
		}
	}
	if e.AfterState != nil {
		afterBytes, err = redactSecrets(e.AfterState)
		if err != nil {
			return DomainEventRecord{}, err
		}
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
		Command:       commandBytes,
		BeforeState:   beforeBytes,
		AfterState:    afterBytes,
		Actor:         e.Actor,
		RoID:          e.RoID,
		Published:     false,
	}, nil
}

func (r DomainEventRecord) ToDomainEvent() (entities.DomainEvent, error) {
	var rawData, rawCommand, rawBefore, rawAfter json.RawMessage
	if err := json.Unmarshal(r.Data, &rawData); err != nil {
		return entities.DomainEvent{}, err
	}
	if len(r.Command) > 0 {
		if err := json.Unmarshal(r.Command, &rawCommand); err != nil {
			return entities.DomainEvent{}, err
		}
	}
	if len(r.BeforeState) > 0 {
		if err := json.Unmarshal(r.BeforeState, &rawBefore); err != nil {
			return entities.DomainEvent{}, err
		}
	}
	if len(r.AfterState) > 0 {
		if err := json.Unmarshal(r.AfterState, &rawAfter); err != nil {
			return entities.DomainEvent{}, err
		}
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
		Data:          rawData,
		Command:       rawCommand,
		BeforeState:   rawBefore,
		AfterState:    rawAfter,
		Actor:         r.Actor,
		RoID:          r.RoID,
	}, nil
}
