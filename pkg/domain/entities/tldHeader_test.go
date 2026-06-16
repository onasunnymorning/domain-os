package entities

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestTLDHeader_String(t *testing.T) {
	h := TLDHeader{
		Soa: dns.SOA{
			Hdr: dns.RR_Header{
				Name:   "example.com.",
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			Ns:      "ns1.example.com.",
			Mbox:    "hostmaster.example.com.",
			Serial:  2021010101,
			Refresh: 3600,
			Retry:   600,
			Expire:  604800,
			Minttl:  60,
		},
		Ns: []dns.NS{
			{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Ns: "ns1.example.com.",
			},
			{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Ns: "ns2.example.net.",
			},
		},
		Glue: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ns1.example.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				A: net.ParseIP("10.0.0.1"),
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ns1.example.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				A: net.ParseIP("192.168.0.1"),
			},
		},
		Ds: []dns.DS{
			{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeDS,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				KeyTag:     12345,
				Algorithm:  8,
				DigestType: 1,
				Digest:     "MY DIGEST 1",
			},
			{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeDS,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				KeyTag:     12345,
				Algorithm:  8,
				DigestType: 2,
				Digest:     "MY DIGEST 2",
			},
		},
		DNSKey: []dns.DNSKEY{
			{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeDNSKEY,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Flags:     256,
				Protocol:  3,
				Algorithm: 8,
				PublicKey: "MY PUBLIC KEY 8",
			},
			{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeDNSKEY,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Flags:     256,
				Protocol:  3,
				Algorithm: 13,
				PublicKey: "MY PUBLIC KEY 13",
			},
		},
	}

	t.Run("TestTLDHeader_String", func(t *testing.T) {
		t.Parallel()

		got := h.String()
		want := "example.com.\t3600\tIN\tSOA\tns1.example.com. hostmaster.example.com. 2021010101 3600 600 604800 60\nexample.com.\t3600\tIN\tNS\tns1.example.com.\nexample.com.\t3600\tIN\tNS\tns2.example.net.\nns1.example.com.\t3600\tIN\tA\t10.0.0.1\nns1.example.com.\t3600\tIN\tA\t192.168.0.1\nexample.com.\t3600\tIN\tDS\t12345 8 1 MY DIGEST 1\nexample.com.\t3600\tIN\tDS\t12345 8 2 MY DIGEST 2\nexample.com.\t3600\tIN\tDNSKEY\t256 3 8 MY PUBLIC KEY 8\nexample.com.\t3600\tIN\tDNSKEY\t256 3 13 MY PUBLIC KEY 13\n"

		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

}
