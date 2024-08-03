package postgres

import (
	"context"

	"gorm.io/gorm"
)

// DNSRecordRepository implements the DNSRecordRepository interface
type DNSRecordRepository struct {
	db *gorm.DB
}

// NewGormDNSRecordRepository returns a new DNSRecordRepository using Gorm
func NewGormDNSRecordRepository(db *gorm.DB) *DNSRecordRepository {
	return &DNSRecordRepository{db}
}

// Create creates a new DNS record in the database
func (r *DNSRecordRepository) Create(ctx context.Context, record *DNSRecord) (*DNSRecord, error) {
	err := r.db.WithContext(ctx).Create(record).Error
	if err != nil {
		return nil, err
	}
	return record, nil
}
