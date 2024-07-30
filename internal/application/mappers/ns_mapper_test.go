package mappers

import (
	"testing"

	"github.com/miekg/dns"
)

func TestToDnsNS(t *testing.T) {
	tc := []struct {
		name     string
		domain   string
		ns       string
		expected dns.RR
	}{
		{
			name:     "valid",
			domain:   "windy.domains",
			ns:       "ns1.windy.domains",
			expected: &dns.NS{Hdr: dns.RR_Header{Name: "windy.domains.", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600}, Ns: "ns1.windy.domains."},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ToDnsNS(tt.domain, tt.ns)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if actual.String() != tt.expected.String() {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}
