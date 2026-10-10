package activities

import (
	"context"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// DiffPlanResult captures the plan for registrar synchronization
type DiffPlanResult struct {
	Creates         []commands.CreateRegistrarCommand
	Updates         []commands.UpdateRegistrarStatusCommand
	SkippedReserved int
	// RenamedForUniqueness lists the ClIDs of planned creates whose IANA name
	// collided with a registrar already in the registry (or earlier in this
	// plan) and were given a "-N" suffix, as the bootstrap import does.
	RenamedForUniqueness []string
}

// DiffAndPlanRegistrars compares IANA registrars with existing platform registrars and produces a plan
// - Creates: new registrars to create (skips Reserved except special reserved GurIDs 9995, 9996, 9997)
// - Updates: status updates where IANA status and platform status differ (special reserved GurIDs expect platform status OK)
// - SkippedReserved: count of reserved registrars skipped
//
// Registrar names are unique in the registry but IANA's are not, so a create whose
// name is already taken is planned with a "-N" suffix. Without that the insert is
// silently dropped on the unique constraint and the same create is re-planned on
// every run.
func DiffAndPlanRegistrars(ctx context.Context, correlationID string, iana []entities.IANARegistrar, existing []entities.RegistrarListItem) (DiffPlanResult, error) {
	result := DiffPlanResult{
		Creates: []commands.CreateRegistrarCommand{},
		Updates: []commands.UpdateRegistrarStatusCommand{},
	}

	// Build index of existing registrars by ClID string
	existingMap := make(map[string]entities.RegistrarListItem, len(existing))
	usedNames := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		existingMap[r.ClID.String()] = r
		usedNames[entities.NormalizeString(r.Name)] = struct{}{}
	}

	for _, i := range iana {
		clid, _ := i.CreateClID()
		clidStr := clid.String()

		if r, ok := existingMap[clidStr]; ok {
			// Consider status update. CompareIANARegistrarStatusWithRarStatus
			// already accounts for special reserved registrars (9995, 9996,
			// 9997) by expecting platform status "ok", so no override is needed
			// here — and it returns nil (no update) once they are in sync.
			if cmd := commands.CompareIANARegistrarStatusWithRarStatus(i, r); cmd != nil {
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
			if unique := entities.UniqueRegistrarName(cmd.Name, usedNames); unique != cmd.Name {
				cmd.Name = unique
				result.RenamedForUniqueness = append(result.RenamedForUniqueness, cmd.ClID)
			}
			result.Creates = append(result.Creates, *cmd)
		} else if err != nil {
			// Continue on error; surface via error if needed later
			// For now, ignore single item failure to be resilient
			continue
		}
	}

	return result, nil
}
