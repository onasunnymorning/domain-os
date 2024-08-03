package postgres

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
)

// TLDDNSRecord represents a DNS record in the database
type TLDDNSRecord struct {
	ID        int       `json:"id"`
	Zone      string    `json:"zone" gorm:"index"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	TTL       uint32    `json:"ttl"`
	Data      string    `json:"data"` // JSON serialized data
	Priority  *uint16   `json:"priority,omitempty"`
	Weight    *uint16   `json:"weight,omitempty"`
	Port      *uint16   `json:"port,omitempty"`
	Target    *string   `json:"target,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName returns the table name for the DNSRecord model
func (TLDDNSRecord) TableName() string {
	return "tld_dns_records"
}

// ARecordData represents the data for an A record use it to marshal/unmarshal the data field in DNSRecord
type ARecordData struct {
	Address string `json:"address"`
}

// AAAARecordData represents the data for an AAAA record use it to marshal/unmarshal the data field in DNSRecord
type AAAARecordData struct {
	Address string `json:"address"`
}

// TXTRecordData represents the data for a TXT record use it to marshal/unmarshal the data field in DNSRecord
type TXTRecordData struct {
	Text string `json:"text"`
}

// MXRecordData represents the data for an MX record use it to marshal/unmarshal the data field in DNSRecord
type MXRecordData struct {
	Preference uint16 `json:"preference"`
	Exchange   string `json:"exchange"`
}

// NSRecordData represents the data for an NS record use it to marshal/unmarshal the data field in DNSRecord
type NSRecordData struct {
	Ns string `json:"ns"`
}

// PTRRecordData represents the data for a PTR record use it to marshal/unmarshal the data field in DNSRecord
type PTRRecordData struct {
	Ptr string `json:"ptr"`
}

// SRVRecordData represents the data for an SRV record use it to marshal/unmarshal the data field in DNSRecord
type SRVRecordData struct {
	Priority uint16 `json:"priority"`
	Weight   uint16 `json:"weight"`
	Port     uint16 `json:"port"`
	Target   string `json:"target"`
}

// CNAMERecordData represents the data for a CNAME record use it to marshal/unmarshal the data field in DNSRecord
type CNAMERecordData struct {
	Target string `json:"target"`
}

// SOARecordData represents the data for a SOA record use it to marshal/unmarshal the data field in DNSRecord
type SOARecordData struct {
	Ns      string `json:"ns"`
	Mbox    string `json:"mbox"`
	Serial  uint32 `json:"serial"`
	Refresh uint32 `json:"refresh"`
	Retry   uint32 `json:"retry"`
	Expire  uint32 `json:"expire"`
	Minttl  uint32 `json:"minttl"`
}

// DSRecordData represents the data for a DS record use it to marshal/unmarshal the data field in DNSRecord
type DSRecordData struct {
	KeyTag     uint16 `json:"keyTag"`
	Algorithm  uint8  `json:"algorithm"`
	DigestType uint8  `json:"digestType"`
	Digest     string `json:"digest"`
}

// DNSKEYRecordData represents the data for a DNSKEY record use it to marshal/unmarshal the data field in DNSRecord
type DNSKEYRecordData struct {
	Flags     uint16 `json:"flags"`
	Protocol  uint8  `json:"protocol"`
	Algorithm uint8  `json:"algorithm"`
	PublicKey string `json:"publicKey"`
}

// Convert DNSRecord to dns.RR
func (record *TLDDNSRecord) ToRR() (dns.RR, error) {
	header := dns.RR_Header{
		Name:   dns.Fqdn(record.Name),
		Rrtype: dns.StringToType[record.Type],
		Class:  dns.ClassINET,
		Ttl:    record.TTL,
	}

	switch record.Type {
	case "A":
		var aData ARecordData
		if err := json.Unmarshal([]byte(record.Data), &aData); err != nil {
			return nil, err
		}
		ip := net.ParseIP(aData.Address)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", aData.Address)
		}
		return &dns.A{
			Hdr: header,
			A:   ip,
		}, nil
	case "AAAA":
		var aaaaData AAAARecordData
		if err := json.Unmarshal([]byte(record.Data), &aaaaData); err != nil {
			return nil, err
		}
		ip := net.ParseIP(aaaaData.Address)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", aaaaData.Address)
		}
		return &dns.AAAA{
			Hdr:  header,
			AAAA: ip,
		}, nil
	case "TXT":
		var txtData TXTRecordData
		if err := json.Unmarshal([]byte(record.Data), &txtData); err != nil {
			return nil, err
		}
		// return an error if the text is empty
		if txtData.Text == "" {
			return nil, fmt.Errorf("TXT record requires Text")
		}
		return &dns.TXT{
			Hdr: header,
			Txt: []string{txtData.Text},
		}, nil
	case "MX":
		if record.Target == nil || record.Priority == nil {
			return nil, fmt.Errorf("MX record requires Target and Priority")
		}
		return &dns.MX{
			Hdr:        header,
			Preference: *record.Priority,
			Mx:         dns.Fqdn(*record.Target),
		}, nil
	case "SRV":
		if record.Target == nil || record.Priority == nil || record.Weight == nil || record.Port == nil {
			return nil, fmt.Errorf("SRV record requires Target, Priority, Weight, and Port")
		}
		return &dns.SRV{
			Hdr:      header,
			Priority: *record.Priority,
			Weight:   *record.Weight,
			Port:     *record.Port,
			Target:   dns.Fqdn(*record.Target),
		}, nil
	case "CNAME":
		if record.Target == nil {
			return nil, fmt.Errorf("CNAME record requires Target")
		}
		return &dns.CNAME{
			Hdr:    header,
			Target: dns.Fqdn(*record.Target),
		}, nil
	case "NS":
		if record.Target == nil {
			return nil, fmt.Errorf("NS record requires Target")
		}
		return &dns.NS{
			Hdr: header,
			Ns:  dns.Fqdn(*record.Target),
		}, nil
	case "PTR":
		if record.Target == nil {
			return nil, fmt.Errorf("PTR record requires Target")
		}
		return &dns.PTR{
			Hdr: header,
			Ptr: dns.Fqdn(*record.Target),
		}, nil
	case "SOA":
		// Unmarshal the SOA record data
		var soaData SOARecordData
		if err := json.Unmarshal([]byte(record.Data), &soaData); err != nil {
			return nil, err
		}

		// Create the SOA record
		soa := &dns.SOA{
			Hdr:     header,
			Ns:      dns.Fqdn(soaData.Ns),
			Mbox:    dns.Fqdn(soaData.Mbox),
			Serial:  soaData.Serial,
			Refresh: soaData.Refresh,
			Retry:   soaData.Retry,
			Expire:  soaData.Expire,
			Minttl:  soaData.Minttl,
		}

		return soa, nil
	case "DS":
		// Unmarshal the DS record data
		var dsData DSRecordData
		if err := json.Unmarshal([]byte(record.Data), &dsData); err != nil {
			return nil, err
		}

		// Create the DS record
		ds := &dns.DS{
			Hdr:        header,
			KeyTag:     dsData.KeyTag,
			Algorithm:  dsData.Algorithm,
			DigestType: dsData.DigestType,
			Digest:     dsData.Digest,
		}

		return ds, nil
	case "DNSKEY":
		// Unmarshal the DNSKEY record data
		var dnskeyData DNSKEYRecordData
		if err := json.Unmarshal([]byte(record.Data), &dnskeyData); err != nil {
			return nil, err
		}

		// Create the DNSKEY record
		dnskey := &dns.DNSKEY{
			Hdr:       header,
			Flags:     dnskeyData.Flags,
			Protocol:  dnskeyData.Protocol,
			Algorithm: dnskeyData.Algorithm,
			PublicKey: dnskeyData.PublicKey,
		}

		return dnskey, nil

	// Add more cases for other DNS record types as needed
	default:
		return nil, fmt.Errorf("unsupported record type: %s", record.Type)
	}
}

