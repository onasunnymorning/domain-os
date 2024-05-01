package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
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
	// If we don't remove the addresses here, they will be present in the response, which could lead the user to believe they were created, while in fact we Omit them
	host.Addresses = nil
	gormHost := ToDBHost(host)
	err := r.db.WithContext(ctx).Omit("Addresses").Create(gormHost).Error // We Omit addresses we want to manage these through the address repository
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, entities.ErrHostAlreadyExists
		}
		return nil, err
	}
	return ToHost(gormHost), nil
}

// GetHostByRoid gets a host by its roid
func (r *HostRepository) GetHostByRoid(ctx context.Context, roid int64) (*entities.Host, error) {
	var gormHost Host
	err := r.db.WithContext(ctx).Preload("Addresses").First(&gormHost, roid).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrHostNotFound
		}
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

// ListHosts lists hosts
func (r *HostRepository) ListHosts(ctx context.Context, pageSize int, cursor string) ([]*entities.Host, error) {
	var roidInt int64
	var err error
	if cursor != "" {
		roid := entities.RoidType(cursor)
		if roid.ObjectIdentifier() != entities.HOST_ROID_ID {
			return nil, entities.ErrInvalidRoid
		}
		roidInt, err = roid.Int64()
		if err != nil {
			return nil, err
		}
	}
	dbHosts := []*Host{}
	err = r.db.WithContext(ctx).Order("ro_id ASC").Limit(pageSize).Find(&dbHosts, "ro_id > ?", roidInt).Error
	if err != nil {
		return nil, err
	}

	hosts := make([]*entities.Host, len(dbHosts))
	for i, h := range dbHosts {
		hosts[i] = ToHost(h)
	}

	return hosts, nil
}

// GetHostAssociationCount returns the number of domains a host is associated with. This can be used to determine if a host needs the linked flag to be unset
func (r *HostRepository) GetHostAssociationCount(ctx context.Context, roid int64) (int64, error) {
	var count int64
	err := r.db.Raw("SELECT COUNT(*) FROM domain_hosts WHERE host_ro_id = ?", roid).Scan(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}
