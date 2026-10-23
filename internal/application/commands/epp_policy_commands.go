package commands

import "github.com/onasunnymorning/domain-os/pkg/domain/entities"

// SaveEPPPolicyCommand creates or replaces an EPP policy (PUT
// /epp/policies/{name}). Every limit must be given, at least 1, except
// creates_per_day, where 0 is no cap.
type SaveEPPPolicyCommand struct {
	Description string                   `json:"description"`
	Limits      entities.EPPPolicyLimits `json:"limits" binding:"required"`
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
}
