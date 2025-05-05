package entities

import "time"

type RegistrarGroupID string

// RegistrarGroup represents a group of registrars for a specific registry operator.
// These groups are used to manage the registrars and their associated domains, promotions, and other related entities.
// It allows nested groups, where a group can have sub-groups.
// A top-level group has a nil ParentGroupID.
type RegistrarGroup struct {
	ID                 RegistrarGroupID
	Name               string
	Description        string
	RegistryOperatorID ClIDType          // links to RegistryOperator
	ParentGroupID      *RegistrarGroupID // nil if top-level group
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
