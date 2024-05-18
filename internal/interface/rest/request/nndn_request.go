package request

import (
	"github.com/onasunnymorning/domain-os/internal/application/commands"
)

// CreateNNDNRequest defines the structure for the create NNDN request payload
type CreateNNDNRequest struct {
	Name   string `json:"Name" binding:"required"`
	Reason string `json:"Reason"`
}

// ToCreateNNDNCommand converts the CreateNNDNRequest to CreateNNDNCommand used by the service layer
func (req *CreateNNDNRequest) ToCreateNNDNCommand() (*commands.CreateNNDNCommand, error) {
	// add any necessary validation or transformation logic
	return &commands.CreateNNDNCommand{
		Name:   req.Name,
		Reason: req.Reason,
	}, nil
}
