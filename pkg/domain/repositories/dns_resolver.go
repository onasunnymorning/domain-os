package repositories

import "context"

// SOAResult holds the fields from a DNS SOA record response.
type SOAResult struct {
	Serial  uint32 `json:"serial"`
	Refresh uint32 `json:"refresh"`
	Retry   uint32 `json:"retry"`
	Expire  uint32 `json:"expire"`
	Minttl  uint32 `json:"minttl"`
}

// DNSResolver abstracts DNS SOA queries for testability.
type DNSResolver interface {
	// QuerySOA sends a SOA query for the given zone to the given nameserver
	// and returns the parsed SOA record fields.
	QuerySOA(ctx context.Context, zone string, nameserver string) (*SOAResult, error)
}
