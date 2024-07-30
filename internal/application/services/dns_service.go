package services

import (
	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/application/mappers"
)

// DNSService implements the DNSInterface
type DNSService struct {
}

// NewDNSService creates a new DNSService
func NewDNSService() *DNSService {
	return &DNSService{}
}

// GetNSRecordsPerTLD gets NS records for a TLD
func (s *DNSService) GetNSRecordsPerTLD(tld string) ([]dns.RR, error) {
	// return 3 dummy records
	ns1, _ := mappers.ToDnsNS("domain1."+tld, "ns1.apexdns.com")
	ns2, _ := mappers.ToDnsNS("domain2."+tld, "ns2.apexdns.com")
	ns3, _ := mappers.ToDnsNS("domain3."+tld, "ns3.apexdns.com")

	return []dns.RR{
		ns1,
		ns2,
		ns3,
	}, nil
}
