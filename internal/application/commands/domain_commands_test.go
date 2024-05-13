package commands

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestCreateDomainCommand_FromRdeDomain(t *testing.T) {
	tetcases := []struct {
		name      string
		rdeDomain *entities.RDEDomain
		cmd       *CreateDomainCommand
		wantErr   error
	}{
		{
			name: "valid RDEDomain with valid Roid",
			rdeDomain: &entities.RDEDomain{
				RoID:   "12345_DOM-APEX",
				Name:   "example.com",
				ClID:   "test",
				CrDate: "2020-01-01T00:00:00Z",
				ExDate: "2021-01-01T00:00:00Z",
				CrRr:   "test",
				UpRr:   "test",
			},
			cmd: &CreateDomainCommand{
				RoID:       "12345_DOM-APEX",
				Name:       "example.com",
				ClID:       "test",
				CrRr:       "test",
				UpRr:       "test",
				CreatedAt:  time.Time(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
				ExpiryDate: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				AuthInfo:   "escr0W1mP*rt",
				Status: entities.DomainStatus{
					Inactive: true,
				},
			},
			wantErr: nil,
		},
		{
			name: "valid RDEDomain with INvalid Roid",
			rdeDomain: &entities.RDEDomain{
				RoID:   "12345",
				Name:   "example.com",
				ClID:   "test",
				CrDate: "2020-01-01T00:00:00Z",
				ExDate: "2021-01-01T00:00:00Z",
				CrRr:   "test",
				UpRr:   "test",
			},
			cmd: &CreateDomainCommand{
				Name:       "example.com",
				ClID:       "test",
				CrRr:       "test",
				UpRr:       "test",
				CreatedAt:  time.Time(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
				ExpiryDate: time.Time(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				AuthInfo:   "escr0W1mP*rt",
				Status: entities.DomainStatus{
					Inactive: true,
				},
			},
			wantErr: nil,
		},
		{
			name: "invalid ClID",
			rdeDomain: &entities.RDEDomain{
				RoID: "12345",
				Name: "example.com",
				ClID: "r",
				CrRr: "test",
				UpRr: "test",
			},
			cmd:     nil,
			wantErr: entities.ErrInvalidClIDType,
		},
	}

	for _, tc := range tetcases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &CreateDomainCommand{}
			err := cmd.FromRdeDomain(tc.rdeDomain)
			require.Equal(t, tc.wantErr, err)
			if err == nil {
				require.Equal(t, tc.cmd, cmd)
			}
		})
	}

}
