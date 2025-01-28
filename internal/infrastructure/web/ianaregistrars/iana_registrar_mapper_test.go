package ianaregistrars

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromIANAXMLRegistrarRecord(t *testing.T) {
	record := &RegistrarRecord{
		Value:  "123",
		Name:   "Example Registrar",
		Status: "Active",
		RdapURL: RdapURL{
			Server: "https://example.com/rdap",
		},
	}

	registrar := FromIANAXMLRegistrarRecord(record)

	require.Equal(t, 123, registrar.GurID, "IANARegistrar GurID mismatch")
	require.Equal(t, record.Name, registrar.Name, "IANARegistrar Name mismatch")
	require.Equal(t, record.Status, string(registrar.Status), "IANARegistrar Status mismatch")
	require.Equal(t, record.RdapURL.Server, registrar.RdapURL, "IANARegistrar RdapURL mismatch")

}
