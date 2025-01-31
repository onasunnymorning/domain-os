package postgres

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// Registrar is the GORM representation of a Registrar
type Registrar struct {
	ClID        string `gorm:"primary_key"`
	Name        string `gorm:"unique;not null"`
	NickName    string `gorm:"unique;not null"`
	GurID       int
	Email       string
	Status      string `gorm:"not null"`
	IANAStatus  string
	Autorenew   bool
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

	// FK relationships with contacts
	Contacts        []*Contact `gorm:"foreignKey:ClID"`
	ContactsCreated []*Contact `gorm:"foreignKey:CrRr"`
	ContactsUpdated []*Contact `gorm:"foreignKey:UpRr"`

	// FK relationships with hosts
	Hosts        []*Host `gorm:"foreignKey:ClID"`
	HostsCreated []*Host `gorm:"foreignKey:CrRr"`
	HostsUpdated []*Host `gorm:"foreignKey:UpRr"`

	// FK relationships with domains
	Domains        []*Domain `gorm:"foreignKey:ClID"`
	DomainsCreated []*Domain `gorm:"foreignKey:CrRr"`
	DomainsUpdated []*Domain `gorm:"foreignKey:UpRr"`

	// Many to Many relationship with TLDs
	TLDs []TLD `gorm:"many2many:accreditations;"`
}

func (Registrar) TableName() string {
	return "registrars"
}

func ToDBRegistrar(r *entities.Registrar) *Registrar {
	rar := &Registrar{
		ClID:        r.ClID.String(),
		Name:        r.Name,
		NickName:    r.NickName,
		GurID:       r.GurID,
		Email:       r.Email,
		Status:      r.Status.String(),
		IANAStatus:  r.IANAStatus.String(),
		Autorenew:   r.Autorenew,
		Voice:       r.Voice.String(),
		Fax:         r.Fax.String(),
		URL:         r.URL.String(),
		Whois43:     r.WhoisInfo.Name.String(),
		Whois80:     r.WhoisInfo.URL.String(),
		RdapBaseUrl: r.RdapBaseURL.String(),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}

	if r.PostalInfo[0] != nil {
		if r.PostalInfo[0].Address != nil {
			rar.Street1Int = r.PostalInfo[0].Address.Street1.String()
			rar.Street2Int = r.PostalInfo[0].Address.Street2.String()
			rar.Street3Int = r.PostalInfo[0].Address.Street3.String()
			rar.CityInt = r.PostalInfo[0].Address.City.String()
			rar.SPInt = r.PostalInfo[0].Address.StateProvince.String()
			rar.PCInt = r.PostalInfo[0].Address.PostalCode.String()
			rar.CCInt = r.PostalInfo[0].Address.CountryCode.String()
		}
	}

	if r.PostalInfo[1] != nil {
		if r.PostalInfo[1].Address != nil {
			rar.Street1Loc = r.PostalInfo[1].Address.Street1.String()
			rar.Street2Loc = r.PostalInfo[1].Address.Street2.String()
			rar.Street3Loc = r.PostalInfo[1].Address.Street3.String()
			rar.CityLoc = r.PostalInfo[1].Address.City.String()
			rar.SPLoc = r.PostalInfo[1].Address.StateProvince.String()
			rar.PCLoc = r.PostalInfo[1].Address.PostalCode.String()
			rar.CCLoc = r.PostalInfo[1].Address.CountryCode.String()
		}
	}

	for _, tld := range r.TLDs {
		rar.TLDs = append(rar.TLDs, *ToDBTLD(tld))
	}

	return rar
}

func FromDBRegistrar(dbr *Registrar) *entities.Registrar {
	registrar := &entities.Registrar{
		ClID:       entities.ClIDType(dbr.ClID),
		Name:       dbr.Name,
		NickName:   dbr.NickName,
		GurID:      dbr.GurID,
		Status:     entities.RegistrarStatus(dbr.Status),
		Autorenew:  dbr.Autorenew,
		IANAStatus: entities.IANARegistrarStatus(dbr.IANAStatus),
		Voice:      entities.E164Type(dbr.Voice),
		Fax:        entities.E164Type(dbr.Fax),
		Email:      dbr.Email,
		WhoisInfo: entities.WhoisInfo{
			Name: entities.DomainName(dbr.Whois43),
			URL:  entities.URL(dbr.Whois80),
		},
		URL:         entities.URL(dbr.URL),
		RdapBaseURL: entities.URL(dbr.RdapBaseUrl),
		CreatedAt:   dbr.CreatedAt,
		UpdatedAt:   dbr.UpdatedAt,
	}

	a0 := &entities.Address{
		Street1:       entities.OptPostalLineType(dbr.Street1Int),
		Street2:       entities.OptPostalLineType(dbr.Street2Int),
		Street3:       entities.OptPostalLineType(dbr.Street3Int),
		City:          entities.PostalLineType(dbr.CityInt),
		StateProvince: entities.OptPostalLineType(dbr.SPInt),
		PostalCode:    entities.PCType(dbr.PCInt),
		CountryCode:   entities.CCType(dbr.CCInt),
	}

	pi0 := &entities.RegistrarPostalInfo{
		Type:    entities.PostalInfoEnumType("int"),
		Address: a0,
	}

	registrar.AddPostalInfo(pi0)

	a1 := &entities.Address{
		Street1:       entities.OptPostalLineType(dbr.Street1Loc),
		Street2:       entities.OptPostalLineType(dbr.Street2Loc),
		Street3:       entities.OptPostalLineType(dbr.Street3Loc),
		City:          entities.PostalLineType(dbr.CityLoc),
		StateProvince: entities.OptPostalLineType(dbr.SPLoc),
		PostalCode:    entities.PCType(dbr.PCLoc),
		CountryCode:   entities.CCType(dbr.CCLoc),
	}

	pi1 := &entities.RegistrarPostalInfo{
		Type:    entities.PostalInfoEnumType("loc"),
		Address: a1,
	}

	registrar.AddPostalInfo(pi1)

	for _, tld := range dbr.TLDs {
		registrar.TLDs = append(registrar.TLDs, FromDBTLD(&tld))
	}

	return registrar
}
