package postgres

import "time"

// RegistrarGroupMembership represents a registrar group membership entity in our repository
type RegistrarGroupMembership struct {
	GroupID     string `gorm:"primary_key"`
	RegistrarID string `gorm:"not null"`
	AddedAt     time.Time
}
