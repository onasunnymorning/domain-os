package commands

import (
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// SaveLifecyclePolicyCommand creates or replaces a lifecycle policy (PUT
// /lifecycle-policies/{name}). A setting left null is the policy's
// default; 0 is zero days.
type SaveLifecyclePolicyCommand struct {
	Description string                     `json:"description"`
	Settings    entities.LifecycleSettings `json:"settings"`
	// CreateOnly refuses to replace an existing policy (set from the
	// request's If-None-Match: *, never from the body).
	CreateOnly bool `json:"-"`
	// ExpectedUpdatedAt, when set, refuses the save unless the stored
	// policy's UpdatedAt is exactly this (from If-Match, never the body).
	ExpectedUpdatedAt *time.Time `json:"-"`
}

// SetPhaseLifecyclePolicyCommand names the lifecycle policy a phase
// follows (PUT /tlds/{tld}/phases/{phase}/lifecycle-policy).
type SetPhaseLifecyclePolicyCommand struct {
	LifecyclePolicy string `json:"lifecyclePolicy" binding:"required" example:"gtld-standard"`
	TLDName         string `json:"-"`
	PhaseName       string `json:"-"`
}
