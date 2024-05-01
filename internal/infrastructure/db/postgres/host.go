package postgres

import (
	"net/netip"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// Host is the GORM model for the host table
type Host struct {
	RoID                int64  `gorm:"primaryKey"`
	Name                string `gorm:"uniqueIndex:idx_uniq_name_clid;not null"`
	ClID                string `gorm:"uniqueIndex:idx_uniq_name_clid;not null"`
	CrRr                *string
	UpRr                *string
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
		addresses[i] = ToHostAddress(&addr)
	}

	h := &entities.Host{
		RoID:        roid,
		Name:        entities.DomainName(dbHost.Name),
		ClID:        entities.ClIDType(dbHost.ClID),
		InBailiwick: dbHost.InBailiwick,
		CreatedAt:   dbHost.CreatedAt,
		UpdatedAt:   dbHost.UpdatedAt,
		Status:      dbHost.HostStatus,
		Addresses:   addresses,
	}

	if dbHost.CrRr != nil {
		h.CrRr = entities.ClIDType(*dbHost.CrRr)
	}

	if dbHost.UpRr != nil {
		h.UpRr = entities.ClIDType(*dbHost.UpRr)
	}

	return h
}

// ToDBHost converts an entities.Host to a postgres.Host
func ToDBHost(host *entities.Host) *Host {
	roid, _ := host.RoID.Int64()

	addr := make([]HostAddress, len(host.Addresses))
	for i, a := range host.Addresses {
		addr[i] = *ToDBHostAddress(a, roid)
	}

	h := &Host{
		RoID:        roid,
		Name:        string(host.Name),
		ClID:        string(host.ClID),
		InBailiwick: host.InBailiwick,
		CreatedAt:   host.CreatedAt,
		UpdatedAt:   host.UpdatedAt,
		HostStatus:  host.Status,
		Addresses:   addr,
	}

	if host.CrRr != entities.ClIDType("") {
		cr := host.CrRr.String()
		h.CrRr = &cr
	}

	if host.UpRr != entities.ClIDType("") {
		ur := host.UpRr.String()
		h.UpRr = &ur
	}

	return h
}
