package entities

import (
	"testing"
)

func TestRDEHeader_IDNCount(t *testing.T) {
	header := RDEHeader{
		Count: []RDECount{
			{Uri: IDN_URI, ID: 5},
		},
	}

	if got := header.IDNCount(); got != 5 {
		t.Errorf("IDNCount() = %v, want %v", got, 5)
	}
}

func TestRDEHeader_ContactCount(t *testing.T) {
	header := RDEHeader{
		Count: []RDECount{
			{Uri: CONTACT_URI, ID: 10},
		},
	}

	if got := header.ContactCount(); got != 10 {
		t.Errorf("ContactCount() = %v, want %v", got, 10)
	}
}

func TestRDEHeader_DomainCount(t *testing.T) {
	header := RDEHeader{
		Count: []RDECount{
			{Uri: DOMAIN_URI, ID: 15},
		},
	}

	if got := header.DomainCount(); got != 15 {
		t.Errorf("DomainCount() = %v, want %v", got, 15)
	}
}

func TestRDEHeader_HostCount(t *testing.T) {
	header := RDEHeader{
		Count: []RDECount{
			{Uri: HOST_URI, ID: 20},
		},
	}

	if got := header.HostCount(); got != 20 {
		t.Errorf("HostCount() = %v, want %v", got, 20)
	}
}

func TestRDEHeader_NNDNCount(t *testing.T) {
	header := RDEHeader{
		Count: []RDECount{
			{Uri: NNDN_URI, ID: 25},
		},
	}

	if got := header.NNDNCount(); got != 25 {
		t.Errorf("NNDNCount() = %v, want %v", got, 25)
	}
}

func TestRDEHeader_RegistrarCount(t *testing.T) {
	header := RDEHeader{
		Count: []RDECount{
			{Uri: REGISTRAR_URI, ID: 30},
		},
	}

	if got := header.RegistrarCount(); got != 30 {
		t.Errorf("RegistrarCount() = %v, want %v", got, 30)
	}
}

func TestRDEHeader_EmptyCounts(t *testing.T) {
	header := RDEHeader{}

	if got := header.IDNCount(); got != 0 {
		t.Errorf("IDNCount() = %v, want %v", got, 0)
	}
	if got := header.ContactCount(); got != 0 {
		t.Errorf("ContactCount() = %v, want %v", got, 0)
	}
	if got := header.DomainCount(); got != 0 {
		t.Errorf("DomainCount() = %v, want %v", got, 0)
	}
	if got := header.HostCount(); got != 0 {
		t.Errorf("HostCount() = %v, want %v", got, 0)
	}
	if got := header.NNDNCount(); got != 0 {
		t.Errorf("NNDNCount() = %v, want %v", got, 0)
	}
	if got := header.RegistrarCount(); got != 0 {
		t.Errorf("RegistrarCount() = %v, want %v", got, 0)
	}
}
