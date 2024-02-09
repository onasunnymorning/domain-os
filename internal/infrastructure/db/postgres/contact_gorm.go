package postgres

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// Contact is the Gorm model for the Contact entity.
type Contact struct {
	// ID is the ID of the contact as provided by the registrar.
	ID                       string `gorm:"primaryKey"`
	RoID                     int64  `gorm:"uniqueIndex;not null"` // It would be very unefficient to store the roid as a string so we use the int64 representation
	Voice                    string
	Fax                      string
	Email                    string
	ClID                     string
	CrRr                     string
	UpRr                     string
	AuthInfo                 string
	CreatedAt                time.Time
	UpdatedAt                time.Time
	NameInt                  string
	OrgInt                   string
	Street1Int               string
	Street2Int               string
	Street3Int               string
	CityInt                  string
	SPInt                    string `gorm:"column:sp_int"`
	PCInt                    string `gorm:"column:pc_int"`
	CCInt                    string `gorm:"column:cc_int"`
	NameLoc                  string
	OrgLoc                   string
	Street1Loc               string
	Street2Loc               string
	Street3Loc               string
	CityLoc                  string
	SPLoc                    string `gorm:"column:sp_loc"`
	PCLoc                    string `gorm:"column:pc_loc"`
	CCLoc                    string `gorm:"column:cc_loc"`
	entities.ContactStatus   `gorm:"embedded"`
	entities.ContactDisclose `gorm:"embedded"`
}

// TableName specifies the table name for contacts
func (r *Contact) TableName() string {
	return "contacts"
}
