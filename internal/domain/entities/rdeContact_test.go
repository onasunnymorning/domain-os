package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRDEContactPostalInfo_ToEntity(t *testing.T) {
	// Test cases
	tests := []struct {
		name     string
		postal   *RDEContactPostalInfo
		expected *ContactPostalInfo
		err      error
	}{
		{
			name: "Valid postal info",
			postal: &RDEContactPostalInfo{
				Type: "int",
				Name: "myName",
				Org:  "myOrganization",
				Address: RDEAddress{
					City:        "New York",
					CountryCode: "US",
				},
			},
			expected: &ContactPostalInfo{
				Type: "int",
				Name: "myName",
				Org:  "myOrganization",
				Address: &Address{
					City:        "New York",
					CountryCode: "US",
				},
			},
			err: nil,
		},
		{
			name: "Invalid Org",
			postal: &RDEContactPostalInfo{
				Type: "int",
				Name: "myName",
				Org:  "thissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooong",
				Address: RDEAddress{
					City:        "New York",
					CountryCode: "US",
				},
			},
			expected: nil,
			err:      ErrInvalidOptPostalLineType,
		},
		{
			name: "Missing Name",
			postal: &RDEContactPostalInfo{
				Type: "int",
				Org:  "myOrganization",
				Address: RDEAddress{
					City:        "New York",
					CountryCode: "US",
				},
			},
			expected: nil,
			err:      ErrInvalidContactPostalInfo,
		},
		{
			name: "Valid type",
			postal: &RDEContactPostalInfo{
				Type: "invalid",
				Address: RDEAddress{
					City:        "New York",
					CountryCode: "US",
				},
			},
			expected: nil,
			err:      ErrInvalidPostalInfoEnumType,
		},
		{
			name: "Invalid address",
			postal: &RDEContactPostalInfo{
				Type: "int",
				Address: RDEAddress{
					City:          "Los Angeles",
					CountryCode:   "US",
					StateProvince: "thissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooong",
				},
			},
			expected: nil,
			err:      ErrInvalidOptPostalLineType,
		},
		{
			name: "Invalid postal info",
			postal: &RDEContactPostalInfo{
				Type: "int",
				Address: RDEAddress{
					City:        "London",
					CountryCode: "GB",
					PostalCode:  "thissssisssstooooooooolooooooongthissssisssstooooooooo",
				},
			},
			expected: nil,
			err:      ErrInvalidPCType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := tc.postal.ToEntity()
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}
