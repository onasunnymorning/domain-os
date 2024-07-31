package postgres

import (
	"context"

	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/application/mappers"
	"gorm.io/gorm"
)

// GetNSRecordsPerTLDQueryResponse is the response object for GetNSRecordsPerTLD Query
type GetNSRecordsPerTLDQueryResponse struct {
	Domain      string
	Ns          string
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
func (r *DNSRepository) GetNSRecordsPerTLD(ctx context.Context, tld string) ([]dns.RR, error) {
	var queryResults []GetNSRecordsPerTLDQueryResponse
	err := r.db.Raw(`
		SELECT dom.name AS domain, ho.name AS host, ho.in_bailiwick
		FROM public.domains dom
		LEFT JOIN domain_hosts dh ON dh.domain_ro_id = dom.ro_id
		LEFT JOIN hosts ho ON dh.host_ro_id = ho.ro_id
	`).Scan(&queryResults).Error
	if err != nil {
		return nil, err
	}

	// Convert to DNS NS
	response := make([]dns.RR, len(queryResults))
	for i, result := range queryResults {
		ns, err := mappers.ToDnsNS(result.Domain, result.Ns)
		if err != nil {
			return nil, err
		}
		response[i] = ns
	}

	return response, nil
}
