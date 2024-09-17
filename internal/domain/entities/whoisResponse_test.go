package entities

import (
	"strconv"
	"testing"
	"time"
)

func TestWhoisResponse_String(t *testing.T) {
	w := WhoisResponse{
		DomainName:                 "example.com",
		RegistryDomainID:           "123456789",
		RegistrarWhoisServer:       "whois.example.com",
		RegistrarURL:               "http://example.com",
		UpdatedDate:                time.Now(),
		CreationDate:               time.Now().AddDate(-1, 0, 0),
		RegistryExpiryDate:         time.Now().AddDate(1, 0, 0),
		Registrar:                  "Example Registrar",
		RegistrarIANAID:            "1234",
		RegistrarAbuseContactEmail: "abuse@example.com",
		RegistrarAbuseContactPhone: "+1.1234567890",
		DomainStatus:               []string{"active", "ok"},
		NameServers:                []string{"ns1.example.com", "ns2.example.com"},
		DNSSEC:                     "unsigned",
		ICANNComplaintURL:          "http://example.com/complaint",
		LastWhoisUpdate:            time.Now(),
	}

	expected := "Domain Name: example.com\n" +
		"Registry Domain ID: 123456789\n" +
		"Registrar WHOIS Server: whois.example.com\n" +
		"Registrar URL: http://example.com\n" +
		"Updated Date: " + w.UpdatedDate.String() + "\n" +
		"Creation Date: " + w.CreationDate.String() + "\n" +
		"Registry Expiry Date: " + w.RegistryExpiryDate.String() + "\n" +
		"Registrar: Example Registrar\n" +
		"Registrar IANA ID: 1234\n" +
		"Registrar Abuse Contact Email: abuse@example.com\n" +
		"Registrar Abuse Contact Phone: +1.1234567890\n" +
		"Domain Status: active\n" +
		"Domain Status: ok\n" +
		"DNSSEC Data: ns1.example.com\n" +
		"DNSSEC Data: ns2.example.com\n" +
		"DNSSEC: unsigned\n" +
		"ICANN Complaint URL: http://example.com/complaint\n" +
		">>> Last update of whois database:" + w.LastWhoisUpdate.String() + " <<<\n"

	if w.String() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, w.String())
	}
}

func TestNewWhoisResponse(t *testing.T) {
	var hosts []*Host
	for i := 0; i < 3; i++ {
		hosts = append(hosts, &Host{
			Name: DomainName("ns" + strconv.Itoa(i) + ".example.com"),
		})
	}
	dom := &Domain{
		Name:       "example.com",
		RoID:       "123456789",
		UpdatedAt:  time.Now(),
		CreatedAt:  time.Now().AddDate(-1, 0, 0),
		ExpiryDate: time.Now().AddDate(1, 0, 0),
		Status: DomainStatus{
			OK: true,
		},
		Hosts: hosts,
	}
	rar := &Registrar{
		Name:        "Example Registrar",
		GurID:       1234,
		WhoisInfo:   WhoisInfo{Name: "whois.example.com"},
		URL:         "http://example.com",
		Email:       "abuse@example.com",
		Voice:       "+1.1234567890",
		RdapBaseURL: "http://example.com/complaint",
	}

	w, err := NewWhoisResponse(dom, rar)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if w.DomainName != dom.Name.String() {
		t.Errorf("Expected DomainName %s, got %s", dom.Name, w.DomainName)
	}
	if w.RegistryDomainID != dom.RoID.String() {
		t.Errorf("Expected RegistryDomainID %s, got %s", dom.RoID, w.RegistryDomainID)
	}
	if w.Registrar != rar.Name {
		t.Errorf("Expected Registrar %s, got %s", rar.Name, w.Registrar)
	}
	if w.RegistrarWhoisServer != rar.WhoisInfo.Name.String() {
		t.Errorf("Expected RegistrarWhoisServer %s, got %s", rar.WhoisInfo.Name, w.RegistrarWhoisServer)
	}
	if w.RegistrarURL != rar.URL.String() {
		t.Errorf("Expected RegistrarURL %s, got %s", rar.URL, w.RegistrarURL)
	}
	if w.UpdatedDate != dom.UpdatedAt {
		t.Errorf("Expected UpdatedDate %s, got %s", dom.UpdatedAt, w.UpdatedDate)
	}
	if w.CreationDate != dom.CreatedAt {
		t.Errorf("Expected CreationDate %s, got %s", dom.CreatedAt, w.CreationDate)
	}
	if w.RegistryExpiryDate != dom.ExpiryDate {
		t.Errorf("Expected RegistryExpiryDate %s, got %s", dom.ExpiryDate, w.RegistryExpiryDate)
	}
	if w.RegistrarIANAID != strconv.Itoa(rar.GurID) {
		t.Errorf("Expected RegistrarIANAID %d, got %s", rar.GurID, w.RegistrarIANAID)
	}
	if w.RegistrarAbuseContactEmail != rar.Email {
		t.Errorf("Expected RegistrarAbuseContactEmail %s, got %s", rar.Email, w.RegistrarAbuseContactEmail)
	}
	if w.RegistrarAbuseContactPhone != rar.Voice.String() {
		t.Errorf("Expected RegistrarAbuseContactPhone %s, got %s", rar.Voice, w.RegistrarAbuseContactPhone)
	}
	if len(w.DomainStatus) != 1 {
		t.Errorf("Expected 1 DomainStatus, got %d", len(w.DomainStatus))
	}
	if w.DomainStatus[0] != "ok" {
		t.Errorf("Expected DomainStatus ok, got %s", w.DomainStatus[0])
	}
	if len(w.NameServers) != 3 {
		t.Errorf("Expected 3 NameServers, got %d", len(w.NameServers))
	}
	if w.NameServers[0] != "ns0.example.com" {
		t.Errorf("Expected NameServer ns0.example.com, got %s", w.NameServers[0])
	}
	if w.NameServers[1] != "ns1.example.com" {
		t.Errorf("Expected NameServer ns1.example.com, got %s", w.NameServers[1])
	}
	if w.NameServers[2] != "ns2.example.com" {
		t.Errorf("Expected NameServer ns2.example.com, got %s", w.NameServers[2])
	}
	if w.DNSSEC != "unsigned" {
		t.Errorf("Expected DNSSEC unsigned, got %s", w.DNSSEC)
	}
	if w.ICANNComplaintURL != "URL of the ICANN Whois Inaccuracy Complaint Form: https://www.icann.org/wicf/" {
		t.Errorf("Expected ICANNComplaintURL %s, got %s", "URL of the ICANN Whois Inaccuracy Complaint Form: https://www.icann.org/wicf/", w.ICANNComplaintURL)
	}
	if w.LastWhoisUpdate.IsZero() {
		t.Errorf("Expected LastWhoisUpdate to be set, got zero value")
	}

}
