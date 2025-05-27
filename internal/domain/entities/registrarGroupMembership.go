package entities

import "time"

type RegistrarGroupMembership struct {
	GroupID     RegistrarGroupID
	RegistrarID ClIDType
	CreatedAt   time.Time
}
