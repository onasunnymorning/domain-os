package postgres

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// TLD is a GORM struct representing a TLD in the database
type TLD struct {
	Name              string `gorm:"primary_key"`
	Type              string `gorm:"index"`
	UName             string
	AllowEscrowImport bool
	EnableDNS         bool
	// One to Many relationship with Phases
	Phases []Phase `gorm:"foreignKey:TLDName;references:Name;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	// FK relationship with RegistryOperator
	RyID      string
	CreatedAt time.Time
	UpdatedAt time.Time
	// Many to Many relationship with Registrars (AKA accreditations)
	Registrars []Registrar `gorm:"many2many:accreditations;"`
	// FK relationships with domains
	DNSRecord []*TLDDNSRecord `gorm:"foreignKey:Zone"`
}

// ToDBTLD converts a TLD struct to a DBTLD struct
func ToDBTLD(tld *entities.TLD) *TLD {
	dbTLD := &TLD{
		Name:              tld.Name.String(),
		Type:              tld.Type.String(),
		UName:             tld.UName.String(),
		RyID:              tld.RyID.String(),
		AllowEscrowImport: tld.AllowEscrowImport,
		EnableDNS:         tld.EnableDNS,
		CreatedAt:         tld.CreatedAt,
		UpdatedAt:         tld.UpdatedAt,
	}

	for _, phase := range tld.Phases {
		dbPhase := &Phase{}
		dbPhase.FromEntity(&phase)
		dbTLD.Phases = append(dbTLD.Phases, *dbPhase)
	}
	return dbTLD
}

// FromDBTLD converts a DBTLD struct to a TLD struct
func FromDBTLD(dbtld *TLD) *entities.TLD {
	tld := &entities.TLD{
		Name:              entities.DomainName(dbtld.Name),
		Type:              entities.TLDType(dbtld.Type),
		UName:             entities.DomainName(dbtld.UName),
		RyID:              entities.ClIDType(dbtld.RyID),
		AllowEscrowImport: dbtld.AllowEscrowImport,
		EnableDNS:         dbtld.EnableDNS,
		CreatedAt:         dbtld.CreatedAt.UTC(),
		UpdatedAt:         dbtld.UpdatedAt.UTC(),
	}
	for _, dbphase := range dbtld.Phases {
		phase := dbphase.ToEntity()
		tld.Phases = append(tld.Phases, *phase)

	}
	return tld
}
