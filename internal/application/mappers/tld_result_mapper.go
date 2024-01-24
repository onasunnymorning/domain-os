package mappers

import (
	"github.com/onasunnymorning/registry-os/internal/application/commands"
	"github.com/onasunnymorning/registry-os/internal/domain/entities"
)

// NewTLDResultFromTLD converts an entity TLD to a command TLDResult
func NewTLDResultFromTLD(tld *entities.TLD) commands.TLDResult {
	return commands.TLDResult{
		Name:      tld.Name.String(),
		Type:      tld.Type.String(),
		UName:     tld.UName,
		CreatedAt: tld.CreatedAt,
		UpdatedAt: tld.UpdatedAt,
	}
}
