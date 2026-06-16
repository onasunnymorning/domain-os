package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
)

// TLDDNSRecordRepository is the interface that wraps the basic DNS record repository methods
type TLDDNSRecordRepository interface {
	Create(ctx context.Context, record *postgres.TLDDNSRecord) (*postgres.TLDDNSRecord, error)
	GetByZone(ctx context.Context, zone string) ([]*postgres.TLDDNSRecord, error)
	Delete(ctx context.Context, id int) error
}
