package commands

import "time"

import "github.com/onasunnymorning/domain-os/pkg/domain/entities"

// SaveEPPPolicyCommand creates or replaces an EPP policy (PUT
// /epp/policies/{name}). Every limit must be given, at least 1, except
// creates_per_day, where 0 is no cap.
type SaveEPPPolicyCommand struct {
	Description string                   `json:"description"`
	Limits      entities.EPPPolicyLimits `json:"limits" binding:"required"`
	// CreateOnly refuses to replace an existing policy (set from the
	// request's If-None-Match: *, never from the body).
	CreateOnly bool `json:"-"`
	// ExpectedUpdatedAt, when set, refuses the save unless the stored
	// policy's UpdatedAt is exactly this (from If-Match, never the body):
	// an edit that started from a version since replaced is not applied.
	ExpectedUpdatedAt *time.Time `json:"-"`
}

// AssignEPPPolicyCommand assigns a registrar its EPP policy (PUT
// /registrars/{clid}/epp/policy). "default", or empty, removes the
// assignment.
type AssignEPPPolicyCommand struct {
	Policy string `json:"policy"`
}

// SaveRegistrarFamilyCommand creates or replaces a registrar family (PUT
// /epp/families/{name}), its members included.
type SaveRegistrarFamilyCommand struct {
	Description string              `json:"description"`
	Policy      string              `json:"policy"`
	Members     []entities.ClIDType `json:"members"`
	// CreateOnly refuses to replace an existing family (set from the
	// request's If-None-Match: *, never from the body).
	CreateOnly bool `json:"-"`
	// ExpectedUpdatedAt is as for SaveEPPPolicyCommand.
	ExpectedUpdatedAt *time.Time `json:"-"`
}
