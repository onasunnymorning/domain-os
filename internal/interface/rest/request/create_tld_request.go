package request

import "github.com/onasunnymorning/domain-os/internal/application/commands"

type CreateTLDRequest struct {
	Name string `json:"Name" binding:"required"`
	RyID string `json:"RyID" binding:"required"`
}

func (r *CreateTLDRequest) ToCreateTLDCommand() (*commands.CreateTLDCommand, error) {
	return &commands.CreateTLDCommand{
		Name: r.Name,
		RyID: r.RyID,
	}, nil
}
