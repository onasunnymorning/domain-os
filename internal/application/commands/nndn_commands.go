package commands

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// CreateNNDNCommand is the command to create a NNDN
type CreateNNDNCommand struct {
	Name   string `json:"name" binding:"required"`
	Reason string `json:"reason"`
}

// FromRDENNDN creates a new CreateNNDNCommand from an RDENNDN
func (cmd *CreateNNDNCommand) FromRDENNDN(rdeNNDN *entities.RDENNDN) error {
	nndn, err := entities.NewNNDN(rdeNNDN.AName)
	if err != nil {
		return err
	}
	cmd.Name = nndn.Name.String()
	return nil
}
