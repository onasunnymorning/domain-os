package interfaces

import (
	"context"

	"github.com/miekg/dns"
)

// DNSService is the interface for the DNS service
type DNSService interface {
	GetNSRecordsPerTLD(ctx context.Context, tld string) ([]*dns.RR, error)
	GetGlueRecordsPerTLD(ctx context.Context, tld string) ([]dns.RR, error)
}
