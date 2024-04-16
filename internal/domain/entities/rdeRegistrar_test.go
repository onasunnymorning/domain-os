package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRDEAddress_ToEntity(t *testing.T) {
	// Test cases
	tests := []struct {
		name     string
		address  *RDEAddress
		expected *Address
		err      error
	}{
		{
			name: "Valid address with city and country code",
			address: &RDEAddress{
				City:        "New York",
				CountryCode: "US",
			},
			expected: &Address{
				City:        "New York",
				CountryCode: "US",
			},
			err: nil,
		},
		{
			name: "InValid country code",
			address: &RDEAddress{
				City:        "New York",
				CountryCode: "PZZZ",
			},
			expected: nil,
			err:      ErrInvalidCountryCode,
		},
		{
			name: "Valid address with city, country code, and state/province",
			address: &RDEAddress{
				City:          "Los Angeles",
				CountryCode:   "US",
				StateProvince: "California",
			},
			expected: &Address{
				City:          "Los Angeles",
				CountryCode:   "US",
				StateProvince: "California",
			},
			err: nil,
		},
		{
			name: "InValid state/province",
			address: &RDEAddress{
				City:          "Los Angeles",
				CountryCode:   "US",
				StateProvince: "thissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooong",
			},
			expected: nil,
			err:      ErrInvalidOptPostalLineType,
		},
		{
			name: "Valid address with city, country code, and postal code",
			address: &RDEAddress{
				City:        "London",
				CountryCode: "GB",
				PostalCode:  "SW1A 1AA",
			},
			expected: &Address{
				City:        "London",
				CountryCode: "GB",
				PostalCode:  "SW1A 1AA",
			},
			err: nil,
		},
		{
			name: "InValid postal code",
			address: &RDEAddress{
				City:        "London",
				CountryCode: "GB",
				PostalCode:  "thissssisssstooooooooolooooooongthissssisssstooooooooo",
			},
			expected: nil,
			err:      ErrInvalidPCType,
		},
		{
			name: "InValid streeet address",
			address: &RDEAddress{
				City:        "London",
				CountryCode: "GB",
				Street: []string{
					"Rue de la Paix",
					"Avenue des Champs-Élysées",
					"thissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooongthissssisssstooooooooolooooooong",
				},
			},
			expected: nil,
			err:      ErrInvalidOptPostalLineType,
		},
		{
			name: "Valid address with city, country code, and multiple streets",
			address: &RDEAddress{
				City:        "Paris",
				CountryCode: "FR",
				Street: []string{
					"Rue de la Paix",
					"Avenue des Champs-Élysées",
					"Apt 3",
				},
			},
			expected: &Address{
				City:        "Paris",
				CountryCode: "FR",
				Street1:     "Rue de la Paix",
				Street2:     "Avenue des Champs-Élysées",
				Street3:     "Apt 3",
			},
			err: nil,
		},
		{
			name: "Invalid address with too many streets",
			address: &RDEAddress{
				City:        "Berlin",
				CountryCode: "DE",
				Street: []string{
					"Street 1",
					"Street 2",
					"Street 3",
					"Street 4",
				},
			},
			expected: nil,
			err:      ErrInvalidStreetCount,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := tc.address.ToEntity()
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}

func TestRDEWhoisInfo_ToEntity(t *testing.T) {
	// Test cases
	tests := []struct {
		name     string
		whois    *RDEWhoisInfo
		expected *WhoisInfo
		err      error
	}{
		{
			name: "Valid WhoisInfo",
			whois: &RDEWhoisInfo{
				Name: "example.com",
				URL:  "https://example.com",
			},
			expected: &WhoisInfo{
				Name: "example.com",
				URL:  "https://example.com",
			},
			err: nil,
		},
		{
			name: "Invalid Domain Name",
			whois: &RDEWhoisInfo{
				Name: "averyyylooooooooongdoooooomaiiiiiiinnaaaaaaameaveryyylooooooooongdoooooomaiiiiiiinnaaaaaaameaveryyylooooooooongdoooooomaiiiiiiinnaaaaaaameaveryyylooooooooongdoooooomaiiiiiiinnaaaaaaameaveryyylooooooooongdoooooomaiiiiiiinnaaaaaaame.com",
				URL:  "https://example.com",
			},
			expected: nil,
			err:      ErrInvalidLabelLength,
		},
		{
			name: "Invalid URL",
			whois: &RDEWhoisInfo{
				Name: "example.com",
				URL:  "//example.com",
			},
			expected: nil,
			err:      ErrInvalidURL,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := tc.whois.ToEntity()
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}
func TestRDERegistrarPostalInfo_ToEntity(t *testing.T) {
	// Test cases
	tests := []struct {
		name     string
		postal   *RDERegistrarPostalInfo
		expected *RegistrarPostalInfo
		err      error
	}{
		{
			name: "Valid postal info",
			postal: &RDERegistrarPostalInfo{
				Type: "int",
				Address: RDEAddress{
					City:        "New York",
					CountryCode: "US",
				},
			},
			expected: &RegistrarPostalInfo{
				Type: "int",
				Address: &Address{
					City:        "New York",
					CountryCode: "US",
				},
			},
			err: nil,
		},
		{
			name: "Valid type",
			postal: &RDERegistrarPostalInfo{
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
			postal: &RDERegistrarPostalInfo{
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
			postal: &RDERegistrarPostalInfo{
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
func TestRDERegistrar_ToEntity(t *testing.T) {
	// Test cases
	tests := []struct {
		name      string
		registrar *RDERegistrar
		expected  *Registrar
		err       error
	}{
		{
			name: "Valid registrar with postal info",
			registrar: &RDERegistrar{
				ID:    "123",
				Name:  "Example Registrar",
				Email: "info@example.com",
				GurID: 456,
				PostalInfo: []RDERegistrarPostalInfo{
					{
						Type: "int",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Address: RDEAddress{
							City:        "London",
							CountryCode: "GB",
						},
					},
				},
				Voice:  "+123456789",
				Fax:    "+987654321",
				URL:    "https://example-registrar.com",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
				WhoisInfo: RDEWhoisInfo{
					Name: "example-registrar.com",
					URL:  "https://example-registrar.com",
				},
				Status: RDERegistrarStatus{
					S: "ok",
				},
			},
			expected: &Registrar{
				ClID:  ClIDType("123"),
				Name:  "Example Registrar",
				Email: "info@example.com",
				GurID: 456,
				PostalInfo: [2]*RegistrarPostalInfo{
					{
						Type: "int",
						Address: &Address{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Address: &Address{
							City:        "London",
							CountryCode: "GB",
						},
					},
				},
				Voice:     E164Type("+123456789"),
				Fax:       E164Type("+987654321"),
				URL:       URL("https://example-registrar.com"),
				CreatedAt: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC),
				WhoisInfo: WhoisInfo{
					Name: "example-registrar.com",
					URL:  "https://example-registrar.com",
				},
				Status: RegistrarStatus("ok"),
			},
			err: nil,
		},
		{
			name: "InValid registrar Name",
			registrar: &RDERegistrar{
				ID:    "123",
				Name:  "",
				Email: "info@example.com",
				GurID: 456,
				PostalInfo: []RDERegistrarPostalInfo{
					{
						Type: "int",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Address: RDEAddress{
							City:        "London",
							CountryCode: "GB",
						},
					},
				},
				Voice:  "+123456789",
				Fax:    "+987654321",
				URL:    "https://example-registrar.com",
				CrDate: "2022-01-01T00:00:00Z",
				UpDate: "2022-01-02T00:00:00Z",
				WhoisInfo: RDEWhoisInfo{
					Name: "example-registrar.com",
					URL:  "https://example-registrar.com",
				},
				Status: RDERegistrarStatus{
					S: "ok",
				},
			},
			expected: nil,
			err:      ErrRegistrarMissingName,
		},
		// {
		// 	name: "InValid date format Create",
		// 	registrar: &RDERegistrar{
		// 		ID:    "123",
		// 		Name:  "Myname",
		// 		Email: "info@example.com",
		// 		GurID: 456,
		// 		PostalInfo: []RDERegistrarPostalInfo{
		// 			{
		// 				Type: "int",
		// 				Address: RDEAddress{
		// 					City:        "New York",
		// 					CountryCode: "US",
		// 				},
		// 			},
		// 			{
		// 				Type: "loc",
		// 				Address: RDEAddress{
		// 					City:        "London",
		// 					CountryCode: "GB",
		// 				},
		// 			},
		// 		},
		// 		Voice:  "+123456789",
		// 		Fax:    "+987654321",
		// 		URL:    "https://example-registrar.com",
		// 		CrDate: "2022--01T00:00:00Z",
		// 		UpDate: "2022-01-02T00:00:00Z",
		// 		WhoisInfo: RDEWhoisInfo{
		// 			Name: "example-registrar.com",
		// 			URL:  "https://example-registrar.com",
		// 		},
		// 		Status: RDERegistrarStatus{
		// 			S: "ok",
		// 		},
		// 	},
		// 	expected: nil,
		// 	err: &time.ParseError{
		// 		Value:      "2022--01T00:00:00Z",
		// 		Layout:     time.RFC3339,
		// 		LayoutElem: "01",
		// 		ValueElem:  "-01T00:00:00Z",
		// 	},
		// },
		{
			name: "Invalid registrar with too many postal info",
			registrar: &RDERegistrar{
				ID:    "123",
				Name:  "Example Registrar",
				Email: "info@example.com",
				GurID: 456,
				PostalInfo: []RDERegistrarPostalInfo{
					{
						Type: "int",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Address: RDEAddress{
							City:        "London",
							CountryCode: "GB",
						},
					},
					{
						Type: "int",
						Address: RDEAddress{
							City:        "Paris",
							CountryCode: "FR",
						},
					},
				},
			},
			expected: nil,
			err:      ErrInvalidPostalInfoCount,
		},
		{
			name: "Invalid registrar with invalid PiType",
			registrar: &RDERegistrar{
				ID:    "123",
				Name:  "Example Registrar",
				Email: "info@example.com",
				GurID: 456,
				PostalInfo: []RDERegistrarPostalInfo{
					{
						Type: "int",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "ext",
						Address: RDEAddress{
							City:        "London",
							CountryCode: "GB",
						},
					},
				},
			},
			expected: nil,
			err:      ErrInvalidPostalInfoEnumType,
		},
		{
			name: "Invalid registrar with invalid WhoisInfo",
			registrar: &RDERegistrar{
				ID:    "123",
				Name:  "Example Registrar",
				Email: "info@example.com",
				GurID: 456,
				PostalInfo: []RDERegistrarPostalInfo{
					{
						Type: "int",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Address: RDEAddress{
							City:        "London",
							CountryCode: "GB",
						},
					},
				},
				WhoisInfo: RDEWhoisInfo{
					Name: "",
					URL:  "//example-registrar.com",
				},
			},
			expected: nil,
			err:      ErrInvalidURL,
		},
		{
			name: "Invalid registrar with invalid URL",
			registrar: &RDERegistrar{
				ID:    "123",
				Name:  "Example Registrar",
				Email: "info@example.com",
				GurID: 456,
				PostalInfo: []RDERegistrarPostalInfo{
					{
						Type: "int",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Address: RDEAddress{
							City:        "London",
							CountryCode: "GB",
						},
					},
				},
				WhoisInfo: RDEWhoisInfo{
					Name: "example-registrar.com",
					URL:  "//example-registrar.com",
				},
			},
			expected: nil,
			err:      ErrInvalidURL,
		},
		{
			name: "Invalid registrar with invalid status",
			registrar: &RDERegistrar{
				ID:    "123",
				Name:  "Example Registrar",
				Email: "info@example.com",
				GurID: 456,
				PostalInfo: []RDERegistrarPostalInfo{
					{
						Type: "int",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Address: RDEAddress{
							City:        "London",
							CountryCode: "GB",
						},
					},
				},
				Status: RDERegistrarStatus{
					S: "bogus",
				},
			},
			expected: nil,
			err:      ErrInvalidRegistrarStatus,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := tc.registrar.ToEntity()
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.registrar.ID, actual.ClID.String())
				require.Equal(t, tc.registrar.Name, actual.Name)
				require.Equal(t, tc.registrar.Email, actual.Email)
				require.Equal(t, tc.registrar.GurID, actual.GurID)
				require.Equal(t, len(tc.registrar.PostalInfo), len(actual.PostalInfo))
				require.Equal(t, tc.registrar.Voice, actual.Voice.String())
				require.Equal(t, tc.registrar.Fax, actual.Fax.String())
				require.Equal(t, tc.registrar.URL, actual.URL.String())
				require.Equal(t, tc.registrar.WhoisInfo.Name, actual.WhoisInfo.Name.String())
				require.Equal(t, tc.registrar.WhoisInfo.URL, actual.WhoisInfo.URL.String())
				require.Equal(t, tc.registrar.Status.S, actual.Status.String())
				require.Equal(t, tc.expected.CreatedAt, actual.CreatedAt)
				require.Equal(t, tc.expected.UpdatedAt, actual.UpdatedAt)
			}
		})
	}
}
