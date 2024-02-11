package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// HostRepository is the postgres implementation of the HostRepository
type HostRepository struct {
	db *gorm.DB
}

// NewGormHostRepository creates a new instance of HostRepository
func NewGormHostRepository(db *gorm.DB) *HostRepository {
	return &HostRepository{db: db}
}

// CreateHost creates a new host
func (r *HostRepository) CreateHost(ctx context.Context, host *entities.Host) (*entities.Host, error) {
	// If we don't remove the addresses here, they will be present in the response, which could lead the user to believe they were created, while in face we Omit them
	host.Addresses = nil
	gormHost := ToDBHost(host)
	err := r.db.WithContext(ctx).Omit("Addresses").Create(gormHost).Error // We Omit addresses we don't want to manage these through this endpoint
	if err != nil {
		return nil, err
	}
	return ToHost(gormHost), nil
}

// GetHostByRoid gets a host by its roid
func (r *HostRepository) GetHostByRoid(ctx context.Context, roid int64) (*entities.Host, error) {
	var gormHost Host
	err := r.db.WithContext(ctx).First(&gormHost, roid).Error
	if err != nil {
		return nil, err
	}
	return ToHost(&gormHost), nil
}

// UpdateHost updates a host
func (r *HostRepository) UpdateHost(ctx context.Context, host *entities.Host) (*entities.Host, error) {
	// If we don't remove the addresses here, they will be present in the response, which could lead the user to believe they were updated, while in face we Omit them
	host.Addresses = nil
	gormHost := ToDBHost(host)
	err := r.db.WithContext(ctx).Omit("Addresses").Save(gormHost).Error // We Omit addresses we don't want to manage these through this endpoint
	if err != nil {
		return nil, err
	}
	return ToHost(gormHost), nil
}

// DeleteHostByRoid deletes a host by its roid
func (r *HostRepository) DeleteHostByRoid(ctx context.Context, roid int64) error {
	return r.db.WithContext(ctx).Delete(&Host{}, roid).Error
}
