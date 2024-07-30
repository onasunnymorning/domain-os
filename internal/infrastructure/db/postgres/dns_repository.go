package postgres

import (
	"context"

	"gorm.io/gorm"
)

// NSRecord represent the delegation records as present in the repository
type NSRecord struct {
	DomainName  string
	Type        string
	Class       string
	TTL         int
	Target      string
	InBailiwick bool
}

// DNSRepository is the GORM implementation of the DNSRepository
type DNSRepository struct {
	db *gorm.DB
}

// NewDNSRepository creates a new DNSRepository instance
func NewDNSRepository(db *gorm.DB) *DNSRepository {
	return &DNSRepository{
		db: db,
	}
}

// GetNSRecordsPerTLD gets the NS records for a given TLD
func (r *DNSRepository) GetNSRecordsPerTLD(ctx context.Context, tld string) ([]*NSRecord, error) {
	var nsRecords []*NSRecord
	err := r.db.Raw(`
		SELECT dom.name AS domain, ho.name AS host, ho.in_bailiwick
		FROM public.domains dom
		LEFT JOIN domain_hosts dh ON dh.domain_ro_id = dom.ro_id
		LEFT JOIN hosts ho ON dh.host_ro_id = ho.ro_id
	`).Scan(&nsRecords).Error
	if err != nil {
		return nil, err
	}

	return nsRecords, nil
}
