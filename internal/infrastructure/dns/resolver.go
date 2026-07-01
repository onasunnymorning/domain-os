package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	mdns "github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

var (
	ErrNXDomain = errors.New("NXDOMAIN: zone does not exist")
	ErrTimeout  = errors.New("DNS query timed out")
	ErrRefused  = errors.New("DNS query refused")
	ErrNoSOA    = errors.New("no SOA record in response")
)

// Resolver implements repositories.DNSResolver using github.com/miekg/dns.
type Resolver struct {
	Timeout time.Duration
}

// Ensure Resolver implements DNSResolver.
var _ repositories.DNSResolver = (*Resolver)(nil)

// NewResolver creates a DNS resolver with the given timeout.
// If timeout is 0, defaults to 5 seconds.
func NewResolver(timeout time.Duration) *Resolver {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &Resolver{Timeout: timeout}
}

// QuerySOA sends a SOA query for the given zone to the given nameserver
// and returns the parsed SOA record fields.
func (r *Resolver) QuerySOA(ctx context.Context, zone string, nameserver string) (*repositories.SOAResult, error) {
	msg := new(mdns.Msg)
	msg.SetQuestion(mdns.Fqdn(zone), mdns.TypeSOA)
	msg.RecursionDesired = false

	// Ensure nameserver has port
	if _, _, err := net.SplitHostPort(nameserver); err != nil {
		nameserver = net.JoinHostPort(nameserver, "53")
	}

	// Try UDP first
	client := &mdns.Client{Net: "udp", Timeout: r.Timeout}
	resp, _, err := client.ExchangeContext(ctx, msg, nameserver)
	if err != nil {
		if isTimeout(err) {
			return nil, fmt.Errorf("%w: %s", ErrTimeout, nameserver)
		}
		return nil, fmt.Errorf("DNS query to %s: %w", nameserver, err)
	}

	// Retry with TCP if truncated
	if resp.Truncated {
		client = &mdns.Client{Net: "tcp", Timeout: r.Timeout}
		resp, _, err = client.ExchangeContext(ctx, msg, nameserver)
		if err != nil {
			return nil, fmt.Errorf("DNS TCP retry to %s: %w", nameserver, err)
		}
	}

	// Check response code
	switch resp.Rcode {
	case mdns.RcodeSuccess:
		// continue
	case mdns.RcodeNameError:
		return nil, fmt.Errorf("%w: %s", ErrNXDomain, zone)
	case mdns.RcodeRefused:
		return nil, fmt.Errorf("%w: %s", ErrRefused, nameserver)
	default:
		return nil, fmt.Errorf("DNS query to %s returned rcode %d (%s)",
			nameserver, resp.Rcode, mdns.RcodeToString[resp.Rcode])
	}

	// Search answer section first, then authority
	for _, sections := range [][]mdns.RR{resp.Answer, resp.Ns} {
		for _, rr := range sections {
			if soa, ok := rr.(*mdns.SOA); ok {
				return &repositories.SOAResult{
					Serial:  soa.Serial,
					Refresh: soa.Refresh,
					Retry:   soa.Retry,
					Expire:  soa.Expire,
					Minttl:  soa.Minttl,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("%w: %s from %s", ErrNoSOA, zone, nameserver)
}

func isTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
