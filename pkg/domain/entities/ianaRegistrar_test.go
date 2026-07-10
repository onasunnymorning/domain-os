package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateClID(t *testing.T) {
	registrar := IANARegistrar{
		GurID:     123,
		Name:      "Example Registrar, Inc.",
		Status:    IANARegistrarStatusAccredited,
		RdapURL:   "https://example-registrar.com/rdap",
		CreatedAt: time.Now(),
	}

	expectedClID := ClIDType("123-example-regi")
	clID, err := registrar.CreateClID()
	require.NoError(t, err)
	require.Equal(t, expectedClID, clID)
}

func TestCreateClID_SpecialReservedRegistrars(t *testing.T) {
	tests := []struct {
		name     string
		gurID    int
		ianaName string
		wantClID string
	}{
		{"9995 PDT#1", 9995, "Reserved for Pre-Delegation Testing transactions #1 reporting", "9995-pdt-1"},
		{"9996 PDT#2", 9996, "Reserved for Pre-Delegation Testing transactions #2 reporting", "9996-pdt-2"},
		{"9997 SLA Monitor", 9997, "Reserved for ICANN's Registry SLA Monitoring System transactions reporting", "9997-sla-monitor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := IANARegistrar{GurID: tt.gurID, Name: tt.ianaName, Status: IANARegistrarStatusReserved}
			clID, err := r.CreateClID()
			require.NoError(t, err)
			require.Equal(t, ClIDType(tt.wantClID), clID, "special reserved registrar should get a descriptive ClID")
		})
	}
}
func TestIANARegistrarStatusIsValid(t *testing.T) {
	tests := []struct {
		name   string
		status IANARegistrarStatus
		want   bool
	}{
		{"Accredited is valid", IANARegistrarStatusAccredited, true},
		{"Reserved is valid", IANARegistrarStatusReserved, true},
		{"Terminated is valid", IANARegistrarStatusTerminated, true},
		{"Unknown is valid", IANARegistrarStatusUnknown, true},
		{"empty string is invalid", IANARegistrarStatus(""), false},
		{"lowercase accredited is invalid", IANARegistrarStatus("accredited"), false},
		{"arbitrary string is invalid", IANARegistrarStatus("Active"), false},
		{"partial match is invalid", IANARegistrarStatus("Accredit"), false},
		{"extra whitespace is invalid", IANARegistrarStatus(" Accredited "), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsValid()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIANARegistrarStatusString(t *testing.T) {
	tests := []struct {
		status   IANARegistrarStatus
		expected string
	}{
		{IANARegistrarStatusAccredited, "Accredited"},
		{IANARegistrarStatusReserved, "Reserved"},
		{IANARegistrarStatusTerminated, "Terminated"},
		{IANARegistrarStatusUnknown, "Unknown"},
	}

	for _, test := range tests {
		t.Run(string(test.status), func(t *testing.T) {
			require.Equal(t, test.expected, test.status.String())
		})
	}
}
