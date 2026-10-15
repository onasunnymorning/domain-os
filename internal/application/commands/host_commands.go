package commands

import "github.com/onasunnymorning/domain-os/pkg/domain/entities"

// CreateHostCommand is the command to create a Host
type CreateHostCommand struct {
	Name      string              `json:"Name" binding:"required"`
	Addresses []string            `json:"Addresses"`
	ClID      entities.ClIDType   `json:"ClID" example:"sh8013"`
	CrRr      entities.ClIDType   `json:"CrRr" example:"sh8013"`
	UpRr      entities.ClIDType   `json:"UpRr" example:"sh8013"`
	Status    entities.HostStatus `json:"Status"`
}

// FromRdeHost converts an RDEHost to a CreateHostCommand
func (cmd *CreateHostCommand) FromRdeHost(rdeHost *entities.RDEHost) error {
	// The deposit's ROID is never kept: every object gets a new ROID of this
	// registry's when it is created (ADR 0017). A placeholder lets ToEntity
	// validate the rest, without touching the caller's RDEHost.
	r := *rdeHost
	r.RoID = "1_HOST-RDE"

	// Create a Host Entity from the RDEHost, this will validate the Host
	host, err := r.ToEntity()
	if err != nil {
		return err
	}

	cmd.Name = host.Name.String()
	cmd.ClID = host.ClID
	cmd.CrRr = host.CrRr
	cmd.UpRr = host.UpRr
	cmd.Status = host.Status

	for _, a := range host.Addresses {
		cmd.Addresses = append(cmd.Addresses, a.String())
	}

	return nil

}
