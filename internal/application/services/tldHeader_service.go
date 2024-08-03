package services

import (
	"fmt"

	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// TLDHeaderService implements the TLDHeaderService interface
type TLDHeaderService struct {
	dnsRecRepo repositories.DNSRecordRepository
}

// NewTLDHeaderService returns a new TLDHeaderService
func NewTLDHeaderService(dnsRecRepo repositories.DNSRecordRepository) *TLDHeaderService {
	return &TLDHeaderService{
		dnsRecRepo: dnsRecRepo,
	}
}

// GetTLDHeader gets a TLD header
func (s *TLDHeaderService) GetTLDHeader(ctx context.Context, name string) (*entities.TLDHeader, error) {
	// Collect the DNSRecords for the TLD
	rec, err := s.dnsRecRepo.GetByZone(ctx, name)
	if err != nil {
		return nil, err
	}
	// Create our return object
	var tldHeader entities.TLDHeader

	// Convert them to dns.RR records
	for _, r := range rec {
		// Convert the DNSRecord to a dns.RR
		rr, err := r.ToRR()
		if err != nil {
			return nil, err
		}
		// Append the RR to the appropriate slice or set soa
		switch r.Type {
		case "SOA":
			s, ok := rr.(*dns.SOA)
			if !ok {
				return nil, fmt.Errorf("Error converting TLDHeader to string: RR is not a SOA record: %s" + rr.String())
			}
			tldHeader.Soa = *s
		case "NS":
			ns, ok := rr.(*dns.NS)
			if !ok {
				return nil, fmt.Errorf("Error converting TLDHeader to string: RR is not a NS record: %s" + rr.String())
			}
			tldHeader.Ns = append(tldHeader.Ns, *ns)
		case "A":
			_, ok := rr.(*dns.A)
			if !ok {
				return nil, fmt.Errorf("Error converting TLDHeader to string: RR is not an A record: %s" + rr.String())
			}
			tldHeader.Glue = append(tldHeader.Glue, rr)
		case "AAAA":
			_, ok := rr.(*dns.AAAA)
			if !ok {
				return nil, fmt.Errorf("Error converting TLDHeader to string: RR is not an AAAA record: %s" + rr.String())
			}
			tldHeader.Glue = append(tldHeader.Glue, rr)
		}
	}

	return &tldHeader, nil
}
