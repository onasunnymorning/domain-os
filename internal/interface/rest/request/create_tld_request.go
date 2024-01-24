package request

import "github.com/onasunnymorning/registry-os/internal/application/commands"

type CreateTLDRequest struct {
	Name string `json:"name" binding:"required"`
}

func (r *CreateTLDRequest) ToCreateTLDCommand() (*commands.CreateTLDCommand, error) {
	return &commands.CreateTLDCommand{
		Name: r.Name,
	}, nil
}
