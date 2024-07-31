package repositories

import (
	"context"

	"github.com/miekg/dns"
)

// DNSRepository is the interface for the DNS repository
type DNSRepository interface {
	GetNSRecordsPerTLD(ctx context.Context, tld string) ([]dns.RR, error)
}
