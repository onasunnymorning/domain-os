package queries

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDomainCheckQuery(t *testing.T) {
	domainName := "example.com"
	includeFees := true

	query, err := NewDomainCheckQuery(domainName, includeFees)
	require.NoError(t, err, "Failed to create DomainCheckQuery")
	require.Equal(t, domainName, query.DomainName.String(), "DomainName mismatch")
	require.Equal(t, includeFees, query.IncludeFees, "IncludeFees mismatch")
}
