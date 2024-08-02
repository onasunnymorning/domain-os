package postgres

import "net/netip"

// HostAddress is the GORM model for the host_address table
type HostAddress struct {
	HostRoID int64  `gorm:"primaryKey"`
	Address  string `gorm:"primaryKey"`
	Version  int
}

// TableName returns the table name for the HostAddress model
func (HostAddress) TableName() string {
	return "host_addresses"
}

// ToHostAddress converts a postgres.HostAddress to an netip.Addr
func ToHostAddress(dbHostAddress *HostAddress) netip.Addr {
	// Convert the IP to a netip.Addr
	addr, _ := netip.ParseAddr(dbHostAddress.Address)

	return addr
}

// ToDBHostAddress converts a netip.Addr to a postgres.HostAddress
func ToDBHostAddress(addr netip.Addr, hostRoID int64) *HostAddress {
	ha := &HostAddress{
		Address:  addr.String(),
		HostRoID: hostRoID,
	}
	if addr.Is4() {
		ha.Version = 4
	} else {
		ha.Version = 6
	}
	return ha
}
