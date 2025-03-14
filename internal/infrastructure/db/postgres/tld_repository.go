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
func (repo *GormTLDRepository) List(ctx context.Context, params queries.ListTldsQuery) ([]*entities.TLD, error) {
	// Get a query object ordering by name (PK used for cursor pagination)
	dbQuery := repo.db.WithContext(ctx).Order("name ASC")

	// Add cursor pagination if a cursor is provided
	if params.PageCursor != "" {
		dbQuery = dbQuery.Where("name > ?", params.PageCursor)
	}

	// Add filters if provided
	if params.Filter.NameLike != "" {
		dbQuery = dbQuery.Where("name LIKE ?", "%"+params.Filter.NameLike+"%")
	}
	if params.Filter.TypeEquals != "" {
		dbQuery = dbQuery.Where("type = ?", params.Filter.TypeEquals)
	}
	if params.Filter.RyIDEquals != "" {
		dbQuery = dbQuery.Where("ry_id = ?", params.Filter.RyIDEquals)
	}

	// Limit the number of results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // Fetch one more than the page size to determine if there is a next page

	// Execute the query
	dbtlds := []*TLD{}
	err := dbQuery.Find(&dbtlds).Error
	if err != nil {
		return nil, err
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
	if hasMore {
		params.PageCursor = tlds[len(tlds)].Name.String()
	}

	return tlds, nil
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
func (repo *GormTLDRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := repo.db.WithContext(ctx).Model(&TLD{}).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}
