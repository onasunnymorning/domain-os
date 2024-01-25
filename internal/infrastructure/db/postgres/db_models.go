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

// IANARegistrar is a struct representing an IANA Registrar in the database
type IANARegistrar struct {
	GurID     int `gorm:"primary_key;auto_increment:false"`
	Name      string
	Status    string
	Updated   string
	RdapURL   string
	CreatedAt time.Time
	UpdateAt  time.Time
}
