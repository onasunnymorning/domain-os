package postgres

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestToRR(t *testing.T) {
	tests := []struct {
		name        string
		record      TLDDNSRecord
		expected    dns.RR
		expectedErr error
	}{
		{
			name: "A record",
			record: TLDDNSRecord{
				Name: "www.example.com.",
				Type: "A",
				TTL:  3600,
				Data: func() string {
					data, _ := json.Marshal(ARecordData{Address: "192.0.2.1"})
					return string(data)
				}(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: &dns.A{
				Hdr: dns.RR_Header{
					Name:   "www.example.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				A: net.ParseIP("192.0.2.1"),
			},
		},
		{
			name: "A record with nil IP address",
			record: TLDDNSRecord{
				Name: "www.example.com.",
				Type: "A",
				TTL:  3600,
				Data: func() string {
					data, _ := json.Marshal(ARecordData{Address: ""})
					return string(data)
				}(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expectedErr: fmt.Errorf("invalid IP address: %s", ""),
		},
		{
			name: "AAAA record",
			record: TLDDNSRecord{
				Name: "www.example.com.",
				Type: "AAAA",
				TTL:  3600,
				Data: func() string {
					data, _ := json.Marshal(AAAARecordData{Address: "2001:db8::1"})
					return string(data)
				}(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   "www.example.com.",
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				AAAA: net.ParseIP("2001:db8::1"),
			},
		},
		{
			name: "AAAA record with nil IP address",
			record: TLDDNSRecord{
				Name: "www.example.com.",
				Type: "AAAA",
				TTL:  3600,
				Data: func() string {
					data, _ := json.Marshal(AAAARecordData{Address: ""})
					return string(data)
				}(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expectedErr: fmt.Errorf("invalid IP address: %s", ""),
		},
		{
			name: "TXT record",
			record: TLDDNSRecord{
				Name: "www.example.com.",
				Type: "TXT",
				TTL:  3600,
				Data: func() string {
					data, _ := json.Marshal(TXTRecordData{Text: "Hello world"})
					return string(data)
				}(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   "www.example.com.",
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Txt: []string{"Hello world"},
			},
		},
		{
			name: "TXT record without data",
			record: TLDDNSRecord{
				Name: "www.example.com.",
				Type: "TXT",
				TTL:  3600,
				Data: func() string {
					data, _ := json.Marshal(TXTRecordData{})
					return string(data)
				}(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expectedErr: fmt.Errorf("TXT record requires Text"),
		},
		{
			name: "MX record",
			record: TLDDNSRecord{
				Name:      "example.com.",
				Type:      "MX",
				TTL:       3600,
				Priority:  func() *uint16 { p := uint16(10); return &p }(),
				Target:    func() *string { s := "mail.example.com."; return &s }(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: &dns.MX{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeMX,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Preference: 10,
				Mx:         "mail.example.com.",
			},
		},
		// TODO Add more test cases for other DNS record types - unhappy paths
		{
			name: "CNAME record",
			record: TLDDNSRecord{
				Name:      "www.example.com.",
				Type:      "CNAME",
				TTL:       3600,
				Target:    func() *string { s := "example.com."; return &s }(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: &dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "www.example.com.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Target: "example.com.",
			},
		},
		{
			name: "NS record",
			record: TLDDNSRecord{
				Name:      "example.com.",
				Type:      "NS",
				TTL:       3600,
				Target:    func() *string { s := "ns1.example.com."; return &s }(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: &dns.NS{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Ns: "ns1.example.com.",
			},
		},
		{
			name: "PTR record",
			record: TLDDNSRecord{
				Name:      "1.2.0.192.in-addr.arpa.",
				Type:      "PTR",
				TTL:       3600,
				Target:    func() *string { s := "example.com."; return &s }(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: &dns.PTR{
				Hdr: dns.RR_Header{
					Name:   "1.2.0.192.in-addr.arpa.",
					Rrtype: dns.TypePTR,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Ptr: "example.com.",
			},
		},
		{
			name: "SRV record",
			record: TLDDNSRecord{
				Name:      "1.2.0.192.in-addr.arpa.",
				Type:      "SRV",
				TTL:       3600,
				Priority:  func() *uint16 { p := uint16(10); return &p }(),
				Weight:    func() *uint16 { p := uint16(5); return &p }(),
				Port:      func() *uint16 { p := uint16(5060); return &p }(),
				Target:    func() *string { s := "sipserver.example.com."; return &s }(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: &dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "1.2.0.192.in-addr.arpa.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Priority: 10,
				Weight:   5,
				Port:     5060,
				Target:   "sipserver.example.com.",
			},
		},
		{
			name: "SOA record",
			record: TLDDNSRecord{
				Name:      "example.com.",
				Type:      "SOA",
				TTL:       3600,
				Data:      `{"Ns":"ns1.example.com.","Mbox":"hostmaster.example.com.","Serial":20210101,"Refresh":3600,"Retry":600,"Expire":604800,"Minttl":3600}`,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: &dns.SOA{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeSOA,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Ns:      "ns1.example.com.",
				Mbox:    "hostmaster.example.com.",
				Serial:  20210101,
				Refresh: 3600,
				Retry:   600,
				Expire:  604800,
				Minttl:  3600,
			},
		},
		{
			name: "DS record",
			record: TLDDNSRecord{
				Name:      "example.com.",
				Type:      "DS",
				TTL:       3600,
				Data:      `{"KeyTag":12345,"Algorithm":8,"DigestType":1,"Digest":"49FD46E6C4B"}`,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: &dns.DS{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeDS,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				KeyTag:     12345,
				Algorithm:  8,
				DigestType: 1,
				Digest:     "49FD46E6C4B",
			},
		},
		{
			name: "DNSKEY record",
			record: TLDDNSRecord{
				Name:      "example.com.",
				Type:      "DNSKEY",
				TTL:       3600,
				Data:      `{"Flags":256,"Protocol":3,"Algorithm":8,"PublicKey":"AwEAAc3NzaC1lZDI1NTE5AAAAIbJ9"}`,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expected: &dns.DNSKEY{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeDNSKEY,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Flags:     256,
				Protocol:  3,
				Algorithm: 8,
				PublicKey: "AwEAAc3NzaC1lZDI1NTE5AAAAIbJ9",
			},
		},
		{
			name: "unsupported record type",
			record: TLDDNSRecord{
				Name: "1.2.0.192.in-addr.arpa.",
				Type: "UNSUPPORTED",
			},
			expectedErr: fmt.Errorf("unsupported record type: %s", "UNSUPPORTED"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr, err := tt.record.ToRR()
			if tt.expectedErr == nil {
				if !dns.IsDuplicate(rr, tt.expected) {
					t.Errorf("ToRR() = %v, want %v", rr, tt.expected)
				}
			} else {
				if err.Error() != tt.expectedErr.Error() {
					t.Errorf("ToRR() error = %v, want %v", err, tt.expectedErr)
				}
			}
		})
	}
}

func TestConvertRRToDNSRecord(t *testing.T) {
	tests := []struct {
		name     string
		rr       dns.RR
		expected TLDDNSRecord
	}{
		{
			name: "A record",
			rr: &dns.A{
				Hdr: dns.RR_Header{
					Name:   "www.example.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				A: net.ParseIP("192.0.2.1"),
			},
			expected: TLDDNSRecord{
				Name: "www.example.com.",
				Type: "A",
				TTL:  3600,
				Data: func() string {
					data, _ := json.Marshal(ARecordData{Address: "192.0.2.1"})
					return string(data)
				}(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		{
			name: "AAAA record",
			rr: &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   "www.example.com.",
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				AAAA: net.ParseIP("2001:db8::1"),
			},
			expected: TLDDNSRecord{
				Name: "www.example.com.",
				Type: "AAAA",
				TTL:  3600,
				Data: func() string {
					data, _ := json.Marshal(AAAARecordData{Address: "2001:db8::1"})
					return string(data)
				}(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		{
			name: "TXT record",
			rr: &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   "www.example.com.",
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Txt: []string{"Hello world"},
			},
			expected: TLDDNSRecord{
				Name: "www.example.com.",
				Type: "TXT",
				TTL:  3600,
				Data: func() string {
					data, _ := json.Marshal(TXTRecordData{Text: "Hello world"})
					return string(data)
				}(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		{
			name: "MX record",
			rr: &dns.MX{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeMX,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Preference: 10,
				Mx:         "mail.example.com.",
			},
			expected: TLDDNSRecord{
				Name:      "example.com.",
				Type:      "MX",
				TTL:       3600,
				Priority:  func() *uint16 { p := uint16(10); return &p }(),
				Target:    func() *string { s := "mail.example.com."; return &s }(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		{
			name: "CNAME record",
			rr: &dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "www.example.com.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Target: "example.com.",
			},
			expected: TLDDNSRecord{
				Name:      "www.example.com.",
				Type:      "CNAME",
				TTL:       3600,
				Target:    func() *string { s := "example.com."; return &s }(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		{
			name: "NS record",
			rr: &dns.NS{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Ns: "ns1.example.com.",
			},
			expected: TLDDNSRecord{
				Name:      "example.com.",
				Type:      "NS",
				TTL:       3600,
				Target:    func() *string { s := "ns1.example.com."; return &s }(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		{
			name: "PTR record",
			rr: &dns.PTR{
				Hdr: dns.RR_Header{
					Name:   "1.2.0.192.in-addr.arpa.",
					Rrtype: dns.TypePTR,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Ptr: "example.com.",
			},
			expected: TLDDNSRecord{
				Name:      "1.2.0.192.in-addr.arpa.",
				Type:      "PTR",
				TTL:       3600,
				Target:    func() *string { s := "example.com."; return &s }(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		{
			name: "SRV record",
			rr: &dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "_sip._tcp.example.com.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Priority: 10,
				Weight:   5,
				Port:     5060,
				Target:   "sipserver.example.com.",
			},
			expected: TLDDNSRecord{
				Name:      "_sip._tcp.example.com.",
				Type:      "SRV",
				TTL:       3600,
				Priority:  func() *uint16 { p := uint16(10); return &p }(),
				Weight:    func() *uint16 { p := uint16(5); return &p }(),
				Port:      func() *uint16 { p := uint16(5060); return &p }(),
				Target:    func() *string { s := "sipserver.example.com."; return &s }(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := ConvertRRToDNSRecord(tt.rr)
			if err != nil {
				t.Fatalf("ConvertRRToDNSRecord() error = %v", err)
			}

			if record.Name != tt.expected.Name ||
				record.Type != tt.expected.Type ||
				record.TTL != tt.expected.TTL ||
				record.Data != tt.expected.Data {
				t.Errorf("ConvertRRToDNSRecord() = %+v, want %+v", record, tt.expected)
			}

			if record.Priority != nil && tt.expected.Priority != nil {
				if *record.Priority != *tt.expected.Priority {
					t.Errorf("ConvertRRToDNSRecord() Priority = %v, want %v", *record.Priority, *tt.expected.Priority)
				}
			}

			if record.Target != nil && tt.expected.Target != nil {
				if *record.Target != *tt.expected.Target {
					t.Errorf("ConvertRRToDNSRecord() Target = %v, want %v", *record.Target, *tt.expected.Target)
				}
			}

			if record.Weight != nil && tt.expected.Weight != nil {
				if *record.Weight != *tt.expected.Weight {
					t.Errorf("ConvertRRToDNSRecord() Weight = %v, want %v", *record.Weight, *tt.expected.Weight)
				}
			}

			if record.Port != nil && tt.expected.Port != nil {
				if *record.Port != *tt.expected.Port {
					t.Errorf("ConvertRRToDNSRecord() Port = %v, want %v", *record.Port, *tt.expected.Port)
				}
			}
		})
	}
}
