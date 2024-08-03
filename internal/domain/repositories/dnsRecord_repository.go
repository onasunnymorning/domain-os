package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
)

// DNSRecordRepository is the interface that wraps the basic DNS record repository methods
type DNSRecordRepository interface {
	Create(ctx context.Context, record *postgres.DNSRecord) (*postgres.DNSRecord, error)
	GetByZone(ctx context.Context, zone string) ([]*postgres.DNSRecord, error)
	Delete(ctx context.Context, id int) error
}
