package services

import (
	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// DNSService implements the DNSInterface
type DNSService struct {
	DNSrepo repositories.DNSRepository
}

// NewDNSService creates a new DNSService
func NewDNSService(dr repositories.DNSRepository) *DNSService {
	return &DNSService{
		DNSrepo: dr,
	}
}

// GetNSRecordsPerTLD gets NS records for a TLD
func (s *DNSService) GetNSRecordsPerTLD(ctx context.Context, tld string) ([]dns.RR, error) {
	response, err := s.DNSrepo.GetActiveDomainsWithHosts(ctx, tld)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// GetGlueRecordsPerTLD gets Glue records for a TLD
func (s *DNSService) GetGlueRecordsPerTLD(ctx context.Context, tld string) ([]dns.RR, error) {
	response, err := s.DNSrepo.GetGlueForActiveDomains(ctx, tld)
	if err != nil {
		return nil, err
	}
	return response, nil
}
