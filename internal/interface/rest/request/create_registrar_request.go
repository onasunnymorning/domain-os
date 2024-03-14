package request

import "github.com/onasunnymorning/domain-os/internal/application/commands"

type CreateRegistrarRequest struct {
	ClID  string `json:"Clid" binding:"required"`
	Name  string `json:"Name" binding:"required"`
	Email string `json:"Email" binding:"required"`
	GurID int    `json:"GurID"`
}

func (r *CreateRegistrarRequest) ToCreateRegistrarCommand() (*commands.CreateRegistrarCommand, error) {
	return &commands.CreateRegistrarCommand{
		ClID:  r.ClID,
		Name:  r.Name,
		Email: r.Email,
		GurID: r.GurID,
	}, nil
}

type CreateRegistrarFromGurIDRequest struct {
	Email string `json:"Email" binding:"required"`
}
