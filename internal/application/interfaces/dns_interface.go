package interfaces

import "github.com/miekg/dns"

// DNSInterface is the interface for the DNS service
type DNSInterface interface {
	GetNSRecordsPerTLD(tld string) ([]*dns.RR, error)
}
