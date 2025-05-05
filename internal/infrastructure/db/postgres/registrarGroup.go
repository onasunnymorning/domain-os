package postgres

import "time"

// RegistrarGroup represents a registrar group entity in our repository
type RegistrarGroup struct {
	ID               string `gorm:"primaryKey"`
	Name             string `gorm:"not null"`
	Description      string `gorm:"not null"`
	RegistryOperator RegistryOperator
	ParentGroup      *RegistrarGroup `gorm:"foreignKey:ParentGroupID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	CreatedAt        time.Time       `gorm:"not null"`
	UpdatedAt        time.Time       `gorm:"not null"`
}
