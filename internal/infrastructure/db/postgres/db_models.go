package postgres

import "time"

// TLD is a struct representing a TLD in the database
type TLD struct {
	Name      string `gorm:"primary_key"`
	Type      string
	UName     string
	CreatedAt time.Time
	UpdatedAt time.Time
}
