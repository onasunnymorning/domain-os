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
