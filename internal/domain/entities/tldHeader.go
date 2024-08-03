package entities

import "github.com/miekg/dns"

// TLDHeader represents the header of a TLD zone; includes the SOA, NS and Glue records
type TLDHeader struct {
	Soa    dns.SOA
	Ns     []dns.NS
	Glue   []dns.RR
	Ds     []dns.DS
	DNSKey []dns.DNSKEY
}

// String returns a string representation of the TLDHeader
func (t *TLDHeader) String() string {

	var ns string
	for _, rr := range t.Ns {
		ns += rr.String() + "\n"
	}

	var glue string
	for _, rr := range t.Glue {
		glue += rr.String() + "\n"
	}

	var ds string
	for _, rr := range t.Ds {
		ds += rr.String() + "\n"
	}

	var dnskey string
	for _, rr := range t.DNSKey {
		dnskey += rr.String() + "\n"
	}

	return t.Soa.String() + "\n" + ns + glue + ds + dnskey
}
