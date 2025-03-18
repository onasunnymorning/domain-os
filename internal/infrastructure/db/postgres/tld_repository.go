package postgres

import (
	"context"
	"errors"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// GormTLDRepository implements the TLDRepo interface
type GormTLDRepository struct {
	db *gorm.DB
}

// NewGormTLDRepo returns a new GormTLDRepo
func NewGormTLDRepo(db *gorm.DB) *GormTLDRepository {
	return &GormTLDRepository{
		db: db,
	}
}

// GetByName retrieves a TLD by the specified name from the repository. If preloadAll
// is true, it preloads additional associated phase and pricing and fee details. If no record
// is found, it returns entities.ErrTLDNotFound; otherwise, it returns any encountered
// error.
func (repo *GormTLDRepository) GetByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
	dbtld := &TLD{}
	var err error

	if preloadAll {
		err = repo.db.WithContext(ctx).Preload("Phases.Prices").Preload("Phases.Fees").Where("name = ?", name).First(dbtld).Error
	} else {
		err = repo.db.WithContext(ctx).Preload("Phases").Where("name = ?", name).First(dbtld).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrTLDNotFound
		}
		return nil, err
	}

	tld := FromDBTLD(dbtld)

	return tld, nil
}

// Create creates a new TLD in the database
func (repo *GormTLDRepository) Create(ctx context.Context, tld *entities.TLD) error {
	// Map the TLD to a DBTLD
	dbtld := ToDBTLD(tld)

	err := repo.db.WithContext(ctx).Create(dbtld).Error
	if err != nil {
		return err
	}

	// Read the data from the repo to ensure we return the same data that was written
	storedDBTLD, err := repo.GetByName(ctx, tld.Name.String(), false)
	if err != nil {
		return err
	}

	// Map the DBTLD back to a TLD
	*tld = *storedDBTLD

	return nil
}

// List returns a list of all TLDs. TLDs are ordered alphabetically by name and user pagination is supported by pagesize and cursor(name)
func (repo *GormTLDRepository) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.TLD, string, error) {
	// Get a query object ordering by name (PK used for cursor pagination)
	dbQuery := repo.db.WithContext(ctx).Order("name ASC")

	// Add cursor pagination if a cursor is provided
	if params.PageCursor != "" {
		dbQuery = dbQuery.Where("name > ?", params.PageCursor)
	}
	if params.Filter != nil {
		// cast interface to ListTldsFilter
		if filter, ok := params.Filter.(queries.ListTldsFilter); !ok {
			return nil, "", ErrInvalidFilterType
		} else {
			// Add filters if provided
			err := setTldFilters(dbQuery, filter)
			if err != nil {
				return nil, "", err
			}
		}

	}

	// Limit the number of results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // Fetch one more than the page size to determine if there is a next page

	// Execute the query
	dbtlds := []*TLD{}
	err := dbQuery.Find(&dbtlds).Error
	if err != nil {
		return nil, "", err
	}

	// Check if there is a next page
	hasMore := len(dbtlds) == params.PageSize+1
	if hasMore {
		// Return only up to Pagesize
		dbtlds = dbtlds[:params.PageSize]
	}

	// Map the DBTLDs to TLDs
	tlds := make([]*entities.TLD, len(dbtlds))
	for i, dbtld := range dbtlds {
		tlds[i] = FromDBTLD(dbtld)
	}

	// Set the cursor to the last name in the list
	var newCursor string
	if hasMore {
		newCursor = tlds[len(tlds)-1].Name.String()
	}

	return tlds, newCursor, nil
}

// Delete deletes a TLD from the database
func (repo *GormTLDRepository) DeleteByName(ctx context.Context, name string) error {
	return repo.db.WithContext(ctx).Where("name = ?", name).Delete(&TLD{}).Error
}

// Update updates a TLD in the database
func (repo *GormTLDRepository) Update(ctx context.Context, tld *entities.TLD) error {
	// Map the TLD to a DBTLD
	dbtld := ToDBTLD(tld)

	err := repo.db.WithContext(ctx).Save(dbtld).Error
	if err != nil {
		return err
	}

	// Read the data from the repo to ensure we return the same data that was written
	storedDBTLD, err := repo.GetByName(ctx, tld.Name.String(), false)
	if err != nil {
		return err
	}

	// Map the DBTLD back to a TLD
	*tld = *storedDBTLD

	return nil
}

// Count returns the total number of TLDs in the database
// TODO: add a filter to count only TLDs that match a certain criteria
func (repo *GormTLDRepository) Count(ctx context.Context, filter queries.ListTldsFilter) (int64, error) {
	var count int64
	// create query object
	dbQuery := repo.db.WithContext(ctx).Model(&TLD{})
	// add filters if provided
	err := setTldFilters(dbQuery, filter)
	if err != nil {
		return 0, err
	}

	err = dbQuery.Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func setTldFilters(dbQuery *gorm.DB, filter queries.ListTldsFilter) error {

	if filter.NameLike != "" {
		dbQuery = dbQuery.Where("name LIKE ?", "%"+filter.NameLike+"%")
	}
	if filter.TypeEquals != "" {
		dbQuery = dbQuery.Where("type = ?", filter.TypeEquals)
	}
	if filter.RyIDEquals != "" {
		dbQuery = dbQuery.Where("ry_id = ?", filter.RyIDEquals)
	}

	return nil

}
