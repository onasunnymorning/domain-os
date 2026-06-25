package activities

import (
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// DiffPlanResult captures the plan for registrar synchronization
type DiffPlanResult struct {
	Creates         []commands.CreateRegistrarCommand
	Updates         []commands.UpdateRegistrarStatusCommand
	SkippedReserved int
}

// DiffAndPlanRegistrars compares IANA registrars with existing platform registrars and produces a plan
// - Creates: new registrars to create (skips Reserved except special reserved GurIDs 9995, 9996, 9997)
// - Updates: status updates where IANA status and platform status differ (forces OK for special reserved GurIDs)
// - SkippedReserved: count of reserved registrars skipped
func DiffAndPlanRegistrars(correlationID string, iana []entities.IANARegistrar, existing []entities.RegistrarListItem) (DiffPlanResult, error) {
	result := DiffPlanResult{
		Creates: []commands.CreateRegistrarCommand{},
		Updates: []commands.UpdateRegistrarStatusCommand{},
	}

	// Build index of existing registrars by ClID string
	existingMap := make(map[string]entities.RegistrarListItem, len(existing))
	for _, r := range existing {
		existingMap[r.ClID.String()] = r
	}

	for _, i := range iana {
		clid, _ := i.CreateClID()
		clidStr := clid.String()

		if r, ok := existingMap[clidStr]; ok {
			// Consider status update
			if cmd := commands.CompareIANARegistrarStatusWithRarStatus(i, r); cmd != nil {
				// Exception for special reserved IANA Registrars (9995, 9996, 9997) — force OK
				if entities.IsSpecialReservedGurID(i.GurID) {
					cmd.NewStatus = string(entities.RegistrarStatusOK)
				}
				result.Updates = append(result.Updates, *cmd)
			}
			continue
		}

		// Not found: consider create
		if strings.EqualFold(i.Status.String(), string(entities.IANARegistrarStatusReserved)) && !entities.IsSpecialReservedGurID(i.GurID) {
			result.SkippedReserved++
			continue
		}

		if cmd, err := commands.CreateCreateRegistrarCommandFromIANARegistrar(i); err == nil && cmd != nil {
			result.Creates = append(result.Creates, *cmd)
		} else if err != nil {
			// Continue on error; surface via error if needed later
			// For now, ignore single item failure to be resilient
			continue
		}
	}

	return result, nil
}
