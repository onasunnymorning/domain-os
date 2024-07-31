package postgres

import (
	"context"
	"fmt"

	"github.com/miekg/dns"
	"gorm.io/gorm"
)

// NSQueryResult is the response object for GetNSRecordsPerTLD Query
type NSQueryResult struct {
	Domain string
	Host   string
}

// GlueQueryResult is the response object for GetGlueRecords Query
type GlueQueryResult struct {
	Host    string
	Address string
	Version int
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

// GetActiveDomainsWithHosts gets the NS records for a given TLD
func (r *DNSRepository) GetActiveDomainsWithHosts(ctx context.Context, tld string) ([]dns.RR, error) {
	var queryResults []NSQueryResult
	err := r.db.Raw(`
		SELECT dom.name AS domain, ho.name AS host
		FROM public.domains dom
		LEFT JOIN domain_hosts dh ON dh.domain_ro_id = dom.ro_id
		LEFT JOIN hosts ho ON dh.host_ro_id = ho.ro_id
		WHERE dom.tld_name = ?
		AND dom.inactive = false
	`, tld).Scan(&queryResults).Error
	if err != nil {
		return nil, err
	}

	// Convert to DNS NS
	response := make([]dns.RR, len(queryResults))
	for i, result := range queryResults {
		ns, err := dns.NewRR(fmt.Sprintf("%s. 3600 IN NS %s", result.Domain, result.Host))
		if err != nil {
			return nil, err
		}
		response[i] = ns
	}

	return response, nil
}

// GetGlueForActiveDomains gets the glue records for a given TLD
func (r *DNSRepository) GetGlueForActiveDomains(ctx context.Context, tld string) ([]dns.RR, error) {
	var queryResults []GlueQueryResult
	err := r.db.Raw(`
		SELECT ho.name AS host, address, version
		FROM public.domains dom
		LEFT JOIN domain_hosts dh ON dh.domain_ro_id = dom.ro_id
		LEFT JOIN hosts ho ON dh.host_ro_id = ho.ro_id
		LEFT JOIN host_addresses ha ON ho.ro_id = ha.host_ro_id 
		WHERE dom.tld_name = ?
		AND dom.inactive = false
		AND ho.in_bailiwick = true
	`, tld).Scan(&queryResults).Error
	if err != nil {
		return nil, err
	}

	// Convert to DNS A or AAAA
	response := make([]dns.RR, len(queryResults))
	for i, result := range queryResults {
		t := "A"
		if result.Version == 6 {
			t = "AAAA"
		}
		rr, err := dns.NewRR(fmt.Sprintf("%s. 3600 IN %s %s", result.Host, t, result.Address))
		if err != nil {
			return nil, err
		}
		response[i] = rr

	}

	return response, nil
}
