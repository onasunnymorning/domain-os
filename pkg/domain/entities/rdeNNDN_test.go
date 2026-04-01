package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRDENNDN_ToCSV(t *testing.T) {
	n := &RDENNDN{
		AName:        "example.com",
		UName:        "example.com",
		IDNTableID:   "1",
		OriginalName: "example.com",
		NameState:    "active",
		CrDate:       "2022-01-01",
	}

	expected := []string{"example.com", "example.com", "1", "example.com", "active", "2022-01-01"}
	result := n.ToCSV()

	require.Equal(t, expected, result, "CSV values mismatch")
	require.Equal(t, len(RDE_NNDN_CSV_HEADER), len(result), "CSV length mismatch")
}
