package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRegistrarPostalInfo(t *testing.T) {
	// Define test cases
	tests := []struct {
		inputType    string
		inputAddress *Address
		expected     *RegistrarPostalInfo
		expectedErr  error
	}{
		{
			inputType: "loc",
			inputAddress: &Address{
				City:        PostalLineType("El Cuyo"),
				CountryCode: CCType("MX"),
			},
			expected: &RegistrarPostalInfo{
				Type: PostalInfoEnumType("loc"),
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expectedErr: nil,
		},
		{
			inputType: "int",
			inputAddress: &Address{
				City:        PostalLineType("El Cuyo"),
				CountryCode: CCType("MX"),
			},
			expected: &RegistrarPostalInfo{
				Type: PostalInfoEnumType("int"),
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expectedErr: nil,
		},
		{
			inputType: "Int",
			inputAddress: &Address{
				City:        PostalLineType("El Cuyo"),
				CountryCode: CCType("MX"),
			},
			expected: &RegistrarPostalInfo{
				Type: PostalInfoEnumType("int"),
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expectedErr: nil,
		},
		{
			inputType:    "invalid",
			inputAddress: &Address{},
			expected:     nil,
			expectedErr:  ErrInvalidPostalInfoEnumType,
		},
		{
			inputType:    "Int",
			inputAddress: &Address{},
			expected:     nil,
			expectedErr:  ErrInvalidRegistrarPostalInfo,
		},
	}

	// Run test cases
	for _, test := range tests {
		actual, err := NewRegistrarPostalInfo(test.inputType, test.inputAddress)
		require.Equal(t, test.expected, actual, "RegistrarPostalInfo mismatch")
		require.Equal(t, test.expectedErr, err, "Error mismatch")
		if test.expectedErr == nil {
			require.True(t, actual.IsValid(), "IsValid() should return true")
		}
	}
}

func TestRegistrarPostalInfo_IsValid(t *testing.T) {
	tests := []struct {
		pi       *RegistrarPostalInfo
		expected bool
	}{
		{
			pi: &RegistrarPostalInfo{
				Type: "int",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expected: true,
		},
		{
			pi: &RegistrarPostalInfo{
				Type: "invalid",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expected: false,
		},
		{
			pi: &RegistrarPostalInfo{
				Type:    "int",
				Address: nil,
			},
			expected: false,
		},
	}

	for _, test := range tests {
		require.Equal(t, test.expected, test.pi.IsValid(), "Validity mismatch for registrar postalinfo")
	}
}
