package request

import "github.com/onasunnymorning/domain-os/internal/application/commands"

type CreateRegistrarRequest struct {
	ClID  string `json:"clid" binding:"required"`
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required"`
	GurID int    `json:"gurid"`
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
	Email string `json:"email" binding:"required"`
}
