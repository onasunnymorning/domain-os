package postgres

import "time"

// Registrar is the GORM representation of a Registrar
type Registrar struct {
	ClID        string `gorm:"primary_key"`
	Name        string `gorm:"unique;not null"`
	NickName    string `gorm:"unique;not null"`
	GurID       int
	Email       string
	Status      string `gorm:"not null"`
	Street1Int  string
	Street2Int  string
	Street3Int  string
	CityInt     string
	SPInt       string `gorm:"column:sp_int"`
	PCInt       string `gorm:"column:pc_int"`
	CCInt       string `gorm:"column:cc_int"`
	Street1Loc  string
	Street2Loc  string
	Street3Loc  string
	CityLoc     string
	SPLoc       string `gorm:"column:sp_loc"`
	PCLoc       string `gorm:"column:pc_loc"`
	CCLoc       string `gorm:"column:cc_loc"`
	Voice       string
	Fax         string
	URL         string
	Whois43     string
	Whois80     string
	RdapBaseUrl string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Registrar) TableName() string {
	return "registrars"
}
