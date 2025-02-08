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

// MockHostAddressRepository is the mock implementation of the HostAddressRepository
type MockHostAddressRepository struct {
	CreateHostAddressFunc                     func(ctx context.Context, hostRoid int64, addr *netip.Addr) (*netip.Addr, error)
	GetHostAddressesByHostRoidFunc            func(ctx context.Context, hostRoid int64) ([]netip.Addr, error)
	DeleteHostAddressByHostRoidAndAddressFunc func(ctx context.Context, hostRoid int64, addr *netip.Addr) error
}

// CreateHostAddress creates a host address
func (m *MockHostAddressRepository) CreateHostAddress(ctx context.Context, hostRoid int64, addr *netip.Addr) (*netip.Addr, error) {
	return m.CreateHostAddressFunc(ctx, hostRoid, addr)
}

// GetHostAddressesByHostRoid gets a host's addresses by its roid
func (m *MockHostAddressRepository) GetHostAddressesByHostRoid(ctx context.Context, hostRoid int64) ([]netip.Addr, error) {
	return m.GetHostAddressesByHostRoidFunc(ctx, hostRoid)
}

// DeleteHostAddressByHostRoidAndAddress deletes a host address by its host roid and address
func (m *MockHostAddressRepository) DeleteHostAddressByHostRoidAndAddress(ctx context.Context, hostRoid int64, addr *netip.Addr) error {
	return m.DeleteHostAddressByHostRoidAndAddressFunc(ctx, hostRoid, addr)
}

// NewMockHostAddressRepository creates a new instance of MockHostAddressRepository
func NewMockHostAddressRepository() *MockHostAddressRepository {
	return &MockHostAddressRepository{}
}
