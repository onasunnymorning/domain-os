package postgres

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// Domain is the GORM model for the Domain entity
type Domain struct {
	RoID                          int64  `gorm:"primaryKey"`
	Name                          string `gorm:"uniqueIndex;not null"`
	OriginalName                  string
	UName                         string
	RegistrantID                  string
	AdminID                       string
	TechID                        string
	BillingID                     string
	ClID                          string
	CrRr                          *string
	UpRr                          *string
	TLDName                       string `gorm:"not null;foreignKey"`
	TLD                           TLD
	ExpiryDate                    time.Time `gorm:"not null;index"`
	DropCatch                     bool
	RenewedYears                  int
	AuthInfo                      string `gorm:"not null"`
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
	entities.DomainStatus         `gorm:"embedded"`
	entities.DomainRGPStatus      `gorm:"embedded"`
	entities.DomainGrandFathering `gorm:"embedded"`
	Hosts                         []Host `gorm:"many2many:domain_hosts;"`
}

// TableName returns the table name for the Domain model
func (Domain) TableName() string {
	return "domains"
}

// ToDomain converts a Domain to a domain model *entities.Domain
func ToDomain(dbDom *Domain) *entities.Domain {
	d := &entities.Domain{}
	roidString, _ := entities.NewRoidType(dbDom.RoID, entities.RoidTypeDomain)
	d.RoID = roidString
	d.Name = entities.DomainName(dbDom.Name)
	d.OriginalName = entities.DomainName(dbDom.OriginalName)
	d.UName = entities.DomainName(dbDom.UName)
	d.RegistrantID = entities.ClIDType(dbDom.RegistrantID)
	d.AdminID = entities.ClIDType(dbDom.AdminID)
	d.TechID = entities.ClIDType(dbDom.TechID)
	d.BillingID = entities.ClIDType(dbDom.BillingID)
	d.ClID = entities.ClIDType(dbDom.ClID)
	d.TLDName = entities.DomainName(dbDom.TLDName)
	d.ExpiryDate = dbDom.ExpiryDate
	d.DropCatch = dbDom.DropCatch
	d.RenewedYears = dbDom.RenewedYears
	d.AuthInfo = entities.AuthInfoType(dbDom.AuthInfo)
	d.CreatedAt = dbDom.CreatedAt
	d.UpdatedAt = dbDom.UpdatedAt
	d.Status = dbDom.DomainStatus
	d.RGPStatus = dbDom.DomainRGPStatus
	d.GrandFathering = dbDom.DomainGrandFathering
	if dbDom.CrRr != nil {
		d.CrRr = entities.ClIDType(*dbDom.CrRr)
	}
	if dbDom.UpRr != nil {
		d.UpRr = entities.ClIDType(*dbDom.UpRr)
	}

	for _, h := range dbDom.Hosts {
		d.Hosts = append(d.Hosts, ToHost(&h))
	}

	return d
}

// FromDomain converts a domain model *entities.Domain to a Domain
func ToDBDomain(d *entities.Domain) *Domain {
	dbDomain := &Domain{}
	dbDomain.RoID, _ = d.RoID.Int64()
	dbDomain.Name = d.Name.String()
	dbDomain.OriginalName = d.OriginalName.String()
	dbDomain.UName = d.UName.String()
	dbDomain.RegistrantID = d.RegistrantID.String()
	dbDomain.AdminID = d.AdminID.String()
	dbDomain.TechID = d.TechID.String()
	dbDomain.BillingID = d.BillingID.String()
	dbDomain.ClID = d.ClID.String()
	dbDomain.TLDName = d.TLDName.String()
	dbDomain.ExpiryDate = d.ExpiryDate
	dbDomain.DropCatch = d.DropCatch
	dbDomain.RenewedYears = d.RenewedYears
	dbDomain.AuthInfo = d.AuthInfo.String()
	dbDomain.CreatedAt = d.CreatedAt
	dbDomain.UpdatedAt = d.UpdatedAt
	dbDomain.DomainStatus = d.Status
	dbDomain.DomainRGPStatus = d.RGPStatus
	dbDomain.DomainGrandFathering = d.GrandFathering

	if d.CrRr != entities.ClIDType("") {
		rar := d.CrRr.String()
		dbDomain.CrRr = &rar
	}
	if d.UpRr != entities.ClIDType("") {
		rar := d.UpRr.String()
		dbDomain.UpRr = &rar
	}

	for _, h := range d.Hosts {
		dbDomain.Hosts = append(dbDomain.Hosts, *ToDBHost(h))
	}

	return dbDomain
}