// ConvertRRToDNSRecord converts a dns.RR to a DNSRecord
func ConvertRRToDNSRecord(rr dns.RR) (*TLDDNSRecord, error) {
	header := rr.Header()

	record := &TLDDNSRecord{
		Name:      dns.Fqdn(header.Name),
		Type:      dns.TypeToString[header.Rrtype],
		TTL:       header.Ttl,
		CreatedAt: time.Now(), // Assume new record creation for this example
		UpdatedAt: time.Now(),
	}

	switch r := rr.(type) {
	case *dns.A:
		aData := ARecordData{
			Address: r.A.String(),
		}
		data, err := json.Marshal(aData)
		if err != nil {
			return nil, err
		}
		record.Data = string(data)
	case *dns.AAAA:
		aaaaData := AAAARecordData{
			Address: r.AAAA.String(),
		}
		data, err := json.Marshal(aaaaData)
		if err != nil {
			return nil, err
		}
		record.Data = string(data)
	case *dns.TXT:
		if len(r.Txt) > 0 {
			txtData := TXTRecordData{
				Text: r.Txt[0],
			}
			data, err := json.Marshal(txtData)
			if err != nil {
				return nil, err
			}
			record.Data = string(data)
		}
	case *dns.MX:
		record.Priority = &r.Preference
		record.Target = &r.Mx
	case *dns.SRV:
		record.Priority = &r.Priority
		record.Weight = &r.Weight
		record.Port = &r.Port
		record.Target = &r.Target
	case *dns.CNAME:
		record.Target = &r.Target
	case *dns.NS:
		record.Target = &r.Ns
	case *dns.PTR:
		record.Target = &r.Ptr
	// Add more cases for other DNS record types as needed
	default:
		return nil, fmt.Errorf("unsupported record type: %s", record.Type)
	}

	return record, nil
}
