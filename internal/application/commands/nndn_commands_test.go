package commands

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestCreateNNDNCommand_FromRDENNDN(t *testing.T) {
	testcases := []struct {
		name    string
		rdeNNDN *entities.RDENNDN
		cmd     *CreateNNDNCommand
		wantErr error
	}{
		{
			name: "success",
			rdeNNDN: &entities.RDENNDN{
				AName: "example.com",
			},
			cmd: &CreateNNDNCommand{
				Name: "example.com",
			},
			wantErr: nil,
		},
		{
			name: "invalid name",
			rdeNNDN: &entities.RDENNDN{
				AName: "ex--ample.com",
			},
			cmd:     nil,
			wantErr: entities.ErrInvalidLabelDoubleDash,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &CreateNNDNCommand{}
			err := cmd.FromRDENNDN(tc.rdeNNDN)
			require.ErrorIs(t, err, tc.wantErr, "Error mismatch")

			if err == nil {
				require.Equal(t, tc.cmd, cmd)
			}
		})
	}

}
