package mappers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/registry-os/internal/domain/entities"
)

func TestNewTLDResultFromTLD(t *testing.T) {
	tld, err := entities.NewTLD("com")
	if err != nil {
		t.Fatal(err)
	}

	tldResult := NewTLDResultFromTLD(tld)

	require.Equal(t, tld.Name.String(), tldResult.Name, "TLDResult Name mismatch")
	require.Equal(t, tld.Type.String(), tldResult.Type, "TLDResult Type mismatch")
	require.Equal(t, tld.UName, tldResult.UName, "TLDResult UName mismatch")
	require.Equal(t, tld.CreatedAt, tldResult.CreatedAt, "TLDResult CreatedAt mismatch")
	require.Equal(t, tld.UpdatedAt, tldResult.UpdatedAt, "TLDResult UpdatedAt mismatch")
}
