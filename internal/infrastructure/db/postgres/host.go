package postgres

import (
	"net/netip"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// Host is the GORM model for the host table
type Host struct {
	RoID                int64  `gorm:"primaryKey"`
	Name                string `gorm:"not null"`
	ClID                string `gorm:"not null"`
	CrRr                string
	UpRr                string
	InBailiwick         bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
	entities.HostStatus `gorm:"embedded"`
	Addresses           []HostAddress `gorm:"foreignKey:HostRoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // When the parent host is deleted, delete the addresses
}

// TableName returns the table name for the Host model
func (Host) TableName() string {
	return "hosts"
}

// ToHost converts a postgres.Host to an entities.Host
func ToHost(dbHost *Host) *entities.Host {
	roid, _ := entities.NewRoidType(dbHost.RoID, entities.RoidTypeHost)

	// When retrieving and preloading the addresses we want to convert them to our entity
	addresses := make([]netip.Addr, len(dbHost.Addresses))
	for i, addr := range dbHost.Addresses {
		a, _ := netip.ParseAddr(addr.Address)
		addresses[i] = a
	}

	return &entities.Host{
		RoID:        roid,
		Name:        entities.DomainName(dbHost.Name),
		ClID:        entities.ClIDType(dbHost.ClID),
		CrRr:        entities.ClIDType(dbHost.CrRr),
		UpRr:        entities.ClIDType(dbHost.UpRr),
		InBailiwick: dbHost.InBailiwick,
		CreatedAt:   dbHost.CreatedAt,
		UpdatedAt:   dbHost.UpdatedAt,
		HostStatus:  dbHost.HostStatus,
		Addresses:   addresses,
	}
}

// ToDBHost converts an entities.Host to a postgres.Host
func ToDBHost(host entities.Host) *Host {
	roid, _ := host.RoID.Int64()

	return &Host{
		RoID:        roid,
		Name:        string(host.Name),
		ClID:        string(host.ClID),
		CrRr:        string(host.CrRr),
		UpRr:        string(host.UpRr),
		InBailiwick: host.InBailiwick,
		CreatedAt:   host.CreatedAt,
		UpdatedAt:   host.UpdatedAt,
		HostStatus:  host.HostStatus,
		// We don't need to convert the addresses since these are managed in the database independently
	}
}
