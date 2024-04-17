package entities

import (
	"testing"
	"time"

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

func TestRDEContact_ToEntity(t *testing.T) {
	// Test cases
	tests := []struct {
		name     string
		contact  *RDEContact
		expected *Contact
		err      error
	}{
		{
			name: "Valid contact with postal info",
			contact: &RDEContact{
				ID:     "123456",
				RoID:   "123456_CONT-APEX",
				Email:  "myemail@me.com",
				ClID:   "123456",
				Voice:  "+51.123456",
				Fax:    "+51.123456",
				CrRr:   "123456",
				UpRr:   "123456",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				Status: []RDEContactStatus{
					{
						S: "ok",
					},
				},
				PostalInfo: []RDEContactPostalInfo{
					{
						Type: "int",
						Name: "myName",
						Org:  "myOrganization",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Name: "myNûme",
						Org:  "myOrganïzation",
						Address: RDEAddress{
							City:        "Cûzco",
							CountryCode: "PE",
						},
					},
				},
			},
			expected: &Contact{
				ID:        "123456",
				Email:     "myemail@me.com",
				RoID:      RoidType("123456_CONT-APEX"),
				ClID:      "123456",
				Voice:     E164Type("+51.123456"),
				Fax:       E164Type("+51.123456"),
				CrRr:      ClIDType("123456"),
				UpRr:      ClIDType("123456"),
				CreatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				Status: ContactStatus{
					OK: true,
				},
				PostalInfo: [2]*ContactPostalInfo{
					{
						Type: "int",
						Name: "myName",
						Org:  "myOrganization",
						Address: &Address{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Name: "myNûme",
						Org:  "myOrganïzation",
						Address: &Address{
							City:        "Cûzco",
							CountryCode: "PE",
						},
					},
				},
			},
		},
		{
			name: "Invalid postal info",
			contact: &RDEContact{
				ID:     "123456",
				RoID:   "123456_CONT-APEX",
				Email:  "myemail@me.com",
				ClID:   "123456",
				Voice:  "+51.123456",
				Fax:    "+51.123456",
				CrRr:   "123456",
				UpRr:   "123456",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				Status: []RDEContactStatus{
					{
						S: "ok",
					},
				},
				PostalInfo: []RDEContactPostalInfo{
					{
						Type: "invalid",
						Name: "myName",
						Org:  "myOrganization",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Name: "myNûme",
						Org:  "myOrganïzation",
						Address: RDEAddress{
							City:        "Cûzco",
							CountryCode: "PE",
						},
					},
				},
			},
			expected: nil,
			err:      ErrInvalidPostalInfoEnumType,
		},
		{
			name: "Invalid RoID",
			contact: &RDEContact{
				ID:     "123456",
				RoID:   "123456_DOM-APEX",
				Email:  "myemail@me.com",
				ClID:   "123456",
				Voice:  "+51.123456",
				Fax:    "+51.123456",
				CrRr:   "123456",
				UpRr:   "123456",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				Status: []RDEContactStatus{
					{
						S: "ok",
					},
				},
				PostalInfo: []RDEContactPostalInfo{
					{
						Type: "int",
						Name: "myName",
						Org:  "myOrganization",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Name: "myNûme",
						Org:  "myOrganïzation",
						Address: RDEAddress{
							City:        "Cûzco",
							CountryCode: "PE",
						},
					},
				},
			},
			expected: nil,
			err:      ErrInvalidContactRoID,
		},
		{
			name: "Invalid Voice",
			contact: &RDEContact{
				ID:     "123456",
				RoID:   "123456_CONT-APEX",
				Email:  "myemail@me.com",
				ClID:   "123456",
				Voice:  "123456",
				Fax:    "+51.123456",
				CrRr:   "123456",
				UpRr:   "123456",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				Status: []RDEContactStatus{
					{
						S: "ok",
					},
				},
				PostalInfo: []RDEContactPostalInfo{
					{
						Type: "int",
						Name: "myName",
						Org:  "myOrganization",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Name: "myNûme",
						Org:  "myOrganïzation",
						Address: RDEAddress{
							City:        "Cûzco",
							CountryCode: "PE",
						},
					},
				},
			},
			expected: nil,
			err:      ErrInvalidE164Type,
		},
		{
			name: "Invalid Fax",
			contact: &RDEContact{
				ID:     "123456",
				RoID:   "123456_CONT-APEX",
				Email:  "myemail@me.com",
				ClID:   "123456",
				Voice:  "+51.123456",
				Fax:    "123456",
				CrRr:   "123456",
				UpRr:   "123456",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				Status: []RDEContactStatus{
					{
						S: "ok",
					},
				},
				PostalInfo: []RDEContactPostalInfo{
					{
						Type: "int",
						Name: "myName",
						Org:  "myOrganization",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Name: "myNûme",
						Org:  "myOrganïzation",
						Address: RDEAddress{
							City:        "Cûzco",
							CountryCode: "PE",
						},
					},
				},
			},
			expected: nil,
			err:      ErrInvalidE164Type,
		},
		// {
		// 	name: "Invalid CrDate",
		// 	contact: &RDEContact{
		// 		ID:     "123456",
		// 		RoID:   "123456_CONT-APEX",
		// 		Email:  "myemail@me.com",
		// 		ClID:   "123456",
		// 		Voice:  "+51.123456",
		// 		Fax:    "+51.123456",
		// 		CrRr:   "123456",
		// 		UpRr:   "123456",
		// 		CrDate: "2021--01-01T00:00:00Z",
		// 		UpDate: "2021-01-01T00:00:00Z",
		// 		Status: []RDEContactStatus{
		// 			{
		// 				S: "ok",
		// 			},
		// 		},
		// 		PostalInfo: []RDEContactPostalInfo{
		// 			{
		// 				Type: "int",
		// 				Name: "myName",
		// 				Org:  "myOrganization",
		// 				Address: RDEAddress{
		// 					City:        "New York",
		// 					CountryCode: "US",
		// 				},
		// 			},
		// 			{
		// 				Type: "loc",
		// 				Name: "myNûme",
		// 				Org:  "myOrganïzation",
		// 				Address: RDEAddress{
		// 					City:        "Cûzco",
		// 					CountryCode: "PE",
		// 				},
		// 			},
		// 		},
		// 	},
		// 	expected: nil,
		// 	err:      ErrInvalidE164Type,
		// },
		// {
		// 	name: "Invalid UpDate",
		// 	contact: &RDEContact{
		// 		ID:     "123456",
		// 		RoID:   "123456_CONT-APEX",
		// 		Email:  "myemail@me.com",
		// 		ClID:   "123456",
		// 		Voice:  "+51.123456",
		// 		Fax:    "+51.123456",
		// 		CrRr:   "123456",
		// 		UpRr:   "123456",
		// 		CrDate: "2021-01-01T00:00:00Z",
		// 		UpDate: "2021--01-01T00:00:00Z",
		// 		Status: []RDEContactStatus{
		// 			{
		// 				S: "ok",
		// 			},
		// 		},
		// 		PostalInfo: []RDEContactPostalInfo{
		// 			{
		// 				Type: "int",
		// 				Name: "myName",
		// 				Org:  "myOrganization",
		// 				Address: RDEAddress{
		// 					City:        "New York",
		// 					CountryCode: "US",
		// 				},
		// 			},
		// 			{
		// 				Type: "loc",
		// 				Name: "myNûme",
		// 				Org:  "myOrganïzation",
		// 				Address: RDEAddress{
		// 					City:        "Cûzco",
		// 					CountryCode: "PE",
		// 				},
		// 			},
		// 		},
		// 	},
		// 	expected: nil,
		// 	err:      ErrInvalidE164Type,
		// },
		{
			name: "Invalid CrRr",
			contact: &RDEContact{
				ID:     "123456",
				RoID:   "123456_CONT-APEX",
				Email:  "myemail@me.com",
				ClID:   "123456",
				Voice:  "+51.123456",
				Fax:    "+51.123456",
				CrRr:   "123456789123456789",
				UpRr:   "123456",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				Status: []RDEContactStatus{
					{
						S: "ok",
					},
				},
				PostalInfo: []RDEContactPostalInfo{
					{
						Type: "int",
						Name: "myName",
						Org:  "myOrganization",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Name: "myNûme",
						Org:  "myOrganïzation",
						Address: RDEAddress{
							City:        "Cûzco",
							CountryCode: "PE",
						},
					},
				},
			},
			expected: nil,
			err:      ErrInvalidClIDType,
		},
		{
			name: "Invalid UpRr",
			contact: &RDEContact{
				ID:     "123456",
				RoID:   "123456_CONT-APEX",
				Email:  "myemail@me.com",
				ClID:   "123456",
				Voice:  "+51.123456",
				Fax:    "+51.123456",
				CrRr:   "123456",
				UpRr:   "123456789123456789",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				Status: []RDEContactStatus{
					{
						S: "ok",
					},
				},
				PostalInfo: []RDEContactPostalInfo{
					{
						Type: "int",
						Name: "myName",
						Org:  "myOrganization",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Name: "myNûme",
						Org:  "myOrganïzation",
						Address: RDEAddress{
							City:        "Cûzco",
							CountryCode: "PE",
						},
					},
				},
			},
			expected: nil,
			err:      ErrInvalidClIDType,
		},
		{
			name: "Invalid Status",
			contact: &RDEContact{
				ID:     "123456",
				RoID:   "123456_CONT-APEX",
				Email:  "myemail@me.com",
				ClID:   "123456",
				Voice:  "+51.123456",
				Fax:    "+51.123456",
				CrRr:   "123456",
				UpRr:   "123456",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				Status: []RDEContactStatus{
					{
						S: "invalid",
					},
				},
				PostalInfo: []RDEContactPostalInfo{
					{
						Type: "int",
						Name: "myName",
						Org:  "myOrganization",
						Address: RDEAddress{
							City:        "New York",
							CountryCode: "US",
						},
					},
					{
						Type: "loc",
						Name: "myNûme",
						Org:  "myOrganïzation",
						Address: RDEAddress{
							City:        "Cûzco",
							CountryCode: "PE",
						},
					},
				},
			},
			expected: nil,
			err:      ErrInvalidContactStatusCombination,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := tc.contact.ToEntity()
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.contact.ID, actual.ID.String())
				require.Equal(t, tc.contact.RoID, actual.RoID.String())
				require.Equal(t, tc.contact.ClID, actual.ClID.String())
				require.Equal(t, tc.contact.CrRr, actual.CrRr.String())
				require.Equal(t, tc.contact.UpRr, actual.UpRr.String())
				require.Equal(t, tc.contact.Email, actual.Email)
				require.Equal(t, len(tc.contact.PostalInfo), len(actual.PostalInfo))
				require.Equal(t, tc.contact.Voice, actual.Voice.String())
				require.Equal(t, tc.contact.Fax, actual.Fax.String())
				// Test status
				require.Equal(t, tc.expected.CreatedAt, actual.CreatedAt)
				require.Equal(t, tc.expected.UpdatedAt, actual.UpdatedAt)
			}
		})
	}
}
