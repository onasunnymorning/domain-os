package commands

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
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
				RoID: "123456_HOST-APEX",
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
