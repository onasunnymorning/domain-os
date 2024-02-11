package repositories

import (
	"context"

	"net/netip"
)

// HostAddressRepository is the interface for the HostAddressRepository
type HostAddressRepository interface {
	CreateHostAddress(ctx context.Context, hostRoid int64, addr *netip.Addr) (*netip.Addr, error)
	GetHostAddressesByHostRoid(ctx context.Context, hostRoid int64) ([]netip.Addr, error)
	DeleteHostAddressByHostRoidAndAddress(ctx context.Context, hostRoid int64, addr *netip.Addr) error
}
