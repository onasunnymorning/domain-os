package postgres

import (
	"context"
	"net/netip"

	"gorm.io/gorm"
)

// HostAddressRepository is the postgres implementation of the HostAddressRepository
type HostAddressRepository struct {
	db *gorm.DB
}

// NewGormHostAddressRepository creates a new instance of HostAddressRepository
func NewGormHostAddressRepository(db *gorm.DB) *HostAddressRepository {
	return &HostAddressRepository{db: db}
}

// CreateHostAddress creates a new host address
func (r *HostAddressRepository) CreateHostAddress(ctx context.Context, hostRoid int64, addr *netip.Addr) (*netip.Addr, error) {
	gormHostAddress := ToDBHostAddress(*addr, hostRoid)
	err := r.db.WithContext(ctx).Create(gormHostAddress).Error
	if err != nil {
		return nil, err
	}
	return addr, nil
}

// GetHostAddressesByHostRoid gets all host addresses by the host roid
func (r *HostAddressRepository) GetHostAddressesByHostRoid(ctx context.Context, hostRoid int64) ([]netip.Addr, error) {
	var gormHostAddresses []HostAddress
	err := r.db.WithContext(ctx).Where("host_ro_id = ?", hostRoid).Find(&gormHostAddresses).Error
	if err != nil {
		return nil, err
	}
	addresses := make([]netip.Addr, len(gormHostAddresses))
	for i, addr := range gormHostAddresses {
		addresses[i] = ToHostAddress(&addr)
	}
	return addresses, nil
}

// DeleteHostAddressByHostRoidAndAddress deletes a host address by the host roid and address
func (r *HostAddressRepository) DeleteHostAddressByHostRoidAndAddress(ctx context.Context, hostRoid int64, addr *netip.Addr) error {
	return r.db.WithContext(ctx).Where("host_ro_id = ?", hostRoid).Where("address = ?", addr.String()).Delete(&HostAddress{}).Error
}
