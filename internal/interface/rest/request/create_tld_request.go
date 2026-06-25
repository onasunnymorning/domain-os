package request

import "github.com/onasunnymorning/domain-os/internal/application/commands"

type CreateTLDRequest struct {
	Name                     string `json:"Name" binding:"required"`
	RyID                     string `json:"RyID" binding:"required"`
	CreateOperatorRegistrars *bool  `json:"CreateOperatorRegistrars,omitempty"` // defaults to true if nil
	AllowEscrowImport        *bool  `json:"AllowEscrowImport,omitempty"`        // defaults to true if nil
}

func (r *CreateTLDRequest) ToCreateTLDCommand() (*commands.CreateTLDCommand, error) {
	// Default to true if not explicitly set
	createOpRars := true
	if r.CreateOperatorRegistrars != nil {
		createOpRars = *r.CreateOperatorRegistrars
	}
	allowEscrow := true
	if r.AllowEscrowImport != nil {
		allowEscrow = *r.AllowEscrowImport
	}
	return &commands.CreateTLDCommand{
		Name:                     r.Name,
		RyID:                     r.RyID,
		CreateOperatorRegistrars: createOpRars,
		AllowEscrowImport:        allowEscrow,
	}, nil
}
