package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
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

// CreateHost creates a new host and does NOT create the addresses
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

// BulkCreate creates multiple hosts in a single transaction. If addresses are provided, they will be created as well
// Should one of the hosts fail to be created, the operation fails and no hosts are created, the error will be returned
func (r *HostRepository) BulkCreate(ctx context.Context, hosts []*entities.Host) error {
	// Convert entities to db entities
	dbHosts := make([]*Host, len(hosts))
	for i, h := range hosts {
		dbHosts[i] = ToDBHost(h)
	}

	return r.db.WithContext(ctx).Create(dbHosts).Error
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

// GetHostByNameAndClID gets a host by its name and clid
func (r *HostRepository) GetHostByNameAndClID(ctx context.Context, name string, clid string) (*entities.Host, error) {
	var gormHost Host
	err := r.db.WithContext(ctx).Preload("Addresses").Where("name = ? AND cl_id = ?", name, clid).First(&gormHost).Error
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
func (r *HostRepository) ListHosts(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Host, string, error) {
	// Get a query object ordering by ro_id (PK used for cursor pagination)
	dbQuery := r.db.WithContext(ctx).Order("ro_id ASC")

	// Add cursor pagination if a cursor is provided
	if params.PageCursor != "" {
		roidInt, err := getInt64RoidFromHostRoidString(params.PageCursor)
		if err != nil {
			return nil, "", err
		}
		dbQuery = dbQuery.Where("ro_id > ?", roidInt)
	}

	// Add filters
	if params.Filter != nil {
		// cast interface to ListDomainsQueryFilter
		if f, ok := params.Filter.(queries.ListHostsFilter); !ok {
			return nil, "", ErrInvalidFilterType
		} else {
			if f.RoidGreaterThan != "" {
				roidInt, err := getInt64RoidFromHostRoidString(f.RoidGreaterThan)
				if err != nil {
					return nil, "", err
				}
				dbQuery = dbQuery.Where("ro_id > ?", roidInt)
			}
			if f.RoidLessThan != "" {
				roidInt, err := getInt64RoidFromHostRoidString(f.RoidLessThan)
				if err != nil {
					return nil, "", err
				}
				dbQuery = dbQuery.Where("ro_id < ?", roidInt)
			}
			if f.ClidEquals != "" {
				dbQuery = dbQuery.Where("cl_id = ?", f.ClidEquals)
			}
			if f.NameLike != "" {
				dbQuery = dbQuery.Where("name ILIKE ?", "%"+f.NameLike+"%")
			}
		}
	}

	// Limit the number of results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // We fetch one more than the page size to determine if there are more results

	// Execute the query
	dbHosts := []*Host{}
	err := dbQuery.Find(&dbHosts).Error
	if err != nil {
		return nil, "", err
	}

	// Check if there is a next page
	hasMore := len(dbHosts) == params.PageSize+1
	if hasMore {
		// Return only up to PageSize
		dbHosts = dbHosts[:params.PageSize]
	}

	// Map the DBHosts to Hosts
	hosts := make([]*entities.Host, len(dbHosts))
	for i, h := range dbHosts {
		hosts[i] = ToHost(h)
	}

	// Set the cursor to the last roid in the list
	var newCursor string
	if hasMore {
		newCursor = hosts[len(hosts)-1].RoID.String()
	}

	return hosts, newCursor, nil
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

func getInt64RoidFromHostRoidString(roidString string) (int64, error) {
	// If the cursor is empty, we don't need to paginate, this is not an error
	if roidString == "" {
		return 0, nil
	}
	roid := entities.RoidType(roidString)
	if validationErr := roid.Validate(); validationErr != nil {
		return 0, validationErr
	}
	if roid.ObjectIdentifier() != entities.HOST_ROID_ID {
		return 0, entities.ErrInvalidRoid
	}
	return roid.Int64()
}
