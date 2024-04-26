package commands

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// CreateHostCommand is the command to create a Host
type CreateHostCommand struct {
	RoID      string              `json:"RoID"` // if not provided, it will be generated
	Name      string              `json:"Name" binding:"required"`
	Addresses []string            `json:"Addresses"`
	ClID      entities.ClIDType   `json:"ClID" example:"sh8013"`
	CrRr      entities.ClIDType   `json:"CrRr" example:"sh8013"`
	UpRr      entities.ClIDType   `json:"UpRr" example:"sh8013"`
	Status    entities.HostStatus `json:"Status"`
}

// FromRdeHost converts an RDEHost to a CreateHostCommand
func (cmd *CreateHostCommand) FromRdeHost(rdeHost *entities.RDEHost) error {
	// Check if we have a valid RoID (this will only be the case if we are importing our own escrows).
	// If the Roid is invalid, use a valid one to pass through domain validation and unset it in the final command to have one generated.
	roid := entities.RoidType(rdeHost.RoID)
	if roid.Validate() != nil || roid.ObjectIdentifier() != "HOST" {
		// set a dummy valid RoID to pass through domain validation
		rdeHost.RoID = "1_HOST-APEX"
	}

	// Create a Host Entity from the RDEHost, this will validate the Host
	host, err := rdeHost.ToEntity()
	if err != nil {
		return err
	}

	// Now that we have a valid Host, convert it to a command
	// Only set the RoID if it is not the dummy RoID
	if host.RoID.String() != "1_HOST-APEX" {
		cmd.RoID = host.RoID.String()
	}
	cmd.Name = host.Name.String()
	cmd.ClID = host.ClID
	cmd.CrRr = host.CrRr
	cmd.UpRr = host.UpRr
	cmd.Status = host.Status

	return nil

}
