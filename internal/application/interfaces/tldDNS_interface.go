package interfaces

import (
	"context"

	"github.com/miekg/dns"
)

// TLDDNSService is the interface for the DNS service
type TLDDNSService interface {
	GetNSRecordsPerTLD(ctx context.Context, tld string) ([]*dns.RR, error)
	GetGlueRecordsPerTLD(ctx context.Context, tld string) ([]dns.RR, error)
}
