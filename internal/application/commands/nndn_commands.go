package commands

import "github.com/onasunnymorning/domain-os/pkg/domain/entities"

// NNDNReasonEscrowImport is the Reason stamped on every NNDN created from an
// escrow (RDE) deposit, by both the CLI and the workflow importer, so imported
// names can be found with a single reason filter.
const NNDNReasonEscrowImport = "RDE-import"

// CreateNNDNCommand is the command to create a NNDN
type CreateNNDNCommand struct {
	Name   string `json:"name" binding:"required"`
	Reason string `json:"reason" binding:"required"`
}

// FromRDENNDN creates a new CreateNNDNCommand from an RDENNDN
func (cmd *CreateNNDNCommand) FromRDENNDN(rdeNNDN *entities.RDENNDN) error {
	nndn, err := entities.NewNNDN(rdeNNDN.AName)
	if err != nil {
		return err
	}
	cmd.Name = nndn.Name.String()
	cmd.Reason = NNDNReasonEscrowImport
	return nil
}
