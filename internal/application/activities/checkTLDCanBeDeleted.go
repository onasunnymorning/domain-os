package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
)

type CheckTLDCanBeDeletedArgs struct {
	TLD              string
	KeepTLDAndPhases bool
}

type CheckTLDCanBeDeletedResult struct {
	CanBeDeleted bool
	Reason       string
}

// CheckTLDCanBeDeleted ensures the TLD is safe to be deleted according to business rules.
func (a *TLDCleanupActivities) CheckTLDCanBeDeleted(ctx context.Context, args CheckTLDCanBeDeletedArgs) (CheckTLDCanBeDeletedResult, error) {
	if args.TLD == "" {
		return CheckTLDCanBeDeletedResult{}, fmt.Errorf("tld is required")
	}

	db := a.DB

	// Rule 1: TLD must exist
	var tldCount int64
	if err := db.Model(&postgres.TLD{}).Where("name = ?", args.TLD).Count(&tldCount).Error; err != nil {
		return CheckTLDCanBeDeletedResult{}, fmt.Errorf("failed to query tld: %w", err)
	}
	if tldCount == 0 {
		return CheckTLDCanBeDeletedResult{CanBeDeleted: false, Reason: "TLD does not exist"}, nil
	}

	// Wait, if KeepTLDAndPhases is true, we ONLY need to verify that we aren't interrupting something critical like an active import,
	// but the business rule "no active phase, no future phases" applies only when doing a full wipe!
	// If keep is true, the user is explicitly wiping domains/hosts/contacts to re-do an import. There's no reason to block on Phase checks.
	if !args.KeepTLDAndPhases {
		// Verify Phases: No active, no future phases
		var phases []postgres.Phase
		if err := db.Where("tld_name = ?", args.TLD).Find(&phases).Error; err != nil {
			return CheckTLDCanBeDeletedResult{}, fmt.Errorf("failed to query phases: %w", err)
		}

		now := time.Now().UTC()
		for _, p := range phases {
			// Phase is active if it started in the past AND (has no end, or end is in the future)
			active := p.Starts.Before(now) && (p.Ends == nil || p.Ends.After(now))
			future := p.Starts.After(now)
			if active {
				return CheckTLDCanBeDeletedResult{CanBeDeleted: false, Reason: fmt.Sprintf("Phase %s is currently active", p.Name)}, nil
			}
			if future {
				return CheckTLDCanBeDeletedResult{CanBeDeleted: false, Reason: fmt.Sprintf("Phase %s is scheduled for the future", p.Name)}, nil
			}
		}
	}

	return CheckTLDCanBeDeletedResult{CanBeDeleted: true, Reason: ""}, nil
}
