package services

import (
	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/application/mappers"
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
	// return 3 dummy records
	ns1, _ := mappers.ToDnsNS("domain1."+tld, "ns1.apexdns.com")
	ns2, _ := mappers.ToDnsNS("domain2."+tld, "ns2.apexdns.com")
	ns3, _ := mappers.ToDnsNS("domain3."+tld, "ns3.apexdns.com")

	return []dns.RR{
		ns1,
		ns2,
		ns3,
	}, nil
	// if s.DNSrepo == nil {
	// 	fmt.Println("DNSrepo is nil !!!!!!!!")
	// 	return nil, nil
	// }
	// response, err := s.DNSrepo.GetNSRecordsPerTLD(ctx, tld)
	// if err != nil {
	// 	return nil, err
	// }
	// return response, nil
}
