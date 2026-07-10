package activities

import (
	"context"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/icannregistrars"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// MakeCreateRegistrarCommands generates a list of CreateRegistrarCommand based on the provided ICANN and IANA registrars.
// It takes a correlation ID, a slice of ICANN CSVRegistrar, and a slice of IANARegistrar as input parameters.
// It returns a slice of CreateRegistrarCommand and an error if any occurs during the command generation process.
func MakeCreateRegistrarCommands(ctx context.Context, correlationID string, icannRars []icannregistrars.CSVRegistrar, ianaRars []entities.IANARegistrar) ([]commands.CreateRegistrarCommand, error) {
	cmds, err := icannregistrars.GetCreateCommands(icannRars, ianaRars)
	if err != nil {
		return nil, err
	}

	return cmds, nil
}
