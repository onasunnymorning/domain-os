package commands

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestCreateDomainCommand_FromRdeDomain(t *testing.T) {
	contacts := []entities.RDEDomainContact{
		{
			Type: "admin",
			ID:   "test-admin",
		},
		{
			Type: "tech",
			ID:   "test-tech",
		},
		{
			Type: "billing",
			ID:   "test-billing",
		},
	}

	tetcases := []struct {
		name      string
		rdeDomain *entities.RDEDomain
		cmd       *CreateDomainCommand
		wantErr   error
	}{
		{
			name: "valid RDEDomain with valid Roid",
			rdeDomain: &entities.RDEDomain{
				RoID:       "12345_DOM-APEX",
				Name:       "example.com",
				ClID:       "test",
				CrDate:     "2020-01-01T00:00:00Z",
				ExDate:     "2021-01-01T00:00:00Z",
				CrRr:       "test",
				UpRr:       "test",
				Registrant: "test-registrant",
				Contact:    contacts,
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
				RegistrantID: "test-registrant",
				AdminID:      "test-admin",
				TechID:       "test-tech",
				BillingID:    "test-billing",
			},
			wantErr: nil,
		},
		{
			name: "valid RDEDomain with INvalid Roid",
			rdeDomain: &entities.RDEDomain{
				RoID:       "12345",
				Name:       "example.com",
				ClID:       "test",
				CrDate:     "2020-01-01T00:00:00Z",
				ExDate:     "2021-01-01T00:00:00Z",
				CrRr:       "test",
				UpRr:       "test",
				Registrant: "test-registrant",
				Contact:    contacts,
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
				RegistrantID: "test-registrant",
				AdminID:      "test-admin",
				TechID:       "test-tech",
				BillingID:    "test-billing",
			},
			wantErr: nil,
		},
		{
			name: "invalid ClID",
			rdeDomain: &entities.RDEDomain{
				RoID:       "12345",
				Name:       "example.com",
				ClID:       "r",
				CrRr:       "test",
				UpRr:       "test",
				Contact:    contacts,
				Registrant: "test-registrant",
			},
			cmd:     nil,
			wantErr: entities.ErrInvalidClIDType,
		},
		{
			name: "missing registrant",
			rdeDomain: &entities.RDEDomain{
				RoID:    "12345",
				Name:    "example.com",
				ClID:    "r",
				CrRr:    "test",
				UpRr:    "test",
				Contact: contacts,
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
func TestUpdateDomainCommand_FromEntity(t *testing.T) {
	dom := &entities.Domain{
		OriginalName: entities.DomainName("example.com"),
		UName:        entities.DomainName("example.com"),
		RegistrantID: entities.ClIDType("registrant"),
		AdminID:      entities.ClIDType("admin"),
		TechID:       entities.ClIDType("tech"),
		BillingID:    entities.ClIDType("billing"),
		ClID:         entities.ClIDType("client"),
		CrRr:         entities.ClIDType("test"),
		UpRr:         entities.ClIDType("test"),
		ExpiryDate:   time.Now(),
		AuthInfo:     entities.AuthInfoType("authinfo"),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Status:       entities.DomainStatus{},
		RGPStatus:    entities.DomainRGPStatus{},
	}

	cmd := &UpdateDomainCommand{}
	cmd.FromEntity(dom)

	require.Equal(t, dom.OriginalName.String(), cmd.OriginalName)
	require.Equal(t, dom.UName.String(), cmd.UName)
	require.Equal(t, dom.RegistrantID.String(), cmd.RegistrantID)
	require.Equal(t, dom.AdminID.String(), cmd.AdminID)
	require.Equal(t, dom.TechID.String(), cmd.TechID)
	require.Equal(t, dom.BillingID.String(), cmd.BillingID)
	require.Equal(t, dom.ClID.String(), cmd.ClID)
	require.Equal(t, dom.CrRr.String(), cmd.CrRr)
	require.Equal(t, dom.UpRr.String(), cmd.UpRr)
	require.Equal(t, dom.ExpiryDate, cmd.ExpiryDate)
	require.Equal(t, dom.AuthInfo.String(), cmd.AuthInfo)
	require.Equal(t, dom.CreatedAt, cmd.CreatedAt)
	require.Equal(t, dom.UpdatedAt, cmd.UpdatedAt)
	require.Equal(t, dom.Status, cmd.Status)
	require.Equal(t, dom.RGPStatus, cmd.RGPStatus)
}
