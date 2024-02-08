package entities

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewContactPostalInfo(t *testing.T) {
	// Define test cases
	tests := []struct {
		name         string
		inputType    string
		inputName    string
		inputAddress *Address
		expected     *ContactPostalInfo
		expectedErr  error
	}{
		{
			name:      "valid loc postalinfo",
			inputType: "loc",
			inputName: "Jhonny Cash",
			inputAddress: &Address{
				City:        PostalLineType("El Cuyo"),
				CountryCode: CCType("MX"),
			},
			expected: &ContactPostalInfo{
				Type: PostalInfoEnumType("loc"),
				Name: "Jhonny Cash",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expectedErr: nil,
		},
		{
			name:      "valid int postalinfo",
			inputType: "int",
			inputName: "Marlon Brando",
			inputAddress: &Address{
				City:        PostalLineType("El Cuyo"),
				CountryCode: CCType("MX"),
			},
			expected: &ContactPostalInfo{
				Type: PostalInfoEnumType("int"),
				Name: "Marlon Brando",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expectedErr: nil,
		},
		{
			name:      "valid int postalinfo with uppercase type",
			inputType: "Int",
			inputName: "James Dean",
			inputAddress: &Address{
				City:        PostalLineType("El Cuyo"),
				CountryCode: CCType("MX"),
			},
			expected: &ContactPostalInfo{
				Type: PostalInfoEnumType("int"),
				Name: "James Dean",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expectedErr: nil,
		},
		{
			name:      "invalid name",
			inputType: "Int",
			inputName: "",
			inputAddress: &Address{
				City:        PostalLineType("El Cuyo"),
				CountryCode: CCType("MX"),
			},
			expected:    nil,
			expectedErr: ErrInvalidContactPostalInfo,
		},
		{
			name:         "invalid type",
			inputType:    "invalid",
			inputName:    "Marlyn Monroe",
			inputAddress: &Address{},
			expected:     nil,
			expectedErr:  ErrInvalidPostalInfoEnumType,
		},
		{
			name:         "nil address",
			inputType:    "Int",
			inputName:    "Elvis Presly",
			inputAddress: &Address{},
			expected:     nil,
			expectedErr:  ErrInvalidPostalLineType,
		},
		{
			name:         "invalid address",
			inputType:    "Int",
			inputName:    "Elvis Presly",
			inputAddress: &Address{City: PostalLineType("El Cuyo")},
			expected:     nil,
			expectedErr:  ErrInvalidCountryCode,
		},
	}

	// Run test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pi, err := NewContactPostalInfo(test.inputType, test.inputName, test.inputAddress)
			require.Equal(t, test.expectedErr, err)
			require.Equal(t, test.expected, pi)
		})
	}
}

func TestContactPostalInfo_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		pi       *ContactPostalInfo
		expected bool
	}{
		{
			name: "valid int",
			pi: &ContactPostalInfo{
				Type: "int",
				Name: "valid name",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expected: true,
		},
		{
			name: "invalid type",
			pi: &ContactPostalInfo{
				Type: "invalid",
				Name: "valid name",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expected: false,
		},
		{
			name: "invalid name",
			pi: &ContactPostalInfo{
				Type: "int",
				Name: "thisiswaaaaaytooooolooooongtobe a postalinfolinethisiswaaaaaytooooolooooongtobe a postalinfolinethisiswaaaaaytooooolooooongtobe a postalinfolinethisiswaaaaaytooooolooooongtobe a postalinfolinethisiswaaaaaytooooolooooongtobe a postalinfolinethisiswaaaaaytooooolooooongtobe a postalinfoline",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expected: false,
		},
		{
			name: "invalid org",
			pi: &ContactPostalInfo{
				Type: "int",
				Name: "valid name",
				Org:  "thisiswaaaaaytooooolooooongtobe a postalinfolinethisiswaaaaaytooooolooooongtobe a postalinfolinethisiswaaaaaytooooolooooongtobe a postalinfolinethisiswaaaaaytooooolooooongtobe a postalinfolinethisiswaaaaaytooooolooooongtobe a postalinfolinethisiswaaaaaytooooolooooongtobe a postalinfoline",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expected: false,
		},
		{
			name: "nil address",
			pi: &ContactPostalInfo{
				Type:    "int",
				Name:    "valid name",
				Address: nil,
			},
			expected: false,
		},
		{
			name: "invalid address",
			pi: &ContactPostalInfo{
				Type: "int",
				Name: "valid name",
				Address: &Address{
					City: PostalLineType("El Cuyo"),
				},
			},
			expected: false,
		},
		{
			name: "invalid ASCII name in int",
			pi: &ContactPostalInfo{
				Type: "int",
				Name: "valid nüme",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expected: false,
		},
		{
			name: "invalid ASCII org in int",
			pi: &ContactPostalInfo{
				Type: "int",
				Name: "valid name",
				Org:  "valid nüme",
				Address: &Address{
					City:        PostalLineType("El Cuyo"),
					CountryCode: CCType("MX"),
				},
			},
			expected: false,
		},
		{
			name: "invalid ASCII address element in int",
			pi: &ContactPostalInfo{
				Type: "int",
				Name: "valid",
				Address: &Address{
					City:        PostalLineType("ünderbar city"),
					CountryCode: CCType("MX"),
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		require.Equal(t, test.expected, test.pi.IsValid(), fmt.Sprintf("Validity mismatch for contact postalinfo for test '%s'", test.name))
	}
}
