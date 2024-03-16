package commands

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// CreateHostCommand is the command to create a Host
type CreateHostCommand struct {
	Name      string              `json:"Name" binding:"required"`
	Addresses []string            `json:"Addresses"`
	ClID      entities.ClIDType   `json:"ClID" example:"sh8013"`
	CrRr      entities.ClIDType   `json:"CrRR" example:"sh8013"`
	UpRr      entities.ClIDType   `json:"UpRR" example:"sh8013"`
	Status    entities.HostStatus `json:"Status"`
}
