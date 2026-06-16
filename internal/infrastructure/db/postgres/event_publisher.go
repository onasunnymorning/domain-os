package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PostgresEventPublisher struct {
	db        *gorm.DB
	logger    *zap.Logger
	logEvents bool
}

func NewPostgresEventPublisher(db *gorm.DB, logger *zap.Logger, logEvents bool) *PostgresEventPublisher {
	return &PostgresEventPublisher{
		db:        db,
		logger:    logger,
		logEvents: logEvents,
	}
}

func (p *PostgresEventPublisher) Publish(ctx context.Context, events ...entities.DomainEvent) error {
	records := make([]DomainEventRecord, len(events))
	for i, e := range events {
		record, err := ToDBDomainEvent(e)
		if err != nil {
			return err
		}
		records[i] = record

		// Toggleable structured logging
		if p.logEvents {
			p.logger.Info(e.Type,
				zap.String("event_id", e.ID),
				zap.String("subject", e.Subject),
				zap.String("source", e.Source),
				zap.String("trace_id", e.TraceID),
				zap.String("correlation_id", e.CorrelationID),
				zap.Any("data", e.Data),
			)
		}
	}
	return p.db.WithContext(ctx).Create(&records).Error
}
