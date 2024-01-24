package interfaces

import (
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type TLDService interface {
	CreateTLD(cmd *commands.CreateTLDCommand) (*commands.CreateTLDCommandResult, error)
	GetTLDByName(name string) (*entities.TLD, error)
	ListTLDs(pageSize int, pageCursor string) ([]*entities.TLD, error)
	DeleteTLDByName(name string) error
	// UpdateTLD(cmd *commands.UpdateTLDCommand) (*commands.UpdateTLDCommandResult, error)
}
