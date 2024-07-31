package repositories

import (
	"context"

	"github.com/miekg/dns"
)

// DNSRepository is the interface for the DNS repository
type DNSRepository interface {
	GetActiveDomainsWithHosts(ctx context.Context, tld string) ([]dns.RR, error)
	GetGlueForActiveDomains(ctx context.Context, tld string) ([]dns.RR, error)
}
