package mappers

import (
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// NewTLDResultFromTLD converts an entity TLD to a command TLDResult
func NewTLDResultFromTLD(tld *entities.TLD) commands.TLDResult {
	return commands.TLDResult{
		Name:      tld.Name.String(),
		Type:      tld.Type.String(),
		UName:     tld.UName.String(),
		CreatedAt: tld.CreatedAt,
		UpdatedAt: tld.UpdatedAt,
	}
}
