package commands

import (
	"testing"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
)

func TestHostCommands_FromRdeHost(t *testing.T) {
	testcases := []struct {
		name    string
		rdeHost *entities.RDEHost
		cmd     *CreateHostCommand
		wantErr error
	}{
		{
			name: "valid host with valid roid",
			rdeHost: &entities.RDEHost{
				Name: "ns1.example.com",
				RoID: "123456_HOST-APEX",
				ClID: "sh8013",
				CrRr: "sh8013",
				UpRr: "sh8013",
			},
			cmd: &CreateHostCommand{
				Name: "ns1.example.com",
				ClID: "sh8013",
				CrRr: "sh8013",
				UpRr: "sh8013",
				Status: entities.HostStatus{
					OK: true,
				},
			},
			wantErr: nil,
		},
		{
			name: "valid host with INvalid roid",
			rdeHost: &entities.RDEHost{
				Name: "ns1.example.com",
				RoID: "123456",
				ClID: "sh8013",
				CrRr: "sh8013",
				UpRr: "sh8013",
			},
			cmd: &CreateHostCommand{
				Name: "ns1.example.com",
				ClID: "sh8013",
				CrRr: "sh8013",
				UpRr: "sh8013",
				Status: entities.HostStatus{
					OK: true,
				},
			},
			wantErr: nil,
		},
		{
			name: "invalid ClID",
			rdeHost: &entities.RDEHost{
				Name: "ns1.example.com",
				RoID: "123456",
				ClID: "d",
				CrRr: "sh8013",
				UpRr: "sh8013",
			},
			cmd:     nil,
			wantErr: entities.ErrInvalidClIDType,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &CreateHostCommand{}
			err := cmd.FromRdeHost(tc.rdeHost)
			assert.Equal(t, tc.wantErr, err)
			if err == nil {
				assert.Equal(t, tc.cmd, cmd)
			}
		})
	}
}

// TestFromRde_DoesNotKeepOrChangeTheDepositsROID pins that an escrow
// object's ROID never reaches the create command — every object gets a new
// one (ADR 0017) — and that the deposit's own record is left as it was.
func TestFromRde_DoesNotKeepOrChangeTheDepositsROID(t *testing.T) {
	host := &entities.RDEHost{Name: "ns1.example.com", RoID: "not a roid", ClID: "sh8013"}
	assert.NoError(t, (&CreateHostCommand{}).FromRdeHost(host))
	assert.Equal(t, "not a roid", host.RoID)

	contact := &entities.RDEContact{ID: "c1", RoID: "Dztys40879-RADIO", Email: "c@example.com", ClID: "sh8013",
		PostalInfo: []entities.RDEContactPostalInfo{{Type: "int", Name: "n", Address: entities.RDEAddress{City: "Lima", CountryCode: "PE"}}}}
	_ = (&CreateContactCommand{}).FromRdeContact(contact)
	assert.Equal(t, "Dztys40879-RADIO", contact.RoID)

	domain := &entities.RDEDomain{Name: "example.com", RoID: "7_DOM-ELSEWHERE", ClID: "sh8013"}
	_, _ = (&CreateDomainCommand{}).FromRdeDomain(domain)
	assert.Equal(t, "7_DOM-ELSEWHERE", domain.RoID)
}
