package commands

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestNewCreateContactCommand(t *testing.T) {
	testcases := []struct {
		name     string
		id       string
		email    string
		authinfo string
		clid     string
		wantErr  error
	}{
		{
			name:     "valid",
			id:       "id",
			email:    "email",
			authinfo: "authinfo",
			clid:     "clid",
			wantErr:  nil,
		},

		{
			name:     "empty id",
			id:       "",
			email:    "email",
			authinfo: "authinfo",
			clid:     "clid",
			wantErr:  ErrMissingCreateContactCommandFields,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewCreateContactCommand(tc.id, tc.email, tc.authinfo, tc.clid)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}

}

func TestFromRdeContact(t *testing.T) {
	testcases := []struct {
		name       string
		rdeContact *entities.RDEContact
		cmd        *CreateContactCommand
		wantErr    error
	}{
		{
			name: "valid",
			rdeContact: &entities.RDEContact{
				ID:    "validClID",
				Email: "email@me.com",
				ClID:  "myRegstrarID",
				Status: []entities.RDEContactStatus{
					{
						S: "OK",
					},
				},
				PostalInfo: []entities.RDEContactPostalInfo{
					{
						Type: "int",
						Name: "name",
						Org:  "org",
						Address: entities.RDEAddress{
							Street:      []string{"street"},
							City:        "Ollantaytambo",
							PostalCode:  "pc",
							CountryCode: "PE",
						},
					},
				},
				Voice:  "+1.123345345",
				Fax:    "+1.123345345",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				CrRr:   "myRegstrarID",
				UpRr:   "myRegstrarID",
			},
			cmd: &CreateContactCommand{
				ID:       "validClID",
				Email:    "email@me.com",
				AuthInfo: "escr0W1mP*rt",
				ClID:     "myRegstrarID",
				Status: entities.ContactStatus{
					OK: true,
				},
				PostalInfo: [2]*entities.ContactPostalInfo{
					{
						Type: "int",
						Name: "name",
						Org:  "org",
						Address: &entities.Address{
							Street1:     "street",
							City:        "Ollantaytambo",
							PostalCode:  "pc",
							CountryCode: "PE",
						},
					},
				},
				Voice:     "+1.123345345",
				Fax:       "+1.123345345",
				CrRr:      "myRegstrarID",
				CreatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				UpRr:      "myRegstrarID",
				UpdatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
			wantErr: nil,
		},
		{
			name: "invalid postalinfo type",
			rdeContact: &entities.RDEContact{
				ID:    "validClID",
				Email: "email@me.com",
				ClID:  "myRegstrarID",
				Status: []entities.RDEContactStatus{
					{
						S: "OK",
					},
				},
				PostalInfo: []entities.RDEContactPostalInfo{
					{
						Type: "invalid",
						Name: "name",
						Org:  "org",
						Address: entities.RDEAddress{
							Street:      []string{"street"},
							City:        "Ollantaytambo",
							PostalCode:  "pc",
							CountryCode: "PE",
						},
					},
				},
				Voice:  "+1.123345345",
				Fax:    "+1.123345345",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				CrRr:   "myRegstrarID",
				UpRr:   "myRegstrarID",
			},
			cmd:     nil,
			wantErr: entities.ErrInvalidPostalInfoEnumType,
		},
		{
			name: "missing id",
			rdeContact: &entities.RDEContact{
				ID:    "",
				Email: "email@me.com",
				ClID:  "myRegstrarID",
				Status: []entities.RDEContactStatus{
					{
						S: "OK",
					},
				},
				PostalInfo: []entities.RDEContactPostalInfo{
					{
						Type: "int",
						Name: "name",
						Org:  "org",
						Address: entities.RDEAddress{
							Street:      []string{"street"},
							City:        "Ollantaytambo",
							PostalCode:  "pc",
							CountryCode: "PE",
						},
					},
				},
				Voice:  "+1.123345345",
				Fax:    "+1.123345345",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				CrRr:   "myRegstrarID",
				UpRr:   "myRegstrarID",
			},
			cmd:     nil,
			wantErr: ErrMissingCreateContactCommandFields,
		},
		{
			name: "invalid CrDate",
			rdeContact: &entities.RDEContact{
				ID:    "validID",
				Email: "email@me.com",
				ClID:  "myRegstrarID",
				Status: []entities.RDEContactStatus{
					{
						S: "OK",
					},
				},
				PostalInfo: []entities.RDEContactPostalInfo{
					{
						Type: "int",
						Name: "name",
						Org:  "org",
						Address: entities.RDEAddress{
							Street:      []string{"street"},
							City:        "Ollantaytambo",
							PostalCode:  "pc",
							CountryCode: "PE",
						},
					},
				},
				Voice:  "+1.123345345",
				Fax:    "+1.123345345",
				CrDate: "2021--01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				CrRr:   "myRegstrarID",
				UpRr:   "myRegstrarID",
			},
			cmd:     nil,
			wantErr: entities.ErrInvalidTimeFormat,
		},
		{
			name: "invalid CrDate",
			rdeContact: &entities.RDEContact{
				ID:    "validID",
				Email: "email@me.com",
				ClID:  "myRegstrarID",
				Status: []entities.RDEContactStatus{
					{
						S: "OK",
					},
				},
				PostalInfo: []entities.RDEContactPostalInfo{
					{
						Type: "int",
						Name: "name",
						Org:  "org",
						Address: entities.RDEAddress{
							Street:      []string{"street"},
							City:        "Ollantaytambo",
							PostalCode:  "pc",
							CountryCode: "PE",
						},
					},
				},
				Voice:  "+1.123345345",
				Fax:    "+1.123345345",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021--01-01T00:00:00Z",
				CrRr:   "myRegstrarID",
				UpRr:   "myRegstrarID",
			},
			cmd:     nil,
			wantErr: entities.ErrInvalidTimeFormat,
		},
		{
			name: "invalid status name",
			rdeContact: &entities.RDEContact{
				ID:    "validID",
				Email: "email@me.com",
				ClID:  "myRegstrarID",
				Status: []entities.RDEContactStatus{
					{
						S: "invalid",
					},
				},
				PostalInfo: []entities.RDEContactPostalInfo{
					{
						Type: "int",
						Name: "name",
						Org:  "org",
						Address: entities.RDEAddress{
							Street:      []string{"street"},
							City:        "Ollantaytambo",
							PostalCode:  "pc",
							CountryCode: "PE",
						},
					},
				},
				Voice:  "+1.123345345",
				Fax:    "+1.123345345",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				CrRr:   "myRegstrarID",
				UpRr:   "myRegstrarID",
			},
			cmd:     nil,
			wantErr: entities.ErrInvalidContactStatus,
		},
		{
			name: "invalid status combination",
			rdeContact: &entities.RDEContact{
				ID:    "validID",
				Email: "email@me.com",
				ClID:  "myRegstrarID",
				Status: []entities.RDEContactStatus{
					{
						S: "OK",
					},
					{
						S: "pendingDelete",
					},
				},
				PostalInfo: []entities.RDEContactPostalInfo{
					{
						Type: "int",
						Name: "name",
						Org:  "org",
						Address: entities.RDEAddress{
							Street:      []string{"street"},
							City:        "Ollantaytambo",
							PostalCode:  "pc",
							CountryCode: "PE",
						},
					},
				},
				Voice:  "+1.123345345",
				Fax:    "+1.123345345",
				CrDate: "2021-01-01T00:00:00Z",
				UpDate: "2021-01-01T00:00:00Z",
				CrRr:   "myRegstrarID",
				UpRr:   "myRegstrarID",
			},
			cmd:     nil,
			wantErr: entities.ErrInvalidContactStatusCombination,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := FromRdeContact(tc.rdeContact)
			require.ErrorIs(t, err, tc.wantErr)
			if err == nil {
				require.Equal(t, tc.cmd, cmd)
			}
		})
	}
}
func TestCreateContactCommand_ToContact(t *testing.T) {
	testcases := []struct {
		name           string
		command        *CreateContactCommand
		expectedResult *entities.Contact
		expectedError  error
	}{
		{
			name: "valid without RoID",
			command: &CreateContactCommand{
				ID:        "validClID",
				Email:     "email@me.com",
				AuthInfo:  "escr0W1mP*rt",
				ClID:      "myRegstrarID",
				CrRr:      "myRegstrarID",
				CreatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				UpRr:      "myRegstrarID",
				UpdatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				Voice:     "+1.123345345",
				Fax:       "+1.123345345",
				Status: entities.ContactStatus{
					OK: true,
				},
			},
			expectedResult: &entities.Contact{
				ID:        "validClID",
				Email:     "email@me.com",
				AuthInfo:  "escr0W1mP*rt",
				ClID:      "myRegstrarID",
				CrRr:      "myRegstrarID",
				CreatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				UpRr:      "myRegstrarID",
				UpdatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				Voice:     "+1.123345345",
				Fax:       "+1.123345345",
				Status:    entities.ContactStatus{OK: true},
			},
			expectedError: nil,
		},
		{
			name: "valid with RoID",
			command: &CreateContactCommand{
				ID:        "validClID",
				RoID:      "12345_CONT-APEX",
				Email:     "email@me.com",
				AuthInfo:  "escr0W1mP*rt",
				ClID:      "myRegstrarID",
				CrRr:      "myRegstrarID",
				CreatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				UpRr:      "myRegstrarID",
				UpdatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				Voice:     "+1.123345345",
				Fax:       "+1.123345345",
				Status: entities.ContactStatus{
					OK: true,
				},
			},
			expectedResult: &entities.Contact{
				ID:        "validClID",
				RoID:      "12345_CONT-APEX",
				Email:     "email@me.com",
				AuthInfo:  "escr0W1mP*rt",
				ClID:      "myRegstrarID",
				CrRr:      "myRegstrarID",
				CreatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				UpRr:      "myRegstrarID",
				UpdatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				Voice:     "+1.123345345",
				Fax:       "+1.123345345",
				Status:    entities.ContactStatus{OK: true},
			},
			expectedError: nil,
		},
		{
			name: "invalid clID",
			command: &CreateContactCommand{
				ID:        "i",
				RoID:      "12345_CONT-APEX",
				Email:     "email@me.com",
				AuthInfo:  "escr0W1mP*rt",
				ClID:      "myRegstrarID",
				CrRr:      "myRegstrarID",
				CreatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				UpRr:      "myRegstrarID",
				UpdatedAt: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				Voice:     "+1.123345345",
				Fax:       "+1.123345345",
				Status: entities.ContactStatus{
					OK: true,
				},
			},
			expectedResult: nil,
			expectedError:  entities.ErrInvalidClIDType,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.command.ToContact()
			require.Equal(t, tc.expectedError, err)
			require.Equal(t, tc.expectedResult, result)
		})
	}
}
