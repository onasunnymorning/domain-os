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
func (r *DNSRecordRepository) Create(ctx context.Context, record *TLDDNSRecord) (*TLDDNSRecord, error) {
	err := r.db.WithContext(ctx).Create(record).Error
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetByZone returns all DNS records for a given zone
func (r *DNSRecordRepository) GetByZone(ctx context.Context, zone string) ([]*TLDDNSRecord, error) {
	var records []*TLDDNSRecord
	err := r.db.WithContext(ctx).Where("zone = ?", zone).Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

// Delete deletes a DNS record from the database
func (r *DNSRecordRepository) Delete(ctx context.Context, id int) error {
	err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&TLDDNSRecord{}).Error
	if err != nil {
		return err
	}
	return nil
}
