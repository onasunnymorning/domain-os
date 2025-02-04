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
		require.ErrorIs(t, err, test.expectedErr, "Error mismatch")
		if test.expectedErr == nil {
			require.Nil(t, actual.IsValid(), "IsValid() should return nil")
		}
	}
}

func TestRegistrarPostalInfo_IsValid(t *testing.T) {
	tests := []struct {
		pi       *RegistrarPostalInfo
		expected error
	}{
		{
			pi: &RegistrarPostalInfo{
				Type: "int",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expected: nil,
		},
		{
			pi: &RegistrarPostalInfo{
				Type: "invalid",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expected: ErrInvalidPostalInfoEnumType,
		},
		{
			pi: &RegistrarPostalInfo{
				Type:    "int",
				Address: nil,
			},
			expected: ErrInvalidRegistrarPostalInfo,
		},
	}

	for _, test := range tests {
		require.Equal(t, test.expected, test.pi.IsValid(), "Validity mismatch for registrar postalinfo")
	}
}
func TestRegistrarPostalInfo_DeepCopy(t *testing.T) {
	tests := []struct {
		original *RegistrarPostalInfo
	}{
		{
			original: &RegistrarPostalInfo{
				Type: "loc",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
		},
		{
			original: &RegistrarPostalInfo{
				Type: "int",
				Address: &Address{
					City:        PostalLineType("New York"),
					CountryCode: CCType("US"),
				},
			},
		},
		{
			original: &RegistrarPostalInfo{
				Type:    "loc",
				Address: nil,
			},
		},
	}

	for _, test := range tests {
		copy := test.original.DeepCopy()
		require.Equal(t, test.original, &copy, "DeepCopy() should create an identical copy")

		// Ensure that the Address field is a deep copy
		if test.original.Address != nil {
			require.NotSame(t, test.original.Address, copy.Address, "Address field should be a deep copy")
		}
	}
}
