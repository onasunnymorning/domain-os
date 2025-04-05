package postgres

import (
	"context"
	"errors"
	"strconv"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// PremiumListRepository implements the PremiumListRepository interface
type PremiumListRepository struct {
	db *gorm.DB
}

// NewPremiumListRepository creates a new PremiumListRepository instance
func NewGORMPremiumListRepository(db *gorm.DB) *PremiumListRepository {
	return &PremiumListRepository{
		db: db,
	}
}

// Create creates a new premium list in the database
func (plr *PremiumListRepository) Create(ctx context.Context, premiumList *entities.PremiumList) (*entities.PremiumList, error) {
	pl := &PremiumList{}
	pl.FromEntity(premiumList)
	err := plr.db.WithContext(ctx).Create(pl).Error
	if err != nil {
		return nil, err
	}
	return pl.ToEntity(), nil
}

// GetByName retrieves a premium list by name
func (plr *PremiumListRepository) GetByName(ctx context.Context, name string) (*entities.PremiumList, error) {
	pl := &PremiumList{}
	if err := plr.db.WithContext(ctx).Where("name = ?", name).First(pl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrPremiumListNotFound
		}
		return nil, err
	}
	return pl.ToEntity(), nil
}

// DeleteByName deletes a premium list by name
func (plr *PremiumListRepository) DeleteByName(ctx context.Context, name string) error {
	return plr.db.WithContext(ctx).Where("name = ?", name).Delete(&PremiumList{}).Error
}

// List retrieves premium lists
func (plr *PremiumListRepository) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.PremiumList, string, error) {
	// Create a query object
	dbQuery := plr.db.WithContext(ctx).Order("name ASC")

	// Add cursor pagination if a cursor is provided
	if params.PageCursor != "" {
		cursorInt64, err := strconv.ParseInt(params.PageCursor, 10, 64)
		if err != nil {
			return nil, "", err
		}
		dbQuery = dbQuery.Where("id > ?", cursorInt64)
	}

	// Apply filter
	if params.Filter != nil {
		var err error
		if filter, ok := params.Filter.(queries.ListPremiumListsFilter); !ok {
			return nil, "", ErrInvalidFilterType
		} else {
			dbQuery, err = setPremiumListFilters(dbQuery, filter)
			if err != nil {
				return nil, "", err
			}
		}
	}

	// Limit results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // Fetch one more than the limit to determine if there are more results

	// Do the query
	dbpls := []*PremiumList{}
	err := dbQuery.Find(&dbpls).Error
	if err != nil {
		return nil, "", err
	}

	// Check result size
	hasMore := len(dbpls) == params.PageSize+1
	if hasMore {
		// Return up to the pagesize
		dbpls = dbpls[:params.PageSize]
	}

	// Convert to entities
	pls := make([]*entities.PremiumList, len(dbpls))
	for i, dbpl := range dbpls {
		pls[i] = dbpl.ToEntity()
	}

	// Set the cursor to the last label if there are more results
	var cursor string
	if hasMore {
		cursor = dbpls[len(dbpls)-1].Name
	}

	return pls, cursor, nil
}

func setPremiumListFilters(dbQuery *gorm.DB, filter queries.ListPremiumListsFilter) (*gorm.DB, error) {
	if filter.NameLike != "" {
		dbQuery = dbQuery.Where("name ILIKE ?", "%"+filter.NameLike+"%")
	}
	if filter.RyIDEquals != "" {
		dbQuery = dbQuery.Where("ry_id = ?", filter.RyIDEquals)
	}
	if filter.CreatedAfter != "" {
		dbQuery = dbQuery.Where("created_at > ?", filter.CreatedAfter)
	}
	if filter.CreatedBefore != "" {
		dbQuery = dbQuery.Where("created_at < ?", filter.CreatedBefore)
	}

	return dbQuery, nil
}
